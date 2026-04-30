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
		 ON CONFLICT (user_id) DO UPDATE 
		 SET password_hash = EXCLUDED.password_hash, nickname = EXCLUDED.nickname
		 WHERE users.password_hash = '' OR users.password_hash IS NULL
		 RETURNING user_id`,
		u.UserID, string(hashedPassword), u.Nickname, u.Level, u.Exp, u.Points, u.AlertEnabled, u.Theme, u.CreatedAt,
	).Scan(&u.UserID)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusConflict, "そのユーザーIDは既に登録されています")
		return
	} else if err != nil {
		log.Printf("ユーザー作成エラー: %v", err)
		respondError(w, http.StatusInternalServerError, "登録に失敗しました")
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

// GoogleLogin POST /auth/google
// フロントエンドから送信された Google の id_token を検証し、ログインまたはユーザーの新規作成を行う
func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.GoogleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}

	if req.IDToken == "" {
		respondError(w, http.StatusBadRequest, "id_token は必須です")
		return
	}

	// Googleのトークン検証エンドポイントを叩く
	verifyURL := "https://oauth2.googleapis.com/tokeninfo?id_token=" + req.IDToken
	resp, err := http.Get(verifyURL)
	if err != nil {
		log.Printf("Googleトークン検証リクエスト失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "Google認証に失敗しました")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Googleトークンが無効です。ステータス: %d", resp.StatusCode)
		respondError(w, http.StatusUnauthorized, "無効な Google トークンです")
		return
	}

	var googleClaims struct {
		Sub   string `json:"sub"`   // GoogleのユーザーID
		Name  string `json:"name"`  // ユーザー名
		Email string `json:"email"` // メールアドレス (今回は使わないかもしれないが念のため)
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleClaims); err != nil {
		log.Printf("Googleトークンレスポンス解析失敗: %v", err)
		respondError(w, http.StatusInternalServerError, "Google認証レスポンスの処理に失敗しました")
		return
	}

	userID := "google_" + googleClaims.Sub
	nickname := googleClaims.Name
	if nickname == "" {
		nickname = "Google User"
	}

	// ユーザーがDBにいるか確認。いなければ作成 (UPSERT)
	var u models.User
	u.UserID = userID
	u.Nickname = nickname
	u.Level = 1
	u.Exp = 0
	u.Points = 0
	u.AlertEnabled = true
	u.Theme = "light"
	u.CreatedAt = time.Now()

	err = db.DB.QueryRow(
		`INSERT INTO users (user_id, password_hash, nickname, level, exp, points, alert_enabled, theme, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (user_id) DO UPDATE 
		 SET nickname = EXCLUDED.nickname
		 RETURNING user_id, nickname, level, exp, points, alert_enabled, theme, created_at`,
		u.UserID, "", u.Nickname, u.Level, u.Exp, u.Points, u.AlertEnabled, u.Theme, u.CreatedAt,
	).Scan(&u.UserID, &u.Nickname, &u.Level, &u.Exp, &u.Points, &u.AlertEnabled, &u.Theme, &u.CreatedAt)

	if err != nil {
		log.Printf("Googleユーザー作成/取得エラー: %v", err)
		respondError(w, http.StatusInternalServerError, "ユーザー情報の処理に失敗しました")
		return
	}

	// 独自のJWTを生成して返す (以降は通常のAPIリクエストと同じように扱える)
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

