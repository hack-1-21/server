package handlers

import (
	"database/sql"
	"encoding/json"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
)

// CreateMeasurement POST /measurements
// WearOS から dB・Hz・緯度・経度・user_id を受け取って保存する
func CreateMeasurement(w http.ResponseWriter, r *http.Request) {
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

	// トランザクションで測定データの保存とユーザーの経験値更新を行う
	tx, err := db.DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DBトランザクション開始失敗")
		return
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow(
		`INSERT INTO measurements (user_id, db, hz, latitude, longitude)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.UserID, req.DB, req.Hz, req.Latitude, req.Longitude,
	).Scan(&id)
	if err != nil {
		log.Printf("INSERT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	// ユーザーが存在するか確認、なければ作成
	var u models.User
	err = tx.QueryRow(
		`SELECT level, exp, points FROM users WHERE user_id = $1 FOR UPDATE`,
		req.UserID,
	).Scan(&u.Level, &u.Exp, &u.Points)

	if err == sql.ErrNoRows {
		// 新規作成
		u.Level = 1
		u.Exp = 0
		u.Points = 0
		_, err = tx.Exec(
			`INSERT INTO users (user_id, level, exp, points) VALUES ($1, $2, $3, $4)`,
			req.UserID, u.Level, u.Exp, u.Points,
		)
		if err != nil {
			log.Printf("ユーザー作成失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "ユーザー作成に失敗しました")
			return
		}
	} else if err != nil {
		log.Printf("ユーザー取得失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の取得に失敗しました")
		return
	}

	// 経験値付与 (例: 1回の測定で10EXP)
	gainedExp := 10
	u.Exp += gainedExp
	u.Points += gainedExp // 探索ポイントも付与

	// レベルアップ計算 (例: 次のレベル = 現在レベル * 100 EXP)
	levelUp := false
	for u.Exp >= u.Level*100 {
		u.Exp -= u.Level * 100
		u.Level++
		levelUp = true
	}

	// ユーザー情報更新
	_, err = tx.Exec(
		`UPDATE users SET level = $1, exp = $2, points = $3 WHERE user_id = $4`,
		u.Level, u.Exp, u.Points, req.UserID,
	)
	if err != nil {
		log.Printf("ユーザー更新失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の更新に失敗しました")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("コミット失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       id,
		"message":  "ok",
		"level_up": levelUp,
		"level":    u.Level,
		"exp":      u.Exp,
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
