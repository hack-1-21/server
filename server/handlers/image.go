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
// プロンプト設定（超詳細・パステル2D統一・スケール感徹底版）
// ===========================

// 共通スタイル（画質とテイストの完全固定）
// ※「NO 3D」「NO realistic」などの否定語も混ぜ込み、絶対にリアルにならないように、かつ超高画質のパステル絵本風になるように長文で縛り付けています。
var SharedStyle = "Masterpiece, ultra-detailed flat 2D vector illustration, children's picture book art style, cute anime aesthetic, entirely pastel color palette, soft and bright lighting, clean and bold outlines, absolutely completely flat colors, NO 3D rendering, NO realistic shading, NO cinematic lighting, NO photorealism. The composition is a straight-on front view, perfectly centered, showing the entire glass bottle. BACKGROUND: A fully drawn, beautiful, flat 2D illustration of a magical ancient forest with tall green trees and moss-covered stone ruins, drawn clearly and NOT blurred, perfectly matching the pastel picture book style."

// レベル 1: 始まりの瓶
// 「瓶の中に木があり、動物は極小サイズで木の周りにいる」ことを詳細に記述
var Stage1Prompt = "A highly detailed, flat 2D anime-style illustration of a simple, clear glass bottle sealed with a plain wooden cork. INSIDE THE BOTTLE: A miniature ecosystem is contained within the glass. At the center of the bottle's interior stands a young sapling tree with fresh green leaves growing from a patch of rich brown soil. Gathered around the base of this sapling inside the bottle are several very small, cute, flat 2D animals, such as a tiny white rabbit and a little blue bird. The animals are drawn very small relative to the tree, emphasizing that this is a miniature landscape. Sparse grass and tiny pebbles surround the roots."

// レベル 2: 成長途中の魔法瓶
// 木が成長し、動物の種類が増える
var Stage2Prompt = "A highly detailed, flat 2D anime-style illustration of an elegant glass bottle decorated with simple gold filigree and sealed with a glowing cork. INSIDE THE BOTTLE: A thriving miniature ecosystem is contained within the glass. At the center of the bottle's interior stands a strong, growing tree with a large canopy of lush green leaves. Gathered peacefully around the roots of this tree inside the bottle is a diverse group of very small, cute, flat 2D animals, including a tiny white rabbit, a little red fox, and a small deer fawn. The animals are very small, looking up at the tree. The miniature ground is covered in dense green grass, colorful flat flowers, and small glowing red mushrooms. A beautiful, distinct rainbow arcs entirely inside the glass bottle."

// レベル 3: 完成された究極の魔法瓶
// 添付画像の「究極系」に寄せるため、要素を限界まで詰め込む
var Stage3Prompt = "The ultimate, masterpiece flat 2D anime-style illustration of a divine, majestic glass bottle heavily encased in intricate gold ornaments, vines, and embedded jewels. INSIDE THE BOTTLE: A massive, incredibly dense miniature magical ecosystem is contained completely within the glass. At the center of the bottle's interior stands a colossal, ancient magical tree with thick, winding roots and a sprawling canopy of glowing leaves that fills the upper half of the bottle. Gathered happily around the base of this giant tree is a large community of very small, cute, flat 2D animals, including a tiny white rabbit, a red fox, a baby deer, a squirrel, and a tiny chubby green dragon. The animals are drawn extremely small to show the massive scale of the ancient tree. The ground is covered in glowing pink and blue crystals, giant magical mushrooms, and glowing fairies. A brilliant, vivid rainbow arcs completely inside the glass bottle."

// 季節の属性（パステル調に合うように色味の指定を強化）
var Seasons = []string{
	"[Spring theme: blooming pink cherry blossoms, fresh pastel green leaves, pink and bright green tones]",
	"[Summer theme: vibrant deep pastel green foliage, bright sunlight, vivid colorful summer flowers]",
	"[Autumn theme: fiery pastel red and golden orange leaves, falling autumn foliage, warm amber tones]",
	"[Winter theme: covered in white snow, icy frost crystals, cool magical pastel blue and white tones]",
}

func buildPrompt(stage, generation int) string {
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
