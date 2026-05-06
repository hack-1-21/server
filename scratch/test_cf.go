package main

import (
	"bytes"
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

	reqData := map[string]interface{}{
		"prompt": "a dog",
		"width":  1024,
		"height": 1024,
		"image":  []int{0, 1, 2}, // dummy
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
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
}
