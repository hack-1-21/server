package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"hack1-server/db"
	"hack1-server/models"
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

type DebugAddGardenPointsRequest struct {
	UserID string `json:"user_id"`
	Points int    `json:"points"`
}

// DebugAddGardenPoints POST /debug/garden/add-points
// テスト用に箱庭のポイントを強制的に追加し、進化・画像生成を確認するためのエンドポイント
func DebugAddGardenPoints(w http.ResponseWriter, r *http.Request) {
	var req DebugAddGardenPointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DBトランザクション開始失敗")
		return
	}
	defer tx.Rollback()

	// ユーザー作成確認
	var u models.User
	err = tx.QueryRow(
		`SELECT level, exp, total_exp FROM users WHERE user_id = $1 FOR UPDATE`,
		req.UserID,
	).Scan(&u.Level, &u.Exp, &u.TotalExp)

	if err == sql.ErrNoRows {
		u.Level = 1
		u.Exp = 0
		u.TotalExp = 0
		_, err = tx.Exec(
			`INSERT INTO users (user_id, level, exp, total_exp) VALUES ($1, $2, $3, $4)`,
			req.UserID, u.Level, u.Exp, u.TotalExp,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "ユーザー作成失敗")
			return
		}
	}

	// 箱庭取得
	var garden models.Garden
	err = tx.QueryRow(
		`SELECT id, generation, points, stage, image_url
		 FROM gardens WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY generation DESC LIMIT 1 FOR UPDATE`,
		req.UserID,
	).Scan(&garden.ID, &garden.Generation, &garden.Points, &garden.Stage, &garden.ImageURL)

	if err == sql.ErrNoRows {
		garden.UserID = req.UserID
		garden.Generation = 1
		garden.Points = 0
		garden.Stage = 1
		err = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, $3, $4, TRUE) RETURNING id`,
			req.UserID, garden.Generation, garden.Points, garden.Stage,
		).Scan(&garden.ID)
	}

	prevStage := garden.Stage
	garden.Points += req.Points
	garden.Stage = models.CalcStage(garden.Points)

	stageUp := garden.Stage > prevStage
	generationUp := garden.Points >= models.MaxPoints

	gainedExp := 0
	if stageUp {
		gainedExp = models.StageExpGain[garden.Stage]
	}
	if generationUp {
		gainedExp += models.WorldGenEXP
	}
	u.TotalExp += gainedExp
	u.Level = models.CalcUserLevel(u.TotalExp)

	// 更新
	_, _ = tx.Exec(
		`UPDATE users SET level = $1, total_exp = $2 WHERE user_id = $3`,
		u.Level, u.TotalExp, req.UserID,
	)

	if generationUp {
		historyImageURL := fmt.Sprintf("/images/gardens/%s/%s_gen%d.png", req.UserID, req.UserID, garden.Generation)
		_, _ = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2, is_active = FALSE, completed_at = NOW(), image_url = $3 WHERE id = $4`,
			garden.Points, garden.Stage, historyImageURL, garden.ID,
		)
		newGen := garden.Generation + 1
		var newID int
		_ = tx.QueryRow(
			`INSERT INTO gardens (user_id, generation, points, stage, is_active)
			 VALUES ($1, $2, 0, 1, TRUE) RETURNING id`,
			req.UserID, newGen,
		).Scan(&newID)
		garden.Generation = newGen
		garden.Points = 0
		garden.Stage = 1
		garden.ID = newID
	} else {
		_, _ = tx.Exec(
			`UPDATE gardens SET points = $1, stage = $2 WHERE id = $3`,
			garden.Points, garden.Stage, garden.ID,
		)
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "コミット失敗")
		return
	}

	if stageUp && isImageGenerationConfigured() {
		GenerateAndSaveGardenImage(garden.ID, garden.Stage, garden.Generation, req.UserID,
			func(gid int, url string) {
				db.DB.Exec(`UPDATE gardens SET image_url = $1 WHERE id = $2 AND is_active = TRUE`, url, gid)
			})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "ポイントを追加しました",
		"added_points":  req.Points,
		"total_points":  garden.Points,
		"stage":         garden.Stage,
		"generation":    garden.Generation,
		"stage_up":      stageUp,
		"generation_up": generationUp,
	})
}

// GetAllUsersDebug GET /debug/users
// テスト用に全ユーザーの user_id の一覧を取得するエンドポイント
func GetAllUsersDebug(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(`SELECT user_id FROM users ORDER BY created_at DESC`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ユーザー一覧の取得に失敗しました")
		return
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err == nil {
			users = append(users, uid)
		}
	}
	respondJSON(w, http.StatusOK, users)
}
