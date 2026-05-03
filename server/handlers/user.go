package handlers

import (
	"database/sql"
	"encoding/json"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// GetUser GET /users/{user_id}
// ユーザー情報を取得する。存在しない場合は新規作成して返す（認証基盤がまだないため）
func GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	var u models.User
	err := db.DB.QueryRow(
		`SELECT user_id, email, nickname, level, exp, points, alert_enabled, theme, created_at
		 FROM users WHERE user_id = $1`,
		userID,
	).Scan(&u.UserID, &u.Email, &u.Nickname, &u.Level, &u.Exp, &u.Points, &u.AlertEnabled, &u.Theme, &u.CreatedAt)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "ユーザーが見つかりません")
		return
	} else if err != nil {
		log.Printf("ユーザー取得失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー取得に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, u)
}

// UpdateUser PUT /users/{user_id}
// ユーザー情報を更新する（ニックネーム、設定など）
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエストのJSON形式が不正です")
		return
	}

	// 現在のユーザー情報を取得
	var u models.User
	err := db.DB.QueryRow(
		`SELECT user_id, email, nickname, alert_enabled, theme FROM users WHERE user_id = $1`,
		userID,
	).Scan(&u.UserID, &u.Email, &u.Nickname, &u.AlertEnabled, &u.Theme)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "ユーザーが見つかりません")
		return
	} else if err != nil {
		log.Printf("ユーザー取得失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "データ取得に失敗しました")
		return
	}

	// 値の更新
	if req.Nickname != nil {
		u.Nickname = *req.Nickname
	}
	if req.AlertEnabled != nil {
		u.AlertEnabled = *req.AlertEnabled
	}
	if req.Theme != nil {
		u.Theme = *req.Theme
	}

	_, err = db.DB.Exec(
		`UPDATE users SET nickname = $1, alert_enabled = $2, theme = $3 WHERE user_id = $4`,
		u.Nickname, u.AlertEnabled, u.Theme, userID,
	)
	if err != nil {
		log.Printf("ユーザー更新失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の更新に失敗しました")
		return
	}

	// 更新後のデータを返すため再度取得
	GetUser(w, r)
}
