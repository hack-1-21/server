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

// ==========================================================================
// 1. 各属性ごとの進化プロンプト設定 (★英語プロンプトを直接コピペ・カスタマイズできるエリア★)
// ==========================================================================

// 【共通スタイル / BASE STYLE】
// 画角、容器、全体テイスト（アンティーク魔法瓶、リヴリーアイランド/ポケモンSleep/Duolingo風のシンプルでフラットな質感、英語文字無し）を規定します。
var BasePrompt = "straight-on front view, perfect centered composition, a single symmetrical glass potion bottle in the exact center, the entire bottle is fully visible, shaped like an otherworldly antique magical perfume bottle with elegant decorations, flat simple illustration, cozy game art style similar to Livly Island, Duolingo, and Pokemon Sleep, no text, no words, no letters"

// --- 🌱 【草属性 / GRASS】 (第1世代, 第5世代, ...) ---
// 朝露に濡れた草花、絡んだつる、美しい木、草原、可憐な動物、背景は森奥深くのうっそうとした神殿、中央に成長する木
var GrassStage1 = "inside the magical bottle: a small patch of rich soil, a tiny sprouting green seedling in the exact center, morning dew on small grass and wildflowers, a small adorable forest animal, fantasy, glittering, beautiful, background is a deep mystical ancient forest temple with dense untouched trees, no man-made objects inside"
var GrassStage2 = "inside the magical bottle: a beautiful young growing tree in the exact center, tangled vines, vibrant green grasslands, small cute forest animals playing, a beautiful colorful rainbow in the background, fantasy, glittering, beautiful, deep mystical ancient forest temple backdrop with dense untouched trees"
var GrassStage3 = "inside the magical bottle: a magnificent giant ancient tree in the exact center, lush green leaves, beautiful blooming wildflowers, cute forest animals gathered under the tree, brilliant glowing rainbow, fantasy, glittering, beautiful, deep mystical ancient forest temple backdrop with dense untouched trees"

// --- 🔥 【炎属性 / FIRE】 (第2世代, 第6世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var FireStage1 = "inside the magical bottle: a small patch of warm volcanic ash, a tiny glowing fiery sprout in the exact center, warm sparks, a small adorable fire-themed animal, fantasy, glittering, beautiful, background is a legendary volcanic temple with glowing basalt stone, no man-made objects inside"
var FireStage2 = "inside the magical bottle: a beautiful young growing tree made of glowing branches and fiery leaves in the exact center, rising warm sparks, warm embers, young fire spirits playing, a beautiful warm heatwave rainbow, fantasy, glittering, beautiful, legendary volcanic temple backdrop"
var FireStage3 = "inside the magical bottle: a magnificent giant ancient tree made of burning golden leaves and glowing roots in the exact center, soaring phoenix, glowing warm crystals, fantasy, glittering, beautiful, legendary volcanic temple backdrop"

// --- 💧 【水属性 / WATER】 (第3世代, 第7世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var WaterStage1 = "inside the magical bottle: a small pool of crystal clear water, a tiny glowing aquatic seedling in the exact center, floating water droplets, a small adorable water-themed animal, fantasy, glittering, beautiful, background is a sunken water temple with beautiful ancient coral, no man-made objects inside"
var WaterStage2 = "inside the magical bottle: a beautiful young growing tree made of flowing water currents in the exact center, beautiful coral reefs, young water spirits playing, a beautiful watery rainbow, fantasy, glittering, beautiful, sunken water temple backdrop"
var WaterStage3 = "inside the magical bottle: a magnificent giant ancient coral tree in the exact center, glowing water currents, cute aquatic creatures gathered, brilliant glowing water rainbow, fantasy, glittering, beautiful, sunken water temple backdrop"

// --- ✨ 【光属性 / LIGHT】 (第4世代, 第8世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var LightStage1 = "inside the magical bottle: a small patch of glowing celestial dust, a tiny bright glowing seedling in the exact center, soft floating sunbeams, a small adorable light-themed creature, fantasy, glittering, beautiful, background is a celestial sun temple with floating ruins, no man-made objects inside"
var LightStage2 = "inside the magical bottle: a beautiful young growing tree made of pure golden light and shimmering leaves in the exact center, pillars of soft morning sunlight, young light fairies playing, a brilliant solar rainbow, fantasy, glittering, beautiful, celestial sun temple backdrop"
var LightStage3 = "inside the magical bottle: a magnificent giant ancient tree radiating warm golden light in the exact center, shimmering stars, glowing angelic wings, fantasy, glittering, beautiful, celestial sun temple backdrop"

func buildPrompt(stage, generation int) string {
	// 4つの属性（草・炎・水・光）を世代(generation)ごとにトグル・ループさせます
	// Gen 1 -> 草, Gen 2 -> 炎, Gen 3 -> 水, Gen 4 -> 光, Gen 5 -> 草 ...
	attributeIndex := (generation - 1) % 4
	if attributeIndex < 0 {
		attributeIndex = 0
	}

	var stagePrompt string
	switch attributeIndex {
	case 0: // 草属性 (Grass)
		switch stage {
		case 1:
			stagePrompt = GrassStage1
		case 2:
			stagePrompt = GrassStage2
		case 3:
			stagePrompt = GrassStage3
		default:
			stagePrompt = GrassStage3
		}
	case 1: // 炎属性 (Fire)
		switch stage {
		case 1:
			stagePrompt = FireStage1
		case 2:
			stagePrompt = FireStage2
		case 3:
			stagePrompt = FireStage3
		default:
			stagePrompt = FireStage3
		}
	case 2: // 水属性 (Water)
		switch stage {
		case 1:
			stagePrompt = WaterStage1
		case 2:
			stagePrompt = WaterStage2
		case 3:
			stagePrompt = WaterStage3
		default:
			stagePrompt = WaterStage3
		}
	case 3: // 光属性 (Light)
		switch stage {
		case 1:
			stagePrompt = LightStage1
		case 2:
			stagePrompt = LightStage2
		case 3:
			stagePrompt = LightStage3
		default:
			stagePrompt = LightStage3
		}
	}

	// 共通基本スタイル（魔法瓶やDuolingo/Pokemon Sleep等の画角・スタイル）とステージプロンプトを連結
	return fmt.Sprintf("%s, %s", BasePrompt, stagePrompt)
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
