package handlers

import (
	"database/sql"
	"encoding/json"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte(getJWTSecret())

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "soundreal-super-secret-key-for-dev"
	}
	return secret
}

// GenerateToken generates a JWT token for a user
func GenerateToken(userID string) (string, error) {
	expirationTime := time.Now().Add(24 * 7 * time.Hour) // 1 week
	claims := &jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// Register POST /auth/register
func Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}

	if req.UserID == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "user_id と password は必須です")
		return
	}

	// パスワードのハッシュ化
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "パスワードのハッシュ化に失敗しました")
		return
	}

	var u models.User
	u.UserID = req.UserID
	u.Nickname = req.Nickname
	if u.Nickname == "" {
		u.Nickname = "No Name"
	}
	u.Level = 1
	u.Exp = 0
	u.Points = 0
	u.AlertEnabled = true
	u.Theme = "light"
	u.CreatedAt = time.Now()

	err = db.DB.QueryRow(
		`INSERT INTO users (user_id, password_hash, nickname, level, exp, points, alert_enabled, theme, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING user_id`,
		u.UserID, string(hashedPassword), u.Nickname, u.Level, u.Exp, u.Points, u.AlertEnabled, u.Theme, u.CreatedAt,
	).Scan(&u.UserID)

	if err != nil {
		log.Printf("ユーザー作成エラー: %v", err)
		respondError(w, http.StatusConflict, "そのユーザーIDは既に存在するか、登録に失敗しました")
		return
	}

	// JWT生成
	tokenString, err := GenerateToken(u.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "トークン生成に失敗しました")
		return
	}

	respondJSON(w, http.StatusCreated, models.AuthResponse{
		Token: tokenString,
		User:  u,
	})
}

// Login POST /auth/login
func Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}

	var u models.User
	var passwordHash string

	err := db.DB.QueryRow(
		`SELECT user_id, password_hash, nickname, level, exp, points, alert_enabled, theme, created_at
		 FROM users WHERE user_id = $1`,
		req.UserID,
	).Scan(&u.UserID, &passwordHash, &u.Nickname, &u.Level, &u.Exp, &u.Points, &u.AlertEnabled, &u.Theme, &u.CreatedAt)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusUnauthorized, "ユーザーIDまたはパスワードが間違っています")
		return
	} else if err != nil {
		log.Printf("ログイン取得エラー: %v", err)
		respondError(w, http.StatusInternalServerError, "ログインに失敗しました")
		return
	}

	// ゲストユーザーの場合はパスワードチェックをスキップする等も可能ですが、
	// 基本はパスワード検証を行います。ゲスト用IDを用意してフロントから特定のパスワードでログインさせるか、
	// もしくはフロントでランダムなID/PWを生成してRegisterさせるのが簡単です。

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "ユーザーIDまたはパスワードが間違っています")
		return
	}

	// JWT生成
	tokenString, err := GenerateToken(u.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "トークン生成に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, models.AuthResponse{
		Token: tokenString,
		User:  u,
	})
}
