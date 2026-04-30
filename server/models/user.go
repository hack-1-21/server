package models

import "time"

// User はユーザーのプロフィールや設定を保持するモデル
type User struct {
	UserID       string    `json:"user_id"`
	Nickname     string    `json:"nickname"`
	Level        int       `json:"level"`
	Exp          int       `json:"exp"`
	Points       int       `json:"points"`
	AlertEnabled bool      `json:"alert_enabled"`
	Theme        string    `json:"theme"`
	CreatedAt    time.Time `json:"created_at"`
}

// UpdateUserRequest は PUT /users/{user_id} または POST /users のリクエストボディ
type UpdateUserRequest struct {
	Nickname     *string `json:"nickname,omitempty"`
	AlertEnabled *bool   `json:"alert_enabled,omitempty"`
	Theme        *string `json:"theme,omitempty"`
}
