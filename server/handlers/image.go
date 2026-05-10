package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Gemini API (generateContent) 用のリクエスト構造体
type GeminiRequest struct {
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inlineData,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // Base64文字列
}

type GenerationConfig struct {
	ResponseModalities []string `json:"responseModalities"`
}

// レスポンス構造体
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData *InlineData `json:"inlineData,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ==========================================================================
// 1. 各属性ごとの進化プロンプト設定 (★英語プロンプトを直接コピペ・カスタマイズできるエリア★)
// ==========================================================================

// 【共通スタイル / BASE STYLE】
// 画角、容器、全体テイスト（アンティーク魔法瓶、リヴリーアイランド/ポケモンSleep/Duolingo風のシンプルでフラットな質感、英語文字無し）を規定します。
var BasePrompt = "straight-on front view, perfect centered composition, a single symmetrical glass potion bottle in the exact center, the entire bottle is fully visible, shaped like an otherworldly antique magical perfume bottle with elegant decorations, flat simple illustration, cozy game art style similar to Livly Island, Duolingo, and Pokemon Sleep, no text, no words, no letters"
// --- 🌱 【草属性 / GRASS】 (第1世代, 第5世代, ...) ---
// 朝露に濡れた草花、絡んだつる、美しい木、草原、可憐な動物、背景は森奥深くのうっそうとした神殿、中央に成長する木
var GrassStage1 = "[Grass Theme, Stage 1 - Tiny Seedling] Front view of a transparent antique glass bottle containing a very simple, tiny young green sprout in the exact center. The interior is mostly empty, spacious, with simple soft soil, sparse green grass, and a single tiny forest mouse. Cozy game art, flat simple illustration, no complex decorations, clean textures, no text."
var GrassStage2 = "[Grass Theme, Stage 2 - Thriving Tree and Flowers] Based on the composition of Stage 1, inside the identical glass bottle, the tiny sprout has grown into a beautiful medium-sized thriving tree with lush green leaves. The ground is now covered with colorful blooming flowers, a small gentle rainbow arches in the background, a field mouse and a cute brown rabbit play under the tree. Richer details, clean cozy textures, no text."
var GrassStage3 = "[Grass Theme, Stage 3 - Colossal Forest Majesty] Based on the composition of Stage 2, inside the identical glass bottle, the tree has evolved into an overwhelmingly giant monumental tree with a massive canopy that completely fills the upper bottle. Rampant thick vines scale the inner glass walls, a thick tapestry of hundreds of glittering morning-dewed wildflowers blankets the floor. A majestic, vivid, glowing multi-layered rainbow spans the scene. A mother deer, rabbit, and owls thrive under the branches. Thousands of glittering magical fairy dust and golden sparkles fill the bottle, ultra-luxurious, breathtakingly beautiful, cozy game art, no text."

// --- 🔥 【炎属性 / FIRE】 (第2世代, 第6世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var FireStage1 = "[Fire Theme, Stage 1 - Tiny Ember Sapling] Front view of a transparent antique glass bottle containing a very simple, tiny red-hot glowing sprout in the exact center. The ground is a plain black obsidian rock, with a tiny warm lava puddle and a single small phoenix chick. Cozy game art, flat simple illustration, minimalist setup, no text."
var FireStage2 = "[Fire Theme, Stage 2 - Thriving Fire Tree] Based on the composition of Stage 1, inside the identical glass bottle, the sprout has grown into a medium-sized fire tree with glowing red branches and glowing red berries. A small flowing lava stream winds through obsidian rocks, an active volcano with a soft smoke plume is visible in the far background, a fully-grown phoenix flies around the tree, a young dragon rests on the ground. Richer details, cozy game art, no text."
var FireStage3 = "[Fire Theme, Stage 3 - Colossal Volcanic Majesty] Based on the composition of Stage 2, inside the identical glass bottle, the fire tree has evolved into an overwhelmingly giant volcanic fire tree loaded with thousands of glowing fire fruits. Massive glowing lava lakes cover the floor, a colossal active volcano erupts dramatically in the background with majestic lava flows. A giant soaring phoenix and a huge fierce dragon coiled around the tree trunk breathing bright flame jets dominate the scene. Intense glittering fire sparks and magical golden embers fill the entire bottle, ultra-luxurious, cozy game art, no text."

// --- 💧 【水属性 / WATER】 (第3世代, 第7世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var WaterStage1 = "[Water Theme, Stage 1 - Tiny Water Sprout] Front view of a transparent antique glass bottle containing a very simple, tiny water sprout with translucent blue leaves in the exact center. The ground is simple soft soil with a quiet tiny clear water puddle, a single cute water spirit (Undine) resting, and simple small crystal stones. Cozy game art, flat simple illustration, minimalist setup, no text."
var WaterStage2 = "[Water Theme, Stage 2 - Thriving Water Tree] Based on the composition of Stage 1, inside the identical glass bottle, the sprout has grown into a beautiful weeping water tree with glowing blue branches. A beautiful flowing river with a small waterfall runs through smooth stones, a beautiful mermaid sits by the riverbank, sparkling fish jump from the water, ancient water temple silhouette in the background. Richer details, cozy game art, no text."
var WaterStage3 = "[Water Theme, Stage 3 - Colossal Aquatic Majesty] Based on the composition of Stage 2, inside the identical glass bottle, the tree has evolved into an overwhelmingly giant ancient water tree with cascading weeping branches like a massive glowing waterfall. A grand sweeping river and a huge dramatic waterfall fill the landscape. Gorgeous natural Western-style water fountains and intricate water sculptures decorate the area. Multiple beautiful mermaids swim, surrounded by schools of intensely sparkling fish and glowing water butterflies. A glorious water temple shines in the background. Thousands of glowing water droplets and sparkling lights fill the bottle, ultra-luxurious, cozy game art, no text."

// --- ✨ 【光属性 / LIGHT】 (第4世代, 第8世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var LightStage1 = "[Light Theme, Stage 1 - Tiny Light Sprout] Front view of a transparent antique glass bottle containing a very simple, tiny golden light sprout in the exact center, emitting a soft warm glow. The ground is plain soft white energy grass, with a few floating light orbs and a single cute small angel resting. Cozy game art, flat simple illustration, minimalist setup, no text."
var LightStage2 = "[Light Theme, Stage 2 - Thriving Tree of Life] Based on the composition of Stage 1, inside the identical glass bottle, the sprout has grown into a beautiful medium-sized Tree of Life with glowing gold leaves. Many brilliant light orbs float around, a beautiful celestial maiden (Tennyo) with a floating transparent robe stands gracefully, a white unicorn rests on the white grass, floating temple in the far background. Richer details, cozy game art, no text."
var LightStage3 = "[Light Theme, Stage 3 - Colossal Holy Majesty] Based on the composition of Stage 2, inside the identical glass bottle, the tree has evolved into an overwhelmingly giant World Tree radiating blinding gold light. Thousands of intense floating golden orbs illuminate the scene. A supreme celestial maiden with an enormous, incredibly intricate transparent lace robe gracefully envelops the scene. Multiple flying angels with brilliant halos flutter, a magnificent pegasus soars. A flawless, pure white floating temple shines with a brilliant holy protective barrier. Thousands of sparkling holy stars and golden light rays fill the bottle, ultra-luxurious, cozy game art, no text."

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

// ==========================================
// Google Gemini API 呼び出し (ハイブリッド i2i 対応)
// ==========================================

// 引数に userID と stage を用いて、Stage 1ではテキストプロンプトのみ、
// Stage 2以上では既存画像も入力に含めたマルチモーダル（Image-to-Image）生成を実行します。
func generateGardenImage(prompt string, userID string, stage int) ([]byte, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY が設定されていません")
	}

	// 無料枠（1日500リクエスト）が適用される gemini-2.5-flash-image エンドポイント
	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent?key=%s",
		apiKey,
	)

	parts := []Part{
		{Text: prompt},
	}

	isImg2Img := false

	// Stage 2以上: ローカルから既存画像を読み込み、Image-to-Image の入力として追加
	if stage > 1 {
		dataDir := os.Getenv("STORAGE_DIR")
		if dataDir == "" {
			dataDir = "./data/images"
		}
		activeFilePath := filepath.Join(dataDir, "gardens", userID, fmt.Sprintf("%s.png", userID))

		imgBytes, err := os.ReadFile(activeFilePath)
		if err == nil {
			imgBase64 := base64.StdEncoding.EncodeToString(imgBytes)
			parts = append(parts, Part{
				InlineData: &InlineData{
					MimeType: "image/png",
					Data:     imgBase64,
				},
			})
			isImg2Img = true
			log.Printf("[Gemini i2i] 既存画像をベースに生成します")
		} else {
			log.Printf("[Gemini txt2img] 既存画像が見つからないため新規生成します: %v", err)
		}
	}

	if !isImg2Img {
		log.Printf("[Gemini txt2img] 新規画像を生成します")
	}

	reqData := GeminiRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: parts,
			},
		},
		GenerationConfig: GenerationConfig{
			// 画像出力を明示的に要求
			ResponseModalities: []string{"IMAGE"},
		},
	}

	reqBodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("JSONエンコード失敗: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("リクエスト生成失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 画像生成のレイテンシを考慮しタイムアウトを長めに設定
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API通信エラー: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("レートリミット超過 (429): しばらく待って再試行してください")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("APIエラー %d: %s", resp.StatusCode, string(body))
	}

	var apiResp GeminiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("JSONパース失敗: %w, body: %s", err, string(body))
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API内部エラー: %s", apiResp.Error.Message)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 || apiResp.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, fmt.Errorf("画像データが返却されませんでした")
	}

	// 取得したBase64データをバイナリにデコード
	imgData, err := base64.StdEncoding.DecodeString(apiResp.Candidates[0].Content.Parts[0].InlineData.Data)
	if err != nil {
		return nil, fmt.Errorf("Base64デコード失敗: %w", err)
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
			log.Printf("[画像生成] Gemini API エラー: %v", err)
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

// isImageGenerationConfigured は Gemini API の設定が揃っているか確認する
func isImageGenerationConfigured() bool {
	return os.Getenv("GEMINI_API_KEY") != ""
}
