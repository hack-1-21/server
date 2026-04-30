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

	// ---- デバッグ・開発用 ----
	r.HandleFunc("/debug/reset", handlers.ResetDatabase).Methods("DELETE")

	// ---- 認証 ----
	r.HandleFunc("/auth/register", handlers.Register).Methods("POST")
	r.HandleFunc("/auth/login", handlers.Login).Methods("POST")

	// ---- ユーザー ----
	r.HandleFunc("/users/{user_id}", handlers.GetUser).Methods("GET")
	r.HandleFunc("/users/{user_id}", handlers.UpdateUser).Methods("PUT")

	// ---- 測定データ ----
	r.HandleFunc("/measurements", handlers.CreateMeasurement).Methods("POST")
	r.HandleFunc("/measurements", handlers.GetMeasurements).Methods("GET")
	r.HandleFunc("/measurements/bbox", handlers.GetMeasurementsBBox).Methods("GET")

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
	log.Printf("  GET    /measurements        ← 測定データ全件/差分取得")
	log.Printf("  GET    /measurements/area   ← ヒートマップ用エリア検索")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("サーバー起動失敗: %v", err)
	}
}
