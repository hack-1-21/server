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

	var id int64
	err := db.DB.QueryRow(
		`INSERT INTO measurements (user_id, db, hz, latitude, longitude)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.UserID, req.DB, req.Hz, req.Latitude, req.Longitude,
	).Scan(&id)
	if err != nil {
		log.Printf("INSERT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"message": "ok",
	})
}

// GetMeasurements GET /measurements
// 最新100件を返す（フロントエンド向け）
func GetMeasurements(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(
		`SELECT id, user_id, db, hz, latitude, longitude, created_at
		 FROM measurements
		 ORDER BY created_at DESC
		 LIMIT 100`,
	)
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

// GetLatestMeasurement GET /measurements/latest
// 最新1件を返す（リアルタイム表示用）
func GetLatestMeasurement(w http.ResponseWriter, r *http.Request) {
	var m models.Measurement
	err := db.DB.QueryRow(
		`SELECT id, user_id, db, hz, latitude, longitude, created_at
		 FROM measurements
		 ORDER BY created_at DESC
		 LIMIT 1`,
	).Scan(&m.ID, &m.UserID, &m.DB, &m.Hz, &m.Latitude, &m.Longitude, &m.CreatedAt)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "データがありません")
		return
	}
	if err != nil {
		log.Printf("QueryRow失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, m)
}

// HealthCheck GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
