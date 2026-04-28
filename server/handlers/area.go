package handlers

import (
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"math"
	"net/http"
	"strconv"
)

// GetMeasurementsArea GET /measurements/area
//
// Query params:
//
//	lat    float64 — 中心緯度
//	lng    float64 — 中心経度
//	radius float64 — 半径 (km)、デフォルト 1.0
//
// 指定した円内の測定値を latitude/longitude でグループ集計し、
// ヒートマップ用ポイント一覧として返す。
//
// PostGIS なしで動くよう、PostgreSQL の算術演算（Haversine 公式）で
// 距離フィルタを実装している。まずバウンディングボックスでインデックスを利用し、
// そのあと正確な球面距離で再フィルタすることで効率を両立する。
func GetMeasurementsArea(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

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

	radius := 1.0
	if rStr := q.Get("radius"); rStr != "" {
		radius, err = strconv.ParseFloat(rStr, 64)
		if err != nil || radius <= 0 {
			respondError(w, http.StatusBadRequest, "radius は正の数（km）を指定してください")
			return
		}
	}
	if radius > 50 {
		radius = 50 // DoS 対策: 最大50km
	}

	// 1度あたり ≒ 111.32 km でバウンディングボックスを計算
	degPerKm := 1.0 / 111.32
	latDelta := radius * degPerKm
	lngDelta := radius * degPerKm / math.Cos(lat*math.Pi/180)

	// Haversine 公式を PostgreSQL の算術で実装。
	// バウンディングボックスで先に絞り込み（インデックス活用）→ 距離で再フィルタ。
	query := `
		SELECT
			latitude,
			longitude,
			AVG(db)  AS avg_db,
			COUNT(*) AS cnt
		FROM measurements
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
		GROUP BY latitude, longitude
		LIMIT 500
	`

	rows, err := db.DB.Query(query,
		lat-latDelta, lat+latDelta,
		lng-lngDelta, lng+lngDelta,
		lat, lng, radius,
	)
	if err != nil {
		log.Printf("エリア検索失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}
	defer rows.Close()

	points := []models.HeatmapPoint{}
	for rows.Next() {
		var p models.HeatmapPoint
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.AvgDB, &p.Count); err != nil {
			log.Printf("Scan失敗: %v", err)
			continue
		}
		points = append(points, p)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"center":    map[string]float64{"lat": lat, "lng": lng},
		"radius_km": radius,
		"count":     len(points),
		"points":    points,
	})
}
