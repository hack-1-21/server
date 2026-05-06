package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ===========================
// プロンプト設定（2Dファンタジーイラスト風）
// ===========================

// ベースとなる世界観（全ステージ共通）
// 「flat 2d vector art」「clean lines」「cel shaded」を追加してイラスト調に固定します
var BasePrompt = "Flat 2D vector art of a magical miniature world inside a glass bottle, clean lines, cel shaded, fantasy storybook illustration, vibrant colors, high contrast, %s %s background"

// ステージ1のプロンプト（シンプルに芽吹きを強調）
var Stage1Prompt = "a single tiny green sprout in the center, simple earth ground, %s watching the sprout, soft magical glow, minimalist background, generation %d"

// ステージ2のプロンプト（少し賑やかに）
var Stage2Prompt = "a healthy growing tree with round green canopy, a small rainbow inside the bottle, %s playing together, colorful flowers and grass, generation %d"

// ステージ3のプロンプト（画像のような豪華な完成形）
var Stage3Prompt = "a magnificent ancient tree with thick canopy, glowing magical fairies, multiple rainbows, %s gathered around, crystals and mushrooms on the ground, legendary atmosphere, generation %d"

// ランダム要素（イラストに合う色調に変更）
var seasons = []string{"pastel spring", "vibrant summer", "warm autumn", "cool winter"}
var weathers = []string{"clear sunny", "mystical misty", "sparkling", "starry", "twilight"}
var animals1 = []string{"a tiny rabbit", "a small deer fawn", "a hedgehog", "a baby fox"}
var animals2 = []string{"rabbits and deer", "foxes and owls", "deer and hedgehogs", "fireflies and frogs"}
var animals3 = []string{"foxes, deer, rabbits, owls, and fireflies", "wolves, deer, bears, and fairies", "unicorns, rabbits, foxes, and butterflies"}

func buildPrompt(stage, generation int) string {
	// 季節・天候は「世代（generation）」をシードにして固定
	genRand := rand.New(rand.NewSource(int64(generation * 1234567)))
	season := seasons[genRand.Intn(len(seasons))]
	weather := weathers[genRand.Intn(len(weathers))]

	// 動物は毎回ランダム
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// ベースプロンプトを構築
	base := fmt.Sprintf(BasePrompt+", ", season, weather)

	switch stage {
	case 1:
		animal := animals1[r.Intn(len(animals1))]
		return base + fmt.Sprintf(Stage1Prompt, animal, generation)
	case 2:
		animal := animals2[r.Intn(len(animals2))]
		return base + fmt.Sprintf(Stage2Prompt, animal, generation)
	case 3:
		animal := animals3[r.Intn(len(animals3))]
		return base + fmt.Sprintf(Stage3Prompt, animal, generation)
	default:
		return base + "a magical garden in full bloom"
	}
}

// ===========================
// Cloudflare Workers AI 呼び出し
// ===========================

func generateGardenImage(prompt string) ([]byte, error) {
	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")

	if accountID == "" || apiToken == "" {
		return nil, fmt.Errorf("CF_ACCOUNT_ID または CF_API_TOKEN が環境変数に設定されていません")
	}

	endpoint := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/ai/run/@cf/stabilityai/stable-diffusion-xl-base-1.0",
		accountID,
	)

	// リクエストボディ: プロンプトと画像サイズ（1:1 に設定）
	reqData := map[string]interface{}{
		"prompt": prompt,
		"width":  512,
		"height": 512,
	}
	reqBodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("JSONエンコード失敗: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("リクエスト生成失敗: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare API 呼び出し失敗: %w", err)
	}
	defer resp.Body.Close()

	// 429: レートリミット
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("Cloudflare API レートリミット超過 (429): しばらく待ってから再試行してください")
	}
	// 401/403: 認証エラー
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cloudflare API 認証エラー (%d): %s", resp.StatusCode, string(body))
	}
	// その他エラー
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cloudflare API エラー %d: %s", resp.StatusCode, string(body))
	}

	// Cloudflare Workers AI はバイナリ（PNG/JPEG）をそのまま返す
	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("レスポンス読み込み失敗: %w", err)
	}
	if len(imgData) == 0 {
		return nil, fmt.Errorf("画像が生成されませんでした（レスポンスが空）")
	}

	return imgData, nil
}

// ===========================
// ローカル保存 (Railway Volume想定)
// ===========================

func saveToLocalFile(imgData []byte, userID string, generation, stage int) (string, error) {
	// 保存先のディレクトリパス（環境変数になければデフォルト値）
	dataDir := os.Getenv("STORAGE_DIR")
	if dataDir == "" {
		dataDir = "./data/images"
	}

	// ユーザーディレクトリの作成
	userDir := filepath.Join(dataDir, "gardens", userID)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("ディレクトリ作成失敗: %w", err)
	}

	// 1. 図鑑保存用・履歴用ファイル名（例: user-001_gen1.png）
	genFileName := fmt.Sprintf("%s_gen%d.png", userID, generation)
	genFilePath := filepath.Join(userDir, genFileName)

	// 2. 現在育成中・アクティブ用ファイル名（例: user-001.png）
	// フロント側が世代管理を気にせず常に固定のURLで取得できるようにする
	activeFileName := fmt.Sprintf("%s.png", userID)
	activeFilePath := filepath.Join(userDir, activeFileName)

	// ファイル書き込み（図鑑用）
	if err := os.WriteFile(genFilePath, imgData, 0644); err != nil {
		return "", fmt.Errorf("図鑑用ファイル保存失敗: %w", err)
	}

	// ファイル書き込み（アクティブ用）
	if err := os.WriteFile(activeFilePath, imgData, 0644); err != nil {
		return "", fmt.Errorf("アクティブ用ファイル保存失敗: %w", err)
	}

	// APIレスポンスとしてDBに保存するURL（アクティブ用のパスを返す）
	// 例: /images/gardens/user-001/user-001.png
	publicURL := fmt.Sprintf("/images/gardens/%s/%s", userID, activeFileName)
	return publicURL, nil
}

// ===========================
// 非同期画像生成・保存メイン関数
// ===========================

// GenerateAndSaveGardenImage は非同期（goroutine）で呼び出す
// 生成完了後に gardens テーブルの image_url を更新する
func GenerateAndSaveGardenImage(gardenID, stage, generation int, userID string, updateImageURL func(gardenID int, imageURL string)) {
	go func() {
		prompt := buildPrompt(stage, generation)
		log.Printf("[画像生成] gardenID=%d stage=%d generation=%d prompt=%s", gardenID, stage, generation, prompt[:50])

		imgData, err := generateGardenImage(prompt)
		if err != nil {
			log.Printf("[画像生成] Cloudflare API エラー: %v", err)
			return
		}

		imageURL, err := saveToLocalFile(imgData, userID, generation, stage)
		if err != nil {
			log.Printf("[画像生成] 保存エラー: %v", err)
			return
		}

		updateImageURL(gardenID, imageURL)
		log.Printf("[画像生成] 完了: gardenID=%d url=%s", gardenID, imageURL)
	}()
}

// isImageGenerationConfigured は Cloudflare Workers AI の設定が揃っているか確認する
func isImageGenerationConfigured() bool {
	return os.Getenv("CF_ACCOUNT_ID") != "" && os.Getenv("CF_API_TOKEN") != ""
}
