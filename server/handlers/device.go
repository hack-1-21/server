package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hack1-server/db"
	"hack1-server/models"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const deviceLinkTTL = 10 * time.Minute

func randomHex(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generatePairingCode() (string, error) {
	letters := make([]byte, 4)
	if _, err := rand.Read(letters); err != nil {
		return "", err
	}
	digits := make([]byte, 2)
	if _, err := rand.Read(digits); err != nil {
		return "", err
	}

	alpha := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	number := (int(digits[0])<<8 | int(digits[1])) % 10000
	return fmt.Sprintf(
		"%c%c%c%c-%04d",
		alpha[int(letters[0])%len(alpha)],
		alpha[int(letters[1])%len(alpha)],
		alpha[int(letters[2])%len(alpha)],
		alpha[int(letters[3])%len(alpha)],
		number,
	), nil
}

func normalizePairingCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 8 && !strings.Contains(code, "-") {
		code = code[:4] + "-" + code[4:]
	}
	return code
}

func deviceTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateDeviceToken() (string, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", err
	}
	return "dt_" + token, nil
}

// StartDeviceLink POST /device/start-link
func StartDeviceLink(w http.ResponseWriter, r *http.Request) {
	deviceIDPart, err := randomHex(16)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "device_id の生成に失敗しました")
		return
	}
	deviceID := "device_" + deviceIDPart
	sessionIDPart, err := randomHex(16)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "リンクセッションの生成に失敗しました")
		return
	}
	sessionID := "link_" + sessionIDPart
	expiresAt := time.Now().Add(deviceLinkTTL)

	var pairingCode string
	for i := 0; i < 5; i++ {
		pairingCode, err = generatePairingCode()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "ペアリングコードの生成に失敗しました")
			return
		}

		_, err = db.DB.Exec(
			`INSERT INTO device_link_sessions (id, device_id, pairing_code, expires_at)
			 VALUES ($1, $2, $3, $4)`,
			sessionID, deviceID, pairingCode, expiresAt,
		)
		if err == nil {
			respondJSON(w, http.StatusCreated, models.StartDeviceLinkResponse{
				DeviceID:         deviceID,
				PairingCode:      pairingCode,
				ExpiresInSeconds: int(deviceLinkTTL.Seconds()),
			})
			return
		}
		log.Printf("device link session insert failed: %v", err)
	}

	respondError(w, http.StatusInternalServerError, "ペアリングコードの保存に失敗しました")
}

// CompleteDeviceLink POST /device/complete-link
func CompleteDeviceLink(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromJWT(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}

	var req models.CompleteDeviceLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエストのJSON形式が不正です")
		return
	}

	code := normalizePairingCode(req.Code)
	if code == "" {
		respondError(w, http.StatusBadRequest, "code は必須です")
		return
	}

	result, err := db.DB.Exec(
		`UPDATE device_link_sessions
		 SET linked_user_id = $1
		 WHERE pairing_code = $2
		   AND expires_at > NOW()
		   AND linked_user_id IS NULL`,
		userID, code,
	)
	if err != nil {
		log.Printf("device link complete failed: %v", err)
		respondError(w, http.StatusInternalServerError, "デバイス連携に失敗しました")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "デバイス連携結果の確認に失敗しました")
		return
	}
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "コードが無効、期限切れ、または使用済みです")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "linked"})
}

