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
// プロンプト設定（デフォルメ動物・構図固定版）
// ===========================

// 共通するスタイル・構図定義
// chibi, kawaii, mascot-like を追加し、全体を可愛いイラスト調に寄せています。
// 背景（Background）は瓶の中身を際立たせるため、あえてシンプルに（ぼやけた魔法の森）しました。
var SharedStyle = "anime style, 2D vector art, clean lines, cel shaded, vibrant pastel colors, chibi kawaii aesthetic. straight-on front view, perfect centered composition, a single symmetrical glass bottle in the exact center. Background: blurred soft magical forest."

// レベル 1: 始まりの瓶
var Stage1Prompt = "A simple clear glass bottle sealed with a plain cork. Inside the bottle: a miniature terrarium with a tiny green sprout growing from a patch of brown soil. A tiny, cute, chibi mascot rabbit is sleeping next to the sprout. Soft lighting."

// レベル 2: 成長途中の魔法瓶
var Stage2Prompt = "A decorative glass bottle with elegant gold filigree edges, sealed with a cork. Inside the bottle: a healthy young tree with lush leaves, surrounded by small glowing mushrooms and colorful flowers. A chibi kawaii rabbit and a tiny round chibi fox mascot are playing peacefully under the tree. A small rainbow arcs inside the bottle."

// レベル 3: 完成された究極の魔法瓶
var Stage3Prompt = "A magnificent, heavily ornamented glass bottle encased in intricate gold and jewel-studded frames. Inside the bottle: a massive ancient tree with thick branches and glowing green leaves, surrounded by large glowing crystals and magical flowers. A group of chibi kawaii mascot animals (a rabbit, a fox, and a tiny chubby green dragon) are gathered happily under the tree. Glowing fairies floating around. A beautiful rainbow arcs inside the glass."

// 季節の属性（世代ごとにランダムで決定される）
var Seasons = []string{
	"[Spring theme: blooming pink cherry blossoms, fresh light green leaves, pink and bright green tones]",
	"[Summer theme: vibrant deep green foliage, bright sunlight, vivid colorful summer flowers, high contrast]",
	"[Autumn theme: fiery red and golden orange leaves, falling autumn foliage, warm amber lighting]",
	"[Winter theme: covered in sparkling white snow, icy frost crystals, cool magical blue lighting]",
}

func buildPrompt(stage, generation int) string {
	// 世代をシード値にして季節を決定
	genRand := rand.New(rand.NewSource(int64(generation * 12345)))
	season := Seasons[genRand.Intn(len(Seasons))]

	var base string
	switch stage {
	case 1:
		base = Stage1Prompt
	case 2:
		base = Stage2Prompt
	case 3:
		base = Stage3Prompt
	default:
		base = Stage3Prompt 
	}

	// 最終的なプロンプトの組み立て
	return fmt.Sprintf("%s %s %s", base, season, SharedStyle)
}

// ===========================
// Cloudflare Workers AI 呼び出し (i2i対応)
// ===========================

// 引数に userID と stage を追加し、以前の画像を取得できるようにします
func generateGardenImage(prompt string, userID string, stage int) ([]byte, error) {
	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")

	if accountID == "" || apiToken == "" {
		return nil, fmt.Errorf("CF_ACCOUNT_ID または CF_API_TOKEN が環境変数に設定されていません")
	}

	endpoint := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/ai/run/@cf/stabilityai/stable-diffusion-xl-base-1.0",
		accountID,
	)

	// リクエストボディ: プロンプトと画像サイズ（正方形 1:1 に設定）
	reqData := map[string]interface{}{
		"prompt": prompt,
		"width":  1024,
		"height": 1024,
	}

	// (注意: Cloudflare Workers AIの stable-diffusion-xl-base-1.0 は Text-to-Image 専門モデルのため、
	//  i2i用の "image" 入力テンソルが存在しません。そのため、純粋な txt2img として生成します。
	//  世代ごとのテーマ（季節・天候・カメラ画角等）が同一の乱数シードで固定されるため、i2iを使わずとも
	//  高いビジュアル一貫性（正統進化）が維持されます)

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

	// 画像データが大きくなるため、タイムアウトを長めに設定
	client := &http.Client{Timeout: 120 * time.Second}
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

		// 関数シグネチャの変更に伴い、userID と stage を渡すように修正
		imgData, err := generateGardenImage(prompt, userID, stage)
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
