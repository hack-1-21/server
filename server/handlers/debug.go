package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
	"os"
)

// ResetDatabase DELETE /debug/reset
// ハッカソンや開発中にデータベースの全データを削除してリセットするためのエンドポイント。
// users テーブルと measurements テーブルを空にします。
func ResetDatabase(w http.ResponseWriter, r *http.Request) {
	// 全テーブルのデータを削除（IDなどはリセットされる）
	_, err := db.DB.Exec(`TRUNCATE TABLE measurements, users RESTART IDENTITY CASCADE`)
	if err != nil {
		log.Printf("データベースリセット失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データベースのリセットに失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "すべてのデータを正常にリセットしました",
	})
}

type DebugAddGardenPointsRequest struct {
	UserID string `json:"user_id"`
	Points int    `json:"points"`
}

type DebugGenerateInitialGardenRequest struct {
	UserID string `json:"user_id"`
}

type DebugBulkMeasurementsRequest struct {
	UserID    string  `json:"user_id"`
	Count     int     `json:"count"`
	CenterLat float64 `json:"center_lat"`
	CenterLng float64 `json:"center_lng"`
	RadiusLat float64 `json:"radius_lat"`
	RadiusLng float64 `json:"radius_lng"`
	MinDB     float64 `json:"min_db"`
	MaxDB     float64 `json:"max_db"`
}

// DebugBulkMeasurements POST /debug/measurements/bulk
// マップ表示確認用に、指定ユーザーの測定データを中心座標周辺へ一括投入する
func DebugBulkMeasurements(w http.ResponseWriter, r *http.Request) {
	var req DebugBulkMeasurementsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}
	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id は必須です")
		return
	}
	if req.Count <= 0 {
		req.Count = 10000
	}
	if req.Count > 50000 {
		respondError(w, http.StatusBadRequest, "count は 50000 以下で指定してください")
		return
	}
	if req.CenterLat == 0 {
		req.CenterLat = 35.6896
	}
	if req.CenterLng == 0 {
		req.CenterLng = 139.7006
	}
	if req.RadiusLat == 0 {
		req.RadiusLat = 0.015
	}
	if req.RadiusLng == 0 {
		req.RadiusLng = 0.015
	}
	if req.MinDB == 0 {
		req.MinDB = 45
	}
	if req.MaxDB == 0 {
		req.MaxDB = 90
	}
	if req.CenterLat < -90 || req.CenterLat > 90 || req.CenterLng < -180 || req.CenterLng > 180 {
		respondError(w, http.StatusBadRequest, "center_lat または center_lng の範囲が不正です")
		return
	}
	if req.MinDB < 0 || req.MaxDB > 140 || req.MinDB > req.MaxDB {
		respondError(w, http.StatusBadRequest, "min_db / max_db の範囲が不正です")
		return
	}

	var exists bool
	if err := db.DB.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users WHERE user_id = $1)`,
		req.UserID,
	).Scan(&exists); err != nil {
		log.Printf("users EXISTS確認失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー確認に失敗しました")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "指定されたユーザーが存在しません")
		return
	}

	var inserted int
	err := db.DB.QueryRow(
		`WITH inserted AS (
			INSERT INTO measurements (user_id, db, hz, latitude, longitude, created_at)
			SELECT
				$1,
				$7::double precision + random() * ($8::double precision - $7::double precision),
				100 + random() * 4900,
				$3::double precision + (random() - 0.5) * 2 * $5::double precision,
				$4::double precision + (random() - 0.5) * 2 * $6::double precision,
				NOW() - (random() * interval '7 days')
			FROM generate_series(1, $2::integer)
			RETURNING id
		)
		SELECT COUNT(*) FROM inserted`,
		req.UserID,
		req.Count,
		req.CenterLat,
		req.CenterLng,
		req.RadiusLat,
		req.RadiusLng,
		req.MinDB,
		req.MaxDB,
	).Scan(&inserted)
	if err != nil {
		log.Printf("measurements bulk INSERT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "測定データの一括投入に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "測定データを一括投入しました",
		"user_id":    req.UserID,
		"inserted":   inserted,
		"center_lat": req.CenterLat,
		"center_lng": req.CenterLng,
		"radius_lat": req.RadiusLat,
		"radius_lng": req.RadiusLng,
		"min_db":     req.MinDB,
		"max_db":     req.MaxDB,
	})
}

// DebugGenerateInitialGarden POST /debug/garden/generate-initial
// 既存ユーザーに初期箱庭がない場合は作成し、Stage 1 / Generation 1 の画像生成を明示的に実行する
func DebugGenerateInitialGarden(w http.ResponseWriter, r *http.Request) {
	var req DebugGenerateInitialGardenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}
	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id は必須です")
		return
	}

	var exists bool
	if err := db.DB.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users WHERE user_id = $1)`,
		req.UserID,
	).Scan(&exists); err != nil {
		log.Printf("users EXISTS確認失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー確認に失敗しました")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "指定されたユーザーが存在しません")
		return
	}

	var gardenID int
	err := db.DB.QueryRow(
		`INSERT INTO gardens (user_id, generation, points, stage, is_active)
		 SELECT $1, 1, 0, 1, TRUE
		 WHERE NOT EXISTS (
		   SELECT 1 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 )
		 RETURNING id`,
		req.UserID,
	).Scan(&gardenID)
	if err == sql.ErrNoRows {
		err = db.DB.QueryRow(
			`SELECT id
			 FROM gardens
			 WHERE user_id = $1 AND is_active = TRUE
			 ORDER BY generation DESC
			 LIMIT 1`,
			req.UserID,
		).Scan(&gardenID)
	}
	if err != nil {
		log.Printf("garden 取得/作成失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "箱庭の取得または作成に失敗しました")
		return
	}

	if !isImageGenerationConfigured() {
		respondError(w, http.StatusInternalServerError, "CF_ACCOUNT_ID または CF_API_TOKEN が環境変数に設定されていません")
		return
	}

	GenerateAndSaveGardenImage(gardenID, 1, 1, req.UserID,
		func(gid int, url string) {
			if _, err := db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid); err != nil {
				log.Printf("gardens image_url UPDATE失敗: %v", err)
			}
		})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "初期箱庭画像生成を開始しました",
		"user_id":   req.UserID,
		"garden_id": gardenID,
	})
}

