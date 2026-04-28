package models

import "time"

// Measurement はスマートウォッチから受け取る1レコード分のデータ
type Measurement struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	DB        float64   `json:"db"`        // 音量 (デシベル)
	Hz        float64   `json:"hz"`        // 周波数 (ヘルツ)
	Latitude  float64   `json:"latitude"`  // 緯度
	Longitude float64   `json:"longitude"` // 経度
	CreatedAt time.Time `json:"created_at"`
}

// CreateMeasurementRequest は POST /measurements のリクエストボディ
type CreateMeasurementRequest struct {
	UserID    string  `json:"user_id"`
	DB        float64 `json:"db"`
	Hz        float64 `json:"hz"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CheckIn は Sound Check-In の1レコード
type CheckIn struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	DB        float64   `json:"db"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCheckInRequest は POST /checkins のリクエストボディ
type CreateCheckInRequest struct {
	UserID    string  `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	DB        float64 `json:"db"`
	Message   string  `json:"message"`
}

// HeatmapPoint はヒートマップ描画用の集計済みポイント
type HeatmapPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	AvgDB     float64 `json:"avg_db"`
	Count     int     `json:"count"`
}

// AreaMaster はエリアマスター情報
type AreaMaster struct {
	UserID     string  `json:"user_id"`
	CheckIns   int     `json:"checkins"`
	Latitude   float64 `json:"center_lat"`
	Longitude  float64 `json:"center_lng"`
	RadiusKm   float64 `json:"radius_km"`
}
