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
// プロンプト設定（画角固定・正統進化ガチガチ版）
// ===========================

// 1. カメラ位置と瓶の形を完全に固定するベースプロンプト
var BasePrompt = "straight-on front view, perfect centered composition, a single symmetrical glass potion bottle in the exact center, flat 2D vector art, clean lines, cel shaded, fantasy storybook illustration, simple minimalist background"

// 2. 世代(generation)ごとの動物の進化ツリー（Stage1 -> Stage2 -> Stage3）
// これにより「途中で動物がすり替わる」のを防ぎ、正統進化を演出します
var animalEvolutions = [][]string{
	{"a tiny sleeping baby fox", "a playful young fox", "a majestic adult fox with glowing magical tails"},
	{"a small baby deer fawn", "a proud young deer", "a legendary stag with glowing crystal antlers"},
	{"a tiny fluffy baby rabbit", "a quick young rabbit", "a mythical guardian rabbit with star-like glowing ears"},
}

func buildPrompt(stage, generation int) string {
	// 世代番号を元に、どの動物の血統（ツリー）にするか決定
	// generation=1ならキツネ、2ならシカ、3ならウサギ...（ループします）
	animalTreeIndex := (generation - 1) % len(animalEvolutions)
	animalTree := animalEvolutions[animalTreeIndex]

	// 進化段階に合わせて動物の姿（文字列）を取得
	// stageは1, 2, 3を想定しているので、インデックスは stage-1
	animalIndex := stage - 1
	if animalIndex < 0 {
		animalIndex = 0
	}
	if animalIndex > 2 {
		animalIndex = 2
	}
	currentAnimal := animalTree[animalIndex]

	// 季節や天候も世代で固定し、Stageが変わっても背景の雰囲気がブレないようにする
	genRand := rand.New(rand.NewSource(int64(generation * 9999)))
	seasons := []string{"spring season", "summer season", "autumn season"}
	season := seasons[genRand.Intn(len(seasons))]

	// ステージごとの内部の描写（瓶の中身の進化）
	var stageDescription string
	switch stage {
	case 1:
		stageDescription = fmt.Sprintf("inside the bottle: a small patch of dirt, a tiny green sprout, %s resting near the sprout", currentAnimal)
	case 2:
		stageDescription = fmt.Sprintf("inside the bottle: a healthy growing sapling tree, %s standing proudly next to the tree, small glowing flowers", currentAnimal)
	case 3:
		stageDescription = fmt.Sprintf("inside the bottle: a giant ancient magical tree filling the glass, %s guarding the tree, brilliant glowing crystals, a small rainbow", currentAnimal)
	default:
		stageDescription = fmt.Sprintf("inside the bottle: a magical garden, %s", currentAnimal)
	}

	// 最終的なプロンプトの組み立て
	// [画角とスタイルの固定] + [季節] + [瓶の中身と動物の進化]
	finalPrompt := fmt.Sprintf("%s, %s, %s", BasePrompt, season, stageDescription)

	return finalPrompt
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

	// ========== ここから i2i ロジック ==========
	// Stage 2 以上なら、現在アクティブな画像を読み込んでベースにする
	if stage > 1 {
		dataDir := os.Getenv("STORAGE_DIR")
		if dataDir == "" {
			dataDir = "./data/images"
		}
		// 前のステージの画像を読み込む
		activeFilePath := filepath.Join(dataDir, "gardens", userID, fmt.Sprintf("%s.png", userID))

		imgBytes, err := os.ReadFile(activeFilePath)
		if err == nil {
			// Cloudflare Workers AI の REST API は、画像を数値(uint8)の配列として受け取ります
			imgInts := make([]int, len(imgBytes))
			for i, b := range imgBytes {
				imgInts[i] = int(b)
			}

			reqData["image"] = imgInts

			// strength (0.0〜1.0)
			// 値が小さいほど元の画像を維持し、大きいほどプロンプトに従って大きく変化します。
			// 0.65 前後が「元の構図を残しつつ成長させる」のに適しています。
			reqData["strength"] = 0.65

			log.Printf("[i2i] 前の画像をベースに生成します (strength: 0.65)")
		} else {
			log.Printf("[i2i] 前の画像が見つからないため、新規(txt2img)で生成します: %v", err)
		}
	}
	// ========== ここまで ==========

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
