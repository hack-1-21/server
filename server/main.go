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
	r.HandleFunc("/auth/google", handlers.GoogleLogin).Methods("POST")

	// ---- デバイス連携 ----
	r.HandleFunc("/device/start-link", handlers.StartDeviceLink).Methods("POST")
	r.HandleFunc("/device/complete-link", handlers.CompleteDeviceLink).Methods("POST")
	r.HandleFunc("/device/poll-link", handlers.PollDeviceLink).Methods("POST")
	r.HandleFunc("/device/me", handlers.GetCurrentDevice).Methods("GET")
	r.HandleFunc("/device/links", handlers.GetLinkedDevices).Methods("GET")
	r.HandleFunc("/device/links/{device_id}", handlers.DeleteLinkedDevice).Methods("DELETE")
	r.HandleFunc("/device/unlink", handlers.UnlinkCurrentDevice).Methods("DELETE")

	// ---- ユーザー ----
	r.HandleFunc("/users/{user_id}", handlers.GetUser).Methods("GET")
	r.HandleFunc("/users/{user_id}", handlers.UpdateUser).Methods("PUT")

	// ---- 測定データ ----
	r.HandleFunc("/measurements", handlers.CreateMeasurement).Methods("POST")
	r.HandleFunc("/measurements", handlers.GetMeasurements).Methods("GET")
	r.HandleFunc("/measurements/bbox", handlers.GetMeasurementsBBox).Methods("GET")

	// ---- ルート ----
	r.HandleFunc("/routes/quiet", handlers.GetQuietRoutes).Methods("POST")

	// ---- 箱庭 ----
	r.HandleFunc("/users/{user_id}/garden", handlers.GetActiveGarden).Methods("GET")
	r.HandleFunc("/users/{user_id}/garden/history", handlers.GetGardenHistory).Methods("GET")
	r.HandleFunc("/users/{user_id}/profile", handlers.GetProfile).Methods("GET")

	// ---- デバッグ ----
	r.HandleFunc("/debug/reset", handlers.ResetDatabase).Methods("DELETE")
	r.HandleFunc("/debug/users", handlers.GetAllUsersDebug).Methods("GET")
	r.HandleFunc("/debug/measurements/bulk", handlers.DebugBulkMeasurements).Methods("POST")
	r.HandleFunc("/debug/measurements/tokyo-random", handlers.DebugTokyoRandomMeasurements).Methods("POST")
	r.HandleFunc("/debug/measurements/delete", handlers.DebugDeleteMeasurements).Methods("POST")
	r.HandleFunc("/debug/garden/generate-initial", handlers.DebugGenerateInitialGarden).Methods("POST")
	r.HandleFunc("/debug/garden/add-points", handlers.DebugAddGardenPoints).Methods("POST")
	r.HandleFunc("/debug/test-cloudflare", handlers.TestCloudflareDebug).Methods("GET")
	r.HandleFunc("/debug/check-cf-config", handlers.CheckCFConfigDebug).Methods("GET")

	// ---- 画像の静的配信 (Railway Volume等) ----
	dataDir := os.Getenv("STORAGE_DIR")
	if dataDir == "" {
		dataDir = "./data/images"
	}
	r.PathPrefix("/images/").Handler(http.StripPrefix("/images/", http.FileServer(http.Dir(dataDir))))

	// CORS設定（React Native / Expo から叩くために必須）
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
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