// DebugAddGardenPoints POST /debug/garden/add-points
// テスト用に箱庭のポイントを強制的に追加し、進化・画像生成を確認するためのエンドポイント
func DebugAddGardenPoints(w http.ResponseWriter, r *http.Request) {
	var req DebugAddGardenPointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DBトランザクション開始失敗")
		return
	}
	defer tx.Rollback()

	// ユーザー作成確認
	var u models.User
	err = tx.QueryRow(
		`SELECT level, exp, total_exp FROM users WHERE user_id = $1 FOR UPDATE`,
		req.UserID,
	).Scan(&u.Level, &u.Exp, &u.TotalExp)

	if err == sql.ErrNoRows {
		u.Level = 1
		u.Exp = 0
		u.TotalExp = 0
		_, err = tx.Exec(
			`INSERT INTO users (user_id, level, exp, total_exp) VALUES ($1, $2, $3, $4)`,
			req.UserID, u.Level, u.Exp, u.TotalExp,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "ユーザー作成失敗")
			return
		}
	}

	// 箱庭取得
	var garden models.Garden
	err = tx.QueryRow(
		`SELECT id, generation, points, stage, image_url
		 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY generation DESC LIMIT 1 FOR UPDATE`,
		req.UserID,
	).Scan(&garden.ID, &garden.Generation, &garden.Points, &garden.Stage, &garden.ImageURL)

	if err == sql.ErrNoRows {
		garden.UserID = req.UserID
		garden.Generation = 1
		garden.Points = 0
		garden.Stage = 1
		err = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, $3, $4, TRUE) RETURNING id`,
			req.UserID, garden.Generation, garden.Points, garden.Stage,
		).Scan(&garden.ID)
	}

	prevStage := garden.Stage
	garden.Points += req.Points
	garden.Stage = models.CalcStage(garden.Points)

	stageUp := garden.Stage > prevStage
	generationUp := garden.Points >= models.MaxPoints

	gainedExp := 0
	if stageUp {
		gainedExp = models.StageExpGain[garden.Stage]
	}
	if generationUp {
		gainedExp += models.WorldGenEXP
	}
	u.TotalExp += gainedExp
	u.Level = models.CalcUserLevel(u.TotalExp)

	// 更新
	_, _ = tx.Exec(
		`UPDATE users SET level = $1, total_exp = $2 WHERE user_id = $3`,
		u.Level, u.TotalExp, req.UserID,
	)

	if generationUp {
		historyImageURL := fmt.Sprintf("/images/gardens/%s/%s_gen%d.png", req.UserID, req.UserID, garden.Generation)
		_, _ = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2, is_active = FALSE, completed_at = NOW(), image_url = $3 WHERE id = $4`,
			garden.Points, garden.Stage, historyImageURL, garden.ID,
		)
		newGen := garden.Generation + 1
		var newID int
		_ = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, 0, 1, TRUE) RETURNING id`,
			req.UserID, newGen,
		).Scan(&newID)

		if isImageGenerationConfigured() {
			GenerateAndSaveGardenImage(newID, 1, newGen, req.UserID,
				func(gid int, url string) {
					db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid)
				})
		}

		garden.Generation = newGen
		garden.Points = 0
		garden.Stage = 1
		garden.ID = newID
	} else {
		_, _ = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2 WHERE id = $3`,
			garden.Points, garden.Stage, garden.ID,
		)
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "コミット失敗")
		return
	}

	if stageUp && isImageGenerationConfigured() {
		GenerateAndSaveGardenImage(garden.ID, garden.Stage, garden.Generation, req.UserID,
			func(gid int, url string) {
				db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid)
			})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "ポイントを追加しました",
		"added_points":  req.Points,
		"total_points":  garden.Points,
		"stage":         garden.Stage,
		"generation":    garden.Generation,
		"stage_up":      stageUp,
		"generation_up": generationUp,
	})
}

