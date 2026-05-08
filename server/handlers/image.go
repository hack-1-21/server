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

// ==========================================================================
// 1. 各属性ごとの進化プロンプト設定 (★英語プロンプトを直接コピペ・カスタマイズできるエリア★)
// ==========================================================================

// 【共通スタイル / BASE STYLE】
// 画角、容器、全体テイスト（アンティーク魔法瓶、リヴリーアイランド/ポケモンSleep/Duolingo風のシンプルでフラットな質感、英語文字無し）を規定します。
var BasePrompt = "straight-on front view, perfect centered composition, a single symmetrical glass potion bottle in the exact center, the entire bottle is fully visible, shaped like an otherworldly antique magical perfume bottle with elegant decorations, flat simple illustration, cozy game art style similar to Livly Island, Duolingo, and Pokemon Sleep, no text, no words, no letters"

// --- 🌱 【草属性 / GRASS】 (第1世代, 第5世代, ...) ---
// 朝露に濡れた草花、絡んだつる、美しい木、草原、可憐な動物、背景は森奥深くのうっそうとした神殿、中央に成長する木
var GrassStage1 = "[Grass Theme, Stage 1 - Seedling]Please generate an image of a miniature landscape within a transparent container, presented from a front view and ensuring the entire scene is clearly visible from edge to edge within a square frame. The container itself is designed to look like a luxurious, antique, and otherworldly glass object, similar to a beautifully detailed perfume bottle or an ancient glass potion vessel from overseas, complete with complex, magical engravings and golden filigree. The interior contains a delicate, early-stage miniature garden focusing on a single, young sapling planted directly in the center. This tiny tree is just beginning to develop its first true branches and a few small leaves. The ground surrounding it is a developing meadow, covered in a sparse but healthy layer of morning-dewed grass and a few early-blooming wildflowers, all sparkling in soft, magical light. Entwined vines are just starting to trail along the floor of the container. The background within the container depicts a deeply recessed, overgrown forest floor leading towards a very ancient, hidden temple structure deep within, seemingly untouched by humans for many years. There are absolutely no man-made objects within the miniature landscape itself; the overall feeling is flat, simple, fantasy-like, and sparkling. Small, delicate creatures, such as a tiny forest mouse and a few small birds, are present in the young grass. The aesthetic is inspired by games like Livly Island, Duolingo, and Pokémon Sleep, with clean textures. No English text is visible."
var GrassStage2 = "[Grass Theme, Stage 2 - Bloom and Growth]Based on the composition and container from Stage 1, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but with significant growth. The central young sapling from image_0.png has now grown into a more substantial, young tree with a sturdy trunk and a spreading canopy of beautiful, lush leaves, becoming a key focal point. The meadow ground cover is much denser and more verdant, filled with an increased abundance of diverse wildflowers—including new species like tiny bluebells and small yellow ranunculus—all heavily covered in glistening morning dew, creating a more sparkling effect. The entwined vines along the floor from image_0.png have grown, wrapping more thickly around the base of the central tree and spreading further. More delicate forest creatures are present; the field mouse is still there, joined by a small brown rabbit and a wider variety of birds like tiny finches and a wren, creating a more bustling forest scene. A clear, delicate rainbow now subtly arches in the distance, appearing within the overgrown forest background leading to the temple, as seen in image_0.png. The antique golden filigree on the container is identical, and the background temple remains, but the forest is denser. The entire scene is still viewed from the front, square composition, with no man-made objects and no English text. The flat, simple aesthetic with Livly Island-style textures is maintained."
var GrassStage3 = "[Grass Theme, Stage 3 - Forest Majesty]Based on the composition and container from Stage 2, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but at its final, overwhelmingly lush stage. The central tree from image_1.png has fully matured into a magnificent, 'giant tree' with an enormous, ancient-looking trunk and a massive, dense canopy of beautiful, lush, dew-laden leaves that nearly fills the entire upper portion of the container. The entire meadow floor is now a thick, rich tapestry of countless wildflowers of all colors and diverse grasses, heavily damp with morning dew and sparkling with countless tiny, magical, glittering motes. The entwined vines from image_1.png are rampant, draped thickly over the giant tree and scaling the container's inner walls, making the entire scene feel like a deep, ancient forest. The background overgrown forest, leading to the fully visible but now heavily integrated background temple, is denser than ever. More diverse delicate forest creatures are present: the mouse and rabbit are joined by a mother deer and fawn, a small family of owls in the tree branches, and various other tiny animals and birds, creating a thriving ecosystem. A brilliant, wide, and intensely vivid multi-layered rainbow dramatically arches over the entire scene, appearing to touch the giant tree's canopy. The antique golden filigree on the container remains identical, but the scene is bursting with life. The overall feeling is fantasy-like, sparkling, and breathtakingly beautiful, maintaining the flat, simple aesthetic and specific game style (Livly Island, Duolingo, Pokémon Sleep textures) with no English text and the square front view composition."

