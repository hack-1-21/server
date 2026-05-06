package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

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

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("No API Key")
		return
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/imagen-3.0-generate-002:predict?key=%s", apiKey)
	
	reqBody := imagenRequest{
		Instances: []imagenInstance{{Prompt: "a tiny rabbit in a garden"}},
		Parameters: imagenParameters{
			SampleCount: 1,
			AspectRatio: "1:1",
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
}
