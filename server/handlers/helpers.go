package handlers

import (
	"encoding/json"
	"net/http"
)

// respondJSON は JSON レスポンスを書き出す共通ヘルパー
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// respondError はエラー JSON レスポンスを書き出す共通ヘルパー
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