// --- 🔥 【炎属性 / FIRE】 (第2世代, 第6世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var FireStage1 = "[Fire Theme, Stage 1 - Young Fire Sapling]Please generate an image of a miniature landscape with a fire theme, enclosed within a transparent container presented from a front view. Ensure the entire scene is clearly visible from edge to edge within a square frame. The container itself is designed as a luxurious, antique, and otherworldly glass object, resembling a unique perfume bottle or an ancient glass potion vessel from overseas, complete with complex, magical engravings and bronze filigree that looks inspired by cooled lava. The interior contains a delicate, early-stage miniature fire garden. The ground is a sparse plain of rough obsidian stone, with a tiny, developing pond of molten lava in one corner. At the center stands a single, small young fire sapling. This tiny tree is just beginning to develop its first few red-hot branches and buds of fire fruit, only a few small, glowing berries are visible. A small phoenix chick is nestled near the tree’s base, and a very young, non-fire-breathing dragon is exploring the obsidian rocks. In the background, a small, dormant volcano is visible under a reddish, hazy sky. The overall feeling is flat, simple, fantasy-like, and sparkling with soft, internal firelight. There are absolutely no man-made objects within the landscape. The aesthetic is inspired by games like Livly Island, Duolingo, and Pokémon Sleep, with clean textures. No English text is visible."
var FireStage2 = "[Fire Theme, Stage 2 - Growing Fire Tree]Based on the composition and container design of Stage 1, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but with significant growth. The central fire tree from image_0.png has now grown into a more substantial tree with sturdy, glowing branches and a spreading canopy, becoming a clear focal point. It is now heavily laden with clusters of mature, glowing fire fruits, which appear as fiery berries. The obsidian ground is more expansive, with a flowing river of molten lava creating a vibrant path across the scene. Large chunks of obsidian are visible, and more intense lava ponds have formed. The background volcano is now active and has begun a small eruption, under a slightly darker, burning red sky with scattered ash clouds. The phoenix has matured, appearing as a full-grown, majestic bird about to take flight from the tree, and the young dragon has grown, now capable of exhaling small bursts of fire near the lava flow. The entire scene is still viewed from the front, square composition, with no man-made objects and no English text. The flat, simple aesthetic with Livly Island-style textures is maintained, with more pronounced fiery sparkling."
var FireStage3 = "[Fire Theme, Stage 3 - Fire Tree Majesty]Based on the composition and container design of Stage 2, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but at its final, overwhelmingly lush and fierce stage. The central fire tree from image_1.png has fully matured into a magnificent, giant fire tree with an enormous, ancient-looking trunk and a massive, dense canopy of glowing, beautiful leaves and thousands of ripe, sparkling fire fruits, making it look like a massive fire treasure. The entire miniature landscape is a thriving volcanic ecosystem. The ground is dominated by extensive lava lakes and intricate obsidian formations, creating a rich tapestry of orange, red, and black textures. The background volcano is now a huge, actively erupting peak, with dramatic plumes of smoke, ash, and lava flows dominating the sky and surrounding area, making the environment look completely hostile and ancient. Both the phoenix and the dragon are now majestic, large creatures: the fully grown phoenix is soaring over the erupting volcano, and the large, fierce fire-breathing dragon is coiled majestically near the giant tree, breathing a powerful jet of fire towards the volcanic landscape. The container itself feels completely filled with light and life. The flat, simple aesthetic and specific game style (Livly Island, Duolingo, Pokémon Sleep textures) are maintained with no English text and the square front view composition, bursting with intense fiery sparkling and magical energy."

