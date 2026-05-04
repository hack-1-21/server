package handlers

import (
	"database/sql"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// GetActiveGarden GET /users/{user_id}/garden
// 現在のアクティブな箱庭（画像URL・ポイント・段階・世代）を取得
func GetActiveGarden(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["user_id"]

	var g models.Garden
	err := db.DB.QueryRow(
		`SELECT id, user_id, generation, points, stage, image_url, is_active, created_at
		 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY generation DESC LIMIT 1`,
		userID,
	).Scan(&g.ID, &g.UserID, &g.Generation, &g.Points, &g.Stage, &g.ImageURL, &g.IsActive, &g.CreatedAt)

	if err == sql.ErrNoRows {
		// まだ箱庭がなければ空のデフォルト値を返す
		respondJSON(w, http.StatusOK, models.Garden{
			UserID:     userID,
			Generation: 1,
			Points:     0,
			Stage:      1,
			ImageURL:   "",
			IsActive:   true,
		})
		return
	} else if err != nil {
		log.Printf("garden SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "箱庭情報の取得に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, g)
}

// GetGardenHistory GET /users/{user_id}/garden/history
// 図鑑：過去世代の箱庭一覧を取得（完了済みのみ）
func GetGardenHistory(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["user_id"]

	rows, err := db.DB.Query(
		`SELECT id, user_id, generation, points, stage, image_url, is_active, completed_at, created_at
		 FROM gardens WHERE user_id = $1 AND is_active = FALSE
		 ORDER BY generation ASC`,
		userID,
	)
	if err != nil {
		log.Printf("garden history SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "図鑑データの取得に失敗しました")
		return
	}
	defer rows.Close()

	gardens := []models.Garden{}
	for rows.Next() {
		var g models.Garden
		if err := rows.Scan(
			&g.ID, &g.UserID, &g.Generation, &g.Points, &g.Stage,
			&g.ImageURL, &g.IsActive, &g.CompletedAt, &g.CreatedAt,
		); err != nil {
			log.Printf("garden history Scan失敗: %v", err)
			continue
		}
		gardens = append(gardens, g)
	}

	respondJSON(w, http.StatusOK, gardens)
}

// GetProfile GET /users/{user_id}/profile
// ユーザーレベル・EXP・現在の箱庭をまとめて取得
func GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["user_id"]

	// ユーザー情報取得
	var u models.User
	err := db.DB.QueryRow(
		`SELECT user_id, email, nickname, level, exp, total_exp, alert_enabled, theme, created_at
		 FROM users WHERE user_id = $1`,
		userID,
	).Scan(&u.UserID, &u.Email, &u.Nickname, &u.Level, &u.Exp, &u.TotalExp, &u.AlertEnabled, &u.Theme, &u.CreatedAt)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "ユーザーが見つかりません")
		return
	} else if err != nil {
		log.Printf("profile user SELECT失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の取得に失敗しました")
		return
	}

	// アクティブ箱庭取得（なければ nil）
	var garden *models.Garden
	var g models.Garden
	err = db.DB.QueryRow(
		`SELECT id, user_id, generation, points, stage, image_url, is_active, created_at
		 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY generation DESC LIMIT 1`,
		userID,
	).Scan(&g.ID, &g.UserID, &g.Generation, &g.Points, &g.Stage, &g.ImageURL, &g.IsActive, &g.CreatedAt)

	if err == nil {
		garden = &g
	} else if err != sql.ErrNoRows {
		log.Printf("profile garden SELECT失敗: %v", err)
	}

	respondJSON(w, http.StatusOK, models.ProfileResponse{
		User:   u,
		Garden: garden,
	})
}
