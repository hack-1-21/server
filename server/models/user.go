package models

import "time"

// User はユーザーのプロフィールや設定を保持するモデル
type User struct {
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
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

// RegisterRequest は POST /auth/register のリクエストボディ
type RegisterRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// LoginRequest は POST /auth/login のリクエストボディ
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// GoogleLoginRequest は POST /auth/google のリクエストボディ
type GoogleLoginRequest struct {
	IDToken string `json:"id_token"`
}

// AuthResponse はログイン・登録成功時のレスポンス
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
