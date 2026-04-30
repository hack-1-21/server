package main

import (
	"hack1-server/db"
	"hack1-server/handlers"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// DB初期化（環境変数 DATABASE_URL で接続先を指定）
	db.Init()

	r := mux.NewRouter()

	// ---- ヘルスチェック ----
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET")

	// ---- ユーザー ----
	r.HandleFunc("/users/{user_id}", handlers.GetUser).Methods("GET")
	r.HandleFunc("/users/{user_id}", handlers.UpdateUser).Methods("PUT")

	// ---- 測定データ ----
	r.HandleFunc("/measurements", handlers.CreateMeasurement).Methods("POST")
	r.HandleFunc("/measurements", handlers.GetMeasurements).Methods("GET")
	r.HandleFunc("/measurements/latest", handlers.GetLatestMeasurement).Methods("GET")
	r.HandleFunc("/measurements/area", handlers.GetMeasurementsArea).Methods("GET")

	// ---- チェックイン（SNSフィード）----
	r.HandleFunc("/checkins", handlers.CreateCheckIn).Methods("POST")
	r.HandleFunc("/checkins", handlers.GetCheckIns).Methods("GET")

	// ---- エリアマスター ----
	// radius_km: エリア半径 (km)、lat/lng はクエリパラメータ
	r.HandleFunc("/areas/{radius_km}/master", handlers.GetAreaMaster).Methods("GET")

	// CORS設定（React Native / Expo から叩くために必須）
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	})
	handler := c.Handler(r)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("サーバー起動: http://localhost%s", addr)
	log.Printf("エンドポイント一覧:")
	log.Printf("  GET    /health")
	log.Printf("  POST   /measurements        ← WearOS からデータ送信")
	log.Printf("  GET    /measurements        ← 最新100件")
	log.Printf("  GET    /measurements/latest ← 最新1件")
	log.Printf("  GET    /measurements/area   ← ヒートマップ用エリア検索")
	log.Printf("  POST   /checkins            ← Sound Check-In 投稿")
	log.Printf("  GET    /checkins            ← SNS フィード")
	log.Printf("  GET    /areas/{r}/master    ← エリアマスター取得")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("サーバー起動失敗: %v", err)
	}
}