// --- 💧 【水属性 / WATER】 (第3世代, 第7世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var WaterStage1 = "[Water Theme, Stage 1 - Pure Water Sapling]Please generate an image of a miniature landscape with a water theme, enclosed within a transparent container presented from a front view. Ensure the entire scene is clearly visible from edge to edge within a square frame. The container itself is designed as a luxurious, antique, and otherworldly glass object, resembling a unique perfume bottle or an ancient crystal potion vessel from overseas, complete with complex, magical engravings and elegant silver filigree that evokes the flow of water. The interior contains a delicate, early-stage miniature water garden. At the center stands a single, small young sapling of a mystical water tree, its translucent leaves just beginning to bud. A gentle, clear stream flows quietly around its base, forming a small, shallow pool. Tiny fish with brilliantly sparkling scales dart in the clear water, and a young, delicate water spirit (Undine) rests peacefully nearby. Natural crystal formations that subtly resemble a small, elegant sculpture rise from the water. The background reveals the faint, ethereal silhouette of an ancient temple dedicated to water, radiating a soft, divine light. There are no mundane man-made objects; everything feels born of nature and divine magic. The overall feeling is flat, simple, fantasy-like, and sparkling. The aesthetic is inspired by games like Livly Island, Duolingo, and Pokémon Sleep, with clean, appealing textures. No English text is visible."
var WaterStage2 = "[Water Theme, Stage 2 - Flowing River and Mermaids]Based on the composition and container design of Stage 1, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but with significant growth and aquatic development. The central mystical water tree has now grown into a more substantial tree with graceful, weeping branches and lush, glowing translucent leaves, becoming a clear focal point. The gentle stream has expanded into a beautiful, flowing river, culminating in a small, elegant waterfall over smooth rocks. Beautiful magical butterflies with water-like wings flutter around the growing tree. The natural crystal and ice formations have grown larger, clearly resembling beautiful, ancient sculptures shaped by the water itself. A beautiful mermaid with a softly sparkling tail sits gracefully by the riverbank, accompanied by a maturing, radiant Undine. Schools of larger fish with intensely sparkling scales leap gracefully from the water's surface. The background ancient water temple is now much clearer, showcasing beautiful water art and glowing with a stronger, magical divine protection. The scene is viewed from the front in a square composition, with absolutely no mundane man-made objects and no English text. The flat, simple aesthetic with Livly Island-style textures is perfectly maintained, with an increased sense of flowing water and magical, sparkling light."
var WaterStage3 = "[Water Theme, Stage 3 - Divine Waterfall and Temple]Based on the composition and container design of Stage 2, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but at its final, overwhelmingly lush and divine stage. The central water tree has fully matured into a magnificent, giant mystical tree, its expansive canopy cascading like a massive, glowing waterfall of leaves and magical crystal droplets. The river has transformed into a grand, sweeping waterway featuring a large, breathtaking waterfall that crashes dramatically into a deep, sparkling basin. Elaborate, breathtaking structures resembling majestic Western-style fountains and intricate sculptures, seemingly formed naturally by divine water magic rather than human hands, decorate the lush aquatic landscape. Multiple beautiful mermaids and fully radiant Undines swim and play harmoniously in the glowing waters, surrounded by immense swarms of magical butterflies and dazzling schools of fish with blindingly sparkling scales. The background is a glorious, fully realized divine water temple, overflowing with the ultimate art of water and radiating brilliant, god-like grace and overwhelming divine protection. The container feels completely filled with pure, sparkling aquatic magic. The flat, simple aesthetic and specific game style (Livly Island, Duolingo, Pokémon Sleep textures) are maintained flawlessly, with no English text and the square front view composition, resulting in a breathtakingly beautiful, top-tier fantasy scene."

