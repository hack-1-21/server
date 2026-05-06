package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env") // 実際の .env がある場所に合わせて調整
	
	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")

	endpoint := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/ai/run/@cf/stabilityai/stable-diffusion-xl-base-1.0",
		accountID,
	)

	// dummy 1x1 pixel PNG
	pngBytes := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 215, 99, 248, 255, 255, 63, 0, 5, 254, 2, 254, 220, 204, 89, 231, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}

	// 1. try int array
	imgInts := make([]int, len(pngBytes))
	for i, b := range pngBytes {
		imgInts[i] = int(b)
	}

	reqData := map[string]interface{}{
		"prompt": "a dog",
		"width":  1024,
		"height": 1024,
		"image":  imgInts,
		"strength": 0.65,
	}

	reqBodyBytes, _ := json.Marshal(reqData)

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Int array result -> Status: %d\nBody: %s\n\n", resp.StatusCode, string(body))

	// 2. try base64 string
	reqDataB64 := map[string]interface{}{
		"prompt": "a dog",
		"width":  1024,
		"height": 1024,
		"image":  base64.StdEncoding.EncodeToString(pngBytes),
		"strength": 0.65,
	}
	reqBodyBytes, _ = json.Marshal(reqDataB64)
	req, _ = http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(resp.Body)
		fmt.Printf("Base64 string 'image' result -> Status: %d\nBody: %s\n\n", resp.StatusCode, string(body))
	}
}
