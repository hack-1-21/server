package handlers

import (
	"bytes"
	"encoding/base64"
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
// プロンプト生成
// ===========================

var seasons = []string{"spring", "summer", "autumn", "winter"}
var weathers = []string{"sunny", "misty", "rainy", "starry night", "golden hour"}
var animals1 = []string{"a tiny rabbit", "a small deer fawn", "a hedgehog", "a baby fox"}
var animals2 = []string{"rabbits and deer", "foxes and owls", "deer and hedgehogs", "fireflies and frogs"}
var animals3 = []string{"foxes, deer, rabbits, owls, and fireflies", "wolves, deer, bears, and fairies", "unicorns, rabbits, foxes, and butterflies"}

func buildPrompt(stage, generation int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	season := seasons[r.Intn(len(seasons))]
	weather := weathers[r.Intn(len(weathers))]

	base := fmt.Sprintf(
		"A magical miniature world inside a glass bottle, fantasy art style, %s %s, ",
		season, weather,
	)

	switch stage {
	case 1:
		animal := animals1[r.Intn(len(animals1))]
		return base + fmt.Sprintf(
			"a tiny sprouting seedling just emerging from the soil, %s exploring nearby, small wildflowers, soft fog, quiet and peaceful atmosphere, generation %d, photorealistic digital art",
			animal, generation,
		)
	case 2:
		animal := animals2[r.Intn(len(animals2))]
		return base + fmt.Sprintf(
			"a growing tree with lush green leaves, %s living in harmony, a rainbow arching over the scene, warm sunlight streaming through, vibrant and lively, generation %d, photorealistic digital art",
			animal, generation,
		)
	case 3:
		animal := animals3[r.Intn(len(animals3))]
		return base + fmt.Sprintf(
			"a magnificent ancient tree with glowing roots, %s in a thriving ecosystem, glowing fairies dancing, a brilliant shining rainbow, bursting with life and magic, generation %d, photorealistic digital art",
			animal, generation,
		)
	default:
		return base + "a magical garden in full bloom"
	}
}

// ===========================
// Gemini Imagen API 呼び出し
// ===========================

type imagenRequest struct {
	Instances  []imagenInstance  `json:"instances"`
	Parameters imagenParameters  `json:"parameters"`
}

type imagenInstance struct {
	Prompt string `json:"prompt"`
}

type imagenParameters struct {
	SampleCount int    `json:"sampleCount"`
	AspectRatio string `json:"aspectRatio"`
}

type imagenResponse struct {
	Predictions []struct {
		BytesBase64Encoded string `json:"bytesBase64Encoded"`
		MimeType           string `json:"mimeType"`
	} `json:"predictions"`
}

func generateGardenImage(prompt string) ([]byte, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY が設定されていません")
	}

	// Imagen 4 モデルを使用
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/imagen-4.0-generate-001:predict?key=%s",
		apiKey,
	)

	reqBody := imagenRequest{
		Instances: []imagenInstance{{Prompt: prompt}},
		Parameters: imagenParameters{
			SampleCount: 1,
			AspectRatio: "1:1",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("リクエスト生成失敗: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("Gemini API 呼び出し失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API エラー %d: %s", resp.StatusCode, string(body))
	}

	var imagenResp imagenResponse
	if err := json.NewDecoder(resp.Body).Decode(&imagenResp); err != nil {
		return nil, fmt.Errorf("レスポンス解析失敗: %w", err)
	}

	if len(imagenResp.Predictions) == 0 {
		return nil, fmt.Errorf("画像が生成されませんでした")
	}

	imgData, err := base64.StdEncoding.DecodeString(imagenResp.Predictions[0].BytesBase64Encoded)
	if err != nil {
		return nil, fmt.Errorf("Base64 デコード失敗: %w", err)
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