// --- ✨ 【光属性 / LIGHT】 (第4世代, 第8世代, ...) ---
// (コピペエリア: ここにお好きな英語プロンプトを直接上書き貼り付けしてください！)
var LightStage1 = "[Light Theme, Stage 1 - Seedling of the World Tree]Please generate an image of a miniature landscape with a light theme, enclosed within a transparent container presented from a front view. Ensure the entire scene is clearly visible from edge to edge within a square frame. The container itself is designed as a luxurious, antique, and otherworldly glass object, resembling a unique perfume bottle or an ancient crystal potion vessel from overseas, complete with complex, magical engravings and elegant white-gold filigree that radiates a holy aura. The interior contains a delicate, early-stage miniature light garden. At the center stands a single, small young sapling of the World Tree that governs life, emitting a faint, warm, and pure glow. Soft, gently glowing orbs float quietly around its base. A single cute little angel with a tiny, glowing halo rests peacefully near the sapling. The ground is a pure, flawless expanse of soft, glowing white energy and pristine white grass. The background features the faint silhouette of a beautiful, pure white floating temple, completely free of any flaws or scratches, radiating a soft light of divine protection. There are absolutely no mundane man-made objects; everything feels born of pure divine magic. The overall feeling is flat, simple, fantasy-like, and heavily sparkling with holy light. The aesthetic is inspired by games like Livly Island, Duolingo, and Pokémon Sleep, with clean textures. No English text is visible."
var LightStage2 = "[Light Theme, Stage 2 - Growing Tree of Life and Celestial Beings]Based on the composition and container design of Stage 1, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but with significant growth and an increase in holy presence. The central World Tree of life has now grown into a more substantial, elegant tree with strong, glowing branches and luminous white and gold leaves, becoming a clear focal point of pure energy. The number of floating, brilliantly glowing orbs has increased, illuminating the scene brightly. A beautiful celestial maiden (Tennyo) now stands gracefully near the tree, draped in a highly detailed, transparent, lace-like celestial robe (Hagoromo) that floats ethereally in the air. The cute angel with a halo is now actively flying around the tree, joined by a majestic, pure white unicorn resting peacefully on the glowing white grass. The background pure white floating temple is now much clearer and closer, shining with a stronger divine protection that feels overwhelmingly pure and powerful. The entire scene is viewed from the front in a square composition, with absolutely no mundane man-made objects and no English text. The flat, simple aesthetic with Livly Island-style textures is perfectly maintained, with an increased sense of overwhelming divine light, sparkling magic, and serenity."
var LightStage3 = "[Light Theme, Stage 3 - Majestic World Tree and Supreme Divine Protection]Based on the composition and container design of Stage 2, please generate an image of the miniature landscape inside the identical luxurious, glass perfume-style container, but at its final, overwhelmingly lush and divine stage. The central World Tree of life has fully matured into a magnificent, giant, and breathtakingly radiant tree, its massive canopy overflowing with pure, blinding light and eternal life energy, illuminating the entire container. The air is densely filled with countless intensely glowing orbs. The beautiful celestial maiden is now adorned in an expansive, incredibly intricate transparent lace-like celestial robe that gracefully envelops the scene with divine beauty. Multiple cute angels with brilliant halos flutter joyfully through the radiant branches. The pure white unicorn is now joined by a magnificent pegasus soaring majestically near the glowing canopy. The background is completely dominated by the ultimate, flawless, pure white floating temple, radiating an absolute, blinding light of supreme divine protection—a sacred barrier so powerful that any evil would instantly vanish upon approach. The container feels completely filled with pure, sparkling holy magic and boundless life force. The flat, simple aesthetic and specific game style (Livly Island, Duolingo, Pokémon Sleep textures) are maintained flawlessly, with no English text and the square front view composition, resulting in a breathtakingly beautiful, top-tier radiant fantasy scene."

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
// Hugging Face Serverless Inference API 呼び出し (i2i対応)
// ==========================================

// 引数に userID と stage を用いて、前ステージの画像があれば Base64 にエンコードして i2i (Image-to-Image) を実行します
func generateGardenImage(prompt string, userID string, stage int) ([]byte, error) {
	apiToken := os.Getenv("HF_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("HF_API_TOKEN が環境変数に設定されていません。Hugging FaceのAPIトークンを登録してください。")
	}

	endpoint := "https://api-inference.huggingface.co/models/runwayml/stable-diffusion-v1-5"

	var reqData map[string]interface{}
	isImg2Img := false

	// Stage 2 以上なら、現在アクティブな画像を読み込んでベースにする (Hugging Face img2img)
	if stage > 1 {
		dataDir := os.Getenv("STORAGE_DIR")
		if dataDir == "" {
			dataDir = "./data/images"
		}
		activeFilePath := filepath.Join(dataDir, "gardens", userID, fmt.Sprintf("%s.png", userID))

		imgBytes, err := os.ReadFile(activeFilePath)
		if err == nil {
			base64Str := base64.StdEncoding.EncodeToString(imgBytes)
			reqData = map[string]interface{}{
				"inputs": "data:image/png;base64," + base64Str,
				"parameters": map[string]interface{}{
					"prompt":             prompt,
					"strength":           0.65, // 元画像を残しつつ変化させる強度 (0.0〜1.0)
					"guidance_scale":     7.5,
					"num_inference_steps": 50,
				},
			}
			isImg2Img = true
			log.Printf("[HuggingFace i2i] 前の画像（Base64）をベースに生成します (strength: 0.65)")
		} else {
			log.Printf("[HuggingFace i2i] 前の画像が見つからないため、新規(txt2img)で生成します: %v", err)
		}
	}

	if !isImg2Img {
		// txt2img 用の標準ペイロード
		reqData = map[string]interface{}{
			"inputs": prompt,
		}
		log.Printf("[HuggingFace txt2img] 新規画像を生成します")
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

	// Hugging Face のコールドスタート対策として長めのタイムアウトを設定
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Hugging Face API 呼び出し失敗: %w", err)
	}
	defer resp.Body.Close()

	// 503: モデルロード中（コールドスタート）
	if resp.StatusCode == http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Hugging Face] モデルロード中 (503): %s", string(body))
		return nil, fmt.Errorf("Hugging Faceのモデルがロード中です（約1分かかる場合があります）。数秒後に再試行してください: %s", string(body))
	}

	// その他エラー
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Hugging Face API エラー %d: %s", resp.StatusCode, string(body))
	}

	// Hugging Face Inference API は生成されたバイナリ（PNG/JPEG等）をそのまま返します
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
