package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

var placesClient = &http.Client{Timeout: 10 * time.Second}

// GET /places/autocomplete?input=新宿駅
func AutocompletePlaces(w http.ResponseWriter, r *http.Request) {
	input := r.URL.Query().Get("input")
	if len([]rune(input)) < 1 {
		respondError(w, http.StatusBadRequest, "input が必要です")
		return
	}

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		respondError(w, http.StatusInternalServerError, "GOOGLE_MAPS_API_KEY が設定されていません")
		return
	}

	apiURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/place/autocomplete/json?input=%s&language=ja&key=%s",
		url.QueryEscape(input),
		apiKey,
	)

	resp, err := placesClient.Get(apiURL)
	if err != nil {
		respondError(w, http.StatusBadGateway, "Places API への接続に失敗しました")
		return
	}
	defer resp.Body.Close()

	var result struct {
		Status      string `json:"status"`
		Predictions []struct {
			PlaceID     string `json:"place_id"`
			Description string `json:"description"`
		} `json:"predictions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondError(w, http.StatusBadGateway, "Places API レスポンスの解析に失敗しました")
		return
	}
	if result.Status != "OK" && result.Status != "ZERO_RESULTS" {
		respondError(w, http.StatusBadGateway, "Places API エラー: "+result.Status)
		return
	}

	type prediction struct {
		PlaceID     string `json:"place_id"`
		Description string `json:"description"`
	}
	predictions := make([]prediction, 0, len(result.Predictions))
	for _, p := range result.Predictions {
		predictions = append(predictions, prediction{PlaceID: p.PlaceID, Description: p.Description})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"predictions": predictions})
}

// GET /places/details?place_id=ChIJ...
func GetPlaceDetails(w http.ResponseWriter, r *http.Request) {
	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		respondError(w, http.StatusBadRequest, "place_id が必要です")
		return
	}

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		respondError(w, http.StatusInternalServerError, "GOOGLE_MAPS_API_KEY が設定されていません")
		return
	}

	apiURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/place/details/json?place_id=%s&fields=geometry&key=%s",
		url.QueryEscape(placeID),
		apiKey,
	)

	resp, err := placesClient.Get(apiURL)
	if err != nil {
		respondError(w, http.StatusBadGateway, "Places Details API への接続に失敗しました")
		return
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
		Result struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondError(w, http.StatusBadGateway, "Places Details API レスポンスの解析に失敗しました")
		return
	}
	if result.Status != "OK" {
		respondError(w, http.StatusBadGateway, "Places Details API エラー: "+result.Status)
		return
	}

	loc := result.Result.Geometry.Location
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"lat": loc.Lat,
		"lng": loc.Lng,
	})
}
