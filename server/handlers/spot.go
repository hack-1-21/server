package handlers

import (
	"database/sql"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// GetAreaMaster GET /areas/{radius_km}/master
//
// Query params:
//
//	lat float64 — 中心緯度
//	lng float64 — 中心経度
//
// 指定エリア内で最も多くチェックインしたユーザーを「エリアマスター」として返す。
func GetAreaMaster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	q := r.URL.Query()

	radiusKm, err := strconv.ParseFloat(vars["radius_km"], 64)
	if err != nil || radiusKm <= 0 || radiusKm > 50 {
		respondError(w, http.StatusBadRequest, "radius_km は 0〜50 の範囲で指定してください")
		return
	}
	lat, err := strconv.ParseFloat(q.Get("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		respondError(w, http.StatusBadRequest, "lat は有効な緯度（-90〜90）を指定してください")
		return
	}
	lng, err := strconv.ParseFloat(q.Get("lng"), 64)
	if err != nil || lng < -180 || lng > 180 {
		respondError(w, http.StatusBadRequest, "lng は有効な経度（-180〜180）を指定してください")
		return
	}

	degPerKm := 1.0 / 111.32
	latDelta := radiusKm * degPerKm
	lngDelta := radiusKm * degPerKm / math.Cos(lat*math.Pi/180)

	// エリア内チェックイン数が最多のユーザーを取得
	query := `
		SELECT
			user_id,
			COUNT(*) AS checkin_count
		FROM checkins
		WHERE
			latitude  BETWEEN $1 AND $2
			AND longitude BETWEEN $3 AND $4
			AND (
				2 * 6371 * asin(
					sqrt(
						power(sin(radians(latitude  - $5) / 2), 2)
						+ cos(radians($5)) * cos(radians(latitude))
						  * power(sin(radians(longitude - $6) / 2), 2)
					)
				)
			) <= $7
		GROUP BY user_id
		ORDER BY checkin_count DESC
		LIMIT 1
	`

	var master models.AreaMaster
	err = db.DB.QueryRow(query,
		lat-latDelta, lat+latDelta,
		lng-lngDelta, lng+lngDelta,
		lat, lng, radiusKm,
	).Scan(&master.UserID, &master.CheckIns)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "このエリアにチェックインがまだありません")
		return
	}
	if err != nil {
		log.Printf("エリアマスター取得失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}

	master.Latitude = lat
	master.Longitude = lng
	master.RadiusKm = radiusKm

	respondJSON(w, http.StatusOK, master)
}