// GetAllUsersDebug GET /debug/users
// テスト用に全ユーザーの user_id の一覧を取得するエンドポイント
func GetAllUsersDebug(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(`SELECT user_id FROM users ORDER BY created_at DESC`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ユーザー一覧の取得に失敗しました")
		return
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err == nil {
			users = append(users, uid)
		}
	}
	respondJSON(w, http.StatusOK, users)
}

// TestCloudflareDebug GET /debug/test-cloudflare
// Cloudflare Workers AI を同期的に呼び出してエラー詳細をブラウザに返すテスト用API
func TestCloudflareDebug(w http.ResponseWriter, r *http.Request) {
	if !isImageGenerationConfigured() {
		respondError(w, http.StatusInternalServerError, "CF_ACCOUNT_ID または CF_API_TOKEN が環境変数に設定されていません")
		return
	}

	// 第2引数: userID (デバッグ用ダミー), 第3引数: stage=1 (i2iを使用せずtxt2imgでテスト)
	imgData, err := generateGardenImage("a tiny rabbit in a beautiful magical garden, photorealistic", "debug-test-user", 1)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Cloudflare API エラー: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Cloudflare Workers AI は正常に動作しています！",
		"size":    fmt.Sprintf("%d bytes", len(imgData)),
	})
}

// CheckCFConfigDebug GET /debug/check-cf-config
// CF_ACCOUNT_ID と CF_API_TOKEN が設定されているか確認する
func CheckCFConfigDebug(w http.ResponseWriter, r *http.Request) {
	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")

	result := map[string]bool{
		"CF_ACCOUNT_ID_set": accountID != "",
		"CF_API_TOKEN_set":  apiToken != "",
		"configured":        accountID != "" && apiToken != "",
	}
	respondJSON(w, http.StatusOK, result)
}
