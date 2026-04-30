package handlers

import (
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
	"strconv"
)

// GetMeasurementsBBox GET /measurements/bbox
//
// Query params:
//   ne_lat  float64 — 北東の緯度
//   ne_lng  float64 — 北東の経度
//   sw_lat  float64 — 南西の緯度
//   sw_lng  float64 — 南西の経度
//   user_id string  — (任意) 指定するとそのユーザーのデータのみ取得
//
// 指定したバウンディングボックス（見えている範囲）内の測定値をすべて返す。
func GetMeasurementsBBox(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	neLat, err := strconv.ParseFloat(q.Get("ne_lat"), 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ne_lat を正しく指定してください")
		return
	}
	neLng, err := strconv.ParseFloat(q.Get("ne_lng"), 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ne_lng を正しく指定してください")
		return
	}
	swLat, err := strconv.ParseFloat(q.Get("sw_lat"), 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "sw_lat を正しく指定してください")
		return
	}
	swLng, err := strconv.ParseFloat(q.Get("sw_lng"), 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "sw_lng を正しく指定してください")
		return
	}

	userID := q.Get("user_id")

	query := `
		SELECT
			id, user_id, db, hz, latitude, longitude, created_at
		FROM measurements
		WHERE
			latitude >= $1 AND latitude <= $2
			AND longitude >= $3 AND longitude <= $4
	`

	args := []interface{}{
		swLat, neLat, swLng, neLng,
	}

	if userID != "" {
		query += ` AND user_id = $5`
		args = append(args, userID)
	}

	// 新しい順、または古い順？全て取得するので古い順（ASC）か新しい順（DESC）か。今回は全取得なのでASC
	query += ` ORDER BY created_at ASC`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		log.Printf("BBox検索失敗: %v", err)
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
