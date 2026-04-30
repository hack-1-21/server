package handlers

import (
	"hack1-server/db"
	"log"
	"net/http"
)

// ResetDatabase DELETE /debug/reset
// ハッカソンや開発中にデータベースの全データを削除してリセットするためのエンドポイント。
// users テーブルと measurements テーブルを空にします。
func ResetDatabase(w http.ResponseWriter, r *http.Request) {
	// 全テーブルのデータを削除（IDなどはリセットされる）
	_, err := db.DB.Exec(`TRUNCATE TABLE measurements, users RESTART IDENTITY CASCADE`)
	if err != nil {
		log.Printf("データベースリセット失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データベースのリセットに失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "すべてのデータを正常にリセットしました",
	})
}