// PollDeviceLink POST /device/poll-link
func PollDeviceLink(w http.ResponseWriter, r *http.Request) {
	var req models.PollDeviceLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエストのJSON形式が不正です")
		return
	}
	if strings.TrimSpace(req.DeviceID) == "" {
		respondError(w, http.StatusBadRequest, "device_id は必須です")
		return
	}

	var userID sql.NullString
	var expiresAt time.Time
	err := db.DB.QueryRow(
		`SELECT linked_user_id, expires_at
		 FROM device_link_sessions
		 WHERE device_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		req.DeviceID,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "リンクセッションが見つかりません")
		return
	}
	if err != nil {
		log.Printf("device link poll failed: %v", err)
		respondError(w, http.StatusInternalServerError, "リンク状態の取得に失敗しました")
		return
	}

	if !userID.Valid {
		if time.Now().After(expiresAt) {
			respondJSON(w, http.StatusOK, models.PollDeviceLinkResponse{Status: "expired"})
			return
		}
		respondJSON(w, http.StatusOK, models.PollDeviceLinkResponse{Status: "pending"})
		return
	}

	deviceToken, err := generateDeviceToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "device_token の生成に失敗しました")
		return
	}

	_, err = db.DB.Exec(
		`INSERT INTO devices (device_id, user_id, device_token_hash, linked_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (device_id) DO UPDATE
		 SET user_id = EXCLUDED.user_id,
		     device_token_hash = EXCLUDED.device_token_hash,
		     linked_at = NOW()`,
		req.DeviceID, userID.String, deviceTokenHash(deviceToken),
	)
	if err != nil {
		log.Printf("device token save failed: %v", err)
		respondError(w, http.StatusInternalServerError, "device_token の保存に失敗しました")
		return
	}

	_, _ = db.DB.Exec(
		`UPDATE device_link_sessions SET consumed_at = NOW() WHERE device_id = $1`,
		req.DeviceID,
	)

	respondJSON(w, http.StatusOK, models.PollDeviceLinkResponse{
		Status:      "linked",
		DeviceToken: deviceToken,
	})
}

// GetCurrentDevice GET /device/me
func GetCurrentDevice(w http.ResponseWriter, r *http.Request) {
	userID, deviceID, err := userIDFromDeviceToken(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "device_token が無効です")
		return
	}

	respondJSON(w, http.StatusOK, models.CurrentDeviceResponse{
		DeviceID: deviceID,
		UserID:   userID,
	})
}

// GetLinkedDevices GET /device/links
func GetLinkedDevices(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromJWT(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}

	rows, err := db.DB.Query(
		`SELECT device_id, linked_at, last_used_at
		 FROM devices
		 WHERE user_id = $1
		 ORDER BY linked_at DESC`,
		userID,
	)
	if err != nil {
		log.Printf("linked devices query failed: %v", err)
		respondError(w, http.StatusInternalServerError, "連携デバイスの取得に失敗しました")
		return
	}
	defer rows.Close()

	devices := []models.LinkedDevice{}
	for rows.Next() {
		var device models.LinkedDevice
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&device.DeviceID, &device.LinkedAt, &lastUsedAt); err != nil {
			log.Printf("linked device scan failed: %v", err)
			continue
		}
		if lastUsedAt.Valid {
			device.LastUsedAt = &lastUsedAt.Time
		}
		devices = append(devices, device)
	}

	respondJSON(w, http.StatusOK, devices)
}

// DeleteLinkedDevice DELETE /device/links/{device_id}
func DeleteLinkedDevice(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromJWT(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}

	vars := mux.Vars(r)
	deviceID := strings.TrimSpace(vars["device_id"])
	if deviceID == "" {
		respondError(w, http.StatusBadRequest, "device_id は必須です")
		return
	}

	result, err := db.DB.Exec(
		`DELETE FROM devices
		 WHERE device_id = $1
		   AND user_id = $2`,
		deviceID, userID,
	)
	if err != nil {
		log.Printf("linked device delete failed: %v", err)
		respondError(w, http.StatusInternalServerError, "デバイス連携の削除に失敗しました")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "デバイス連携の削除結果確認に失敗しました")
		return
	}
	if rowsAffected == 0 {
		respondError(w, http.StatusNotFound, "連携デバイスが見つかりません")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

// UnlinkCurrentDevice DELETE /device/unlink
func UnlinkCurrentDevice(w http.ResponseWriter, r *http.Request) {
	_, deviceID, err := userIDFromDeviceToken(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "device_token が無効です")
		return
	}

	_, err = db.DB.Exec(
		`DELETE FROM devices WHERE device_id = $1`,
		deviceID,
	)
	if err != nil {
		log.Printf("current device unlink failed: %v", err)
		respondError(w, http.StatusInternalServerError, "デバイス連携の削除に失敗しました")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

func userIDFromDeviceToken(r *http.Request) (string, string, error) {
	token, err := bearerToken(r)
	if err != nil {
		return "", "", err
	}

	var deviceID string
	var userID string
	err = db.DB.QueryRow(
		`UPDATE devices
		 SET last_used_at = NOW()
		 WHERE device_token_hash = $1
		 RETURNING device_id, user_id`,
		deviceTokenHash(token),
	).Scan(&deviceID, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", errors.New("invalid device token")
	}
	if err != nil {
		return "", "", err
	}

	return userID, deviceID, nil
}
