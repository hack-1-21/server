package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
)

// CreateMeasurement POST /measurements
// WearOS から dB・Hz・緯度・経度を受け取り、device_token に紐づく user_id で保存する
func CreateMeasurement(w http.ResponseWriter, r *http.Request) {
	userID, _, err := userIDFromDeviceToken(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "device_token が無効です")
		return
	}

	var req models.CreateMeasurementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエストのJSON形式が不正です")
		return
	}

	if req.Latitude < -90 || req.Latitude > 90 {
		respondError(w, http.StatusBadRequest, "latitude は -90〜90 の範囲で指定してください")
		return
	}
	if req.Longitude < -180 || req.Longitude > 180 {
		respondError(w, http.StatusBadRequest, "longitude は -180〜180 の範囲で指定してください")
		return
	}

	// トランザクションで測定データ保存・ポイント加算・レベルアップを一括処理
	tx, err := db.DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DBトランザクション開始失敗")
		return
	}
	defer tx.Rollback()

	// 1. 音データ保存
	var measurementID int64
	err = tx.QueryRow(
		`INSERT INTO measurements (user_id, db, hz, latitude, longitude)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, req.DB, req.Hz, req.Latitude, req.Longitude,
	).Scan(&measurementID)
	if err != nil {
		log.Printf("measurements INSERT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	// 2. ユーザーが存在しなければ作成
	var u models.User
	err = tx.QueryRow(
		`SELECT user_id, level, exp, total_exp FROM users WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&u.UserID, &u.Level, &u.Exp, &u.TotalExp)

	if err == sql.ErrNoRows {
		u.UserID = userID
		u.Level = 1
		u.Exp = 0
		u.TotalExp = 0
		_, err = tx.Exec(
			`INSERT INTO users (user_id, level, exp, total_exp) VALUES ($1, $2, $3, $4)`,
			userID, u.Level, u.Exp, u.TotalExp,
		)
		if err != nil {
			log.Printf("users INSERT失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "ユーザー作成に失敗しました")
			return
		}
	} else if err != nil {
		log.Printf("users SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の取得に失敗しました")
		return
	}

	// 3. アクティブな箱庭を取得。なければ作成
	var garden models.Garden
	err = tx.QueryRow(
		`SELECT id, generation, points, stage, image_url
		 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY generation DESC LIMIT 1 FOR UPDATE`,
		userID,
	).Scan(&garden.ID, &garden.Generation, &garden.Points, &garden.Stage, &garden.ImageURL)

	if err == sql.ErrNoRows {
		// 初回：箱庭を作成
		garden.UserID = userID
		garden.Generation = 1
		garden.Points = 0
		garden.Stage = 1
		err = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, $3, $4, TRUE) RETURNING id`,
			userID, garden.Generation, garden.Points, garden.Stage,
		).Scan(&garden.ID)
		if err != nil {
			log.Printf("gardens INSERT失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "箱庭の作成に失敗しました")
			return
		}
	} else if err != nil {
		log.Printf("gardens SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "箱庭情報の取得に失敗しました")
		return
	}

	// 4. +1pt 加算
	prevStage := garden.Stage
	garden.Points++
	garden.Stage = models.CalcStage(garden.Points)

	stageUp := garden.Stage > prevStage
	generationUp := garden.Points >= models.MaxPoints

	// EXP加算（段階アップ時）
	gainedExp := 0
	if stageUp {
		gainedExp = models.StageExpGain[garden.Stage]
	}
	// 世代交代時はさらに +400 EXP
	if generationUp {
		gainedExp += models.WorldGenEXP
	}

	u.TotalExp += gainedExp
	u.Level = models.CalcUserLevel(u.TotalExp)

	// 5. ユーザー更新
	_, err = tx.Exec(
		`UPDATE users SET level = $1, total_exp = $2 WHERE user_id = $3`,
		u.Level, u.TotalExp, userID,
	)
	if err != nil {
		log.Printf("users UPDATE失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の更新に失敗しました")
		return
	}

	if generationUp {
		// 世代交代: 現世代を完了 → 新世代作成
		historyImageURL := fmt.Sprintf("/images/gardens/%s/%s_gen%d.png", userID, userID, garden.Generation)
		_, err = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2, is_active = FALSE, completed_at = NOW(), image_url = $3 WHERE id = $4`,
			garden.Points, garden.Stage, historyImageURL, garden.ID,
		)
		if err != nil {
			log.Printf("gardens UPDATE(完了)失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "箱庭の更新に失敗しました")
			return
		}
		// 新世代作成
		newGen := garden.Generation + 1
		var newGardenID int
		err = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, 0, 1, TRUE) RETURNING id`,
			userID, newGen,
		).Scan(&newGardenID)
		if err != nil {
			log.Printf("gardens INSERT(新世代)失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "新世代の箱庭作成に失敗しました")
			return
		}

		// 新世代（Stage 1）の初期画像も生成する
		if isImageGenerationConfigured() {
			GenerateAndSaveGardenImage(newGardenID, 1, newGen, userID,
				func(gid int, url string) {
					db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid)
				})
		}
		// レスポンス用に新世代情報を反映
		garden.Generation = newGen
		garden.Points = 0
		garden.Stage = 1
		garden.ImageURL = ""
		garden.ID = newGardenID
	} else {
		// 通常の更新
		_, err = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2 WHERE id = $3`,
			garden.Points, garden.Stage, garden.ID,
		)
		if err != nil {
			log.Printf("gardens UPDATE失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "箱庭の更新に失敗しました")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("コミット失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	// 6. 段階アップ時は非同期で画像生成
	if stageUp && isImageGenerationConfigured() {
		GenerateAndSaveGardenImage(garden.ID, garden.Stage, garden.Generation, userID,
			func(gid int, url string) {
				// 既に世代交代して非アクティブになっている場合はURLを上書きしない（履歴用の_genXURLを保つため）
				_, err := db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid)
				if err != nil {
					log.Printf("gardens image_url UPDATE失敗: %v", err)
				}
			},
		)
	}

	respondJSON(w, http.StatusCreated, models.MeasurementResponse{
		ID:           measurementID,
		Message:      "ok",
		Points:       garden.Points,
		Stage:        garden.Stage,
		ImageURL:     garden.ImageURL,
		Generation:   garden.Generation,
		StageUp:      stageUp,
		GenerationUp: generationUp,
		Level:        u.Level,
		TotalExp:     u.TotalExp,
	})
}

// GetMeasurements GET /measurements
// 測定データを全件または差分（after_id）で取得する
func GetMeasurements(w http.ResponseWriter, r *http.Request) {
	afterID := r.URL.Query().Get("after_id")

	query := `SELECT id, user_id, db, hz, latitude, longitude, created_at
		      FROM measurements`
	args := []interface{}{}

	if afterID != "" {
		query += ` WHERE id > $1`
		args = append(args, afterID)
	}

	// 全件取得を考慮し、昇順で返す（古いものから順に適用するため）
	query += ` ORDER BY id ASC`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		log.Printf("SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}
	defer rows.Close()

	measurements := []models.Measurement{}
	for rows.Next() {
		var m models.Measurement
		if err := rows.Scan(&m.ID, &m.UserID, &m.DB, &m.Hz, &m.Latitude, &m.Longitude, &m.CreatedAt); err != nil {
			log.Printf("Scan失敗: %v", err)
			continue
		}
		measurements = append(measurements, m)
	}

	respondJSON(w, http.StatusOK, measurements)
}

// HealthCheck GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
