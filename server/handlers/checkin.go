package handlers

import (
	"encoding/json"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
)

// CreateCheckIn POST /checkins
// BeReal 風の Sound Check-In を投稿する
func CreateCheckIn(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCheckInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエストのJSON形式が不正です")
		return
	}

	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id は必須です")
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
		`INSERT INTO checkins (user_id, latitude, longitude, db, message)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.UserID, req.Latitude, req.Longitude, req.DB, req.Message,
	).Scan(&id)
	if err != nil {
		log.Printf("チェックインINSERT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "DB保存に失敗しました")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"message": "ok",
	})
}

// GetCheckIns GET /checkins
// 最新50件のチェックイン一覧を返す（SNSフィード）
func GetCheckIns(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(
		`SELECT id, user_id, latitude, longitude, db, message, created_at
		 FROM checkins
		 ORDER BY created_at DESC
		 LIMIT 50`,
	)
	if err != nil {
		log.Printf("チェックインSELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}
	defer rows.Close()

	checkins := []models.CheckIn{}
	for rows.Next() {
		var c models.CheckIn
		if err := rows.Scan(&c.ID, &c.UserID, &c.Latitude, &c.Longitude, &c.DB, &c.Message, &c.CreatedAt); err != nil {
			log.Printf("Scan失敗: %v", err)
			continue
		}
		checkins = append(checkins, c)
	}

	respondJSON(w, http.StatusOK, checkins)
}
