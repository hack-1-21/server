package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hack1-server/db"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type routePoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type QuietRouteRequest struct {
	Origin      routePoint `json:"origin"`
	Destination routePoint `json:"destination"`
	Mode        string     `json:"mode"`
}

type QuietRouteResponse struct {
	Routes []QuietRouteCandidate `json:"routes"`
}

type QuietRouteCandidate struct {
	Rank        int          `json:"rank"`
	Label       string       `json:"label"`
	DistanceM   int          `json:"distance_m"`
	DurationSec int          `json:"duration_sec"`
	AvgDB       float64      `json:"avg_db"`
	LoudSpots   int          `json:"loud_spots"`
	QuietScore  int          `json:"quiet_score"`
	Cost        float64      `json:"cost"`
	Polyline    string       `json:"polyline"`
	Points      []routePoint `json:"points"`
}

type googleRoutesRequest struct {
	Origin                   googleWaypoint `json:"origin"`
	Destination              googleWaypoint `json:"destination"`
	TravelMode               string         `json:"travelMode"`
	ComputeAlternativeRoutes bool           `json:"computeAlternativeRoutes"`
	PolylineEncoding         string         `json:"polylineEncoding"`
	RoutingPreference        string         `json:"routingPreference,omitempty"`
}

type googleWaypoint struct {
	Location googleLocation `json:"location"`
}

type googleLocation struct {
	LatLng googleLatLng `json:"latLng"`
}

type googleLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type googleRoutesResponse struct {
	Routes []googleRoute `json:"routes"`
	Error  *googleError  `json:"error,omitempty"`
}

type googleError struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

type googleRoute struct {
	DistanceMeters int            `json:"distanceMeters"`
	Duration       string         `json:"duration"`
	Polyline       googlePolyline `json:"polyline"`
}

type googlePolyline struct {
	EncodedPolyline string `json:"encodedPolyline"`
}

type noiseMeasurement struct {
	Lat float64
	Lng float64
	DB  float64
}

// GetQuietRoutes POST /routes/quiet
// Google Routes API の徒歩ルート候補を、DB上の音量データで静音ランキングする
func GetQuietRoutes(w http.ResponseWriter, r *http.Request) {
	var req QuietRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト形式が不正です")
		return
	}
	if !isValidRoutePoint(req.Origin) || !isValidRoutePoint(req.Destination) {
		respondError(w, http.StatusBadRequest, "origin / destination の緯度経度が不正です")
		return
	}
	if req.Mode == "" {
		req.Mode = "quiet"
	}
	if req.Mode != "quiet" && req.Mode != "balanced" && req.Mode != "fast" {
		respondError(w, http.StatusBadRequest, "mode は quiet / balanced / fast のいずれかを指定してください")
		return
	}

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		respondError(w, http.StatusInternalServerError, "GOOGLE_MAPS_API_KEY が環境変数に設定されていません")
		return
	}

	googleRoutes, err := fetchGoogleWalkingRoutes(req, apiKey)
	if err != nil {
		log.Printf("Google Routes API エラー: %v", err)
		respondError(w, http.StatusBadGateway, "Google Routes API からルート取得に失敗しました")
		return
	}
	if len(googleRoutes) == 0 {
		respondError(w, http.StatusNotFound, "候補ルートが見つかりませんでした")
		return
	}

	candidates := make([]QuietRouteCandidate, 0, len(googleRoutes))
	for _, gr := range googleRoutes {
		if gr.Polyline.EncodedPolyline == "" {
			continue
		}
		points, err := decodePolyline(gr.Polyline.EncodedPolyline)
		if err != nil || len(points) == 0 {
			log.Printf("polyline decode失敗: %v", err)
			continue
		}

		measurements, err := fetchNoiseMeasurementsForRoute(points)
		if err != nil {
			log.Printf("measurements SELECT失敗: %v", err)
			respondError(w, http.StatusInternalServerError, "ルート周辺の音量データ取得に失敗しました")
			return
		}

		avgDB, loudSpots := estimateRouteNoise(points, measurements)
		quietScore := calcQuietScore(avgDB, loudSpots)
		durationSec := parseGoogleDurationSec(gr.Duration)
		cost := calcRouteCost(req.Mode, gr.DistanceMeters, durationSec, avgDB, loudSpots)

		candidates = append(candidates, QuietRouteCandidate{
			Label:       routeLabel(req.Mode),
			DistanceM:   gr.DistanceMeters,
			DurationSec: durationSec,
			AvgDB:       math.Round(avgDB*10) / 10,
			LoudSpots:   loudSpots,
			QuietScore:  quietScore,
			Cost:        math.Round(cost*10) / 10,
			Polyline:    gr.Polyline.EncodedPolyline,
			Points:      points,
		})
	}
	if len(candidates) == 0 {
		respondError(w, http.StatusInternalServerError, "評価可能な候補ルートがありませんでした")
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Cost < candidates[j].Cost
	})
	for i := range candidates {
		candidates[i].Rank = i + 1
		if i == 0 {
			candidates[i].Label = routeLabel(req.Mode)
		} else {
			candidates[i].Label = "候補" + strconv.Itoa(i+1)
		}
	}

	respondJSON(w, http.StatusOK, QuietRouteResponse{Routes: candidates})
}

func fetchGoogleWalkingRoutes(req QuietRouteRequest, apiKey string) ([]googleRoute, error) {
	body := googleRoutesRequest{
		Origin: googleWaypoint{Location: googleLocation{LatLng: googleLatLng{
			Latitude:  req.Origin.Lat,
			Longitude: req.Origin.Lng,
		}}},
		Destination: googleWaypoint{Location: googleLocation{LatLng: googleLatLng{
			Latitude:  req.Destination.Lat,
			Longitude: req.Destination.Lng,
		}}},
		TravelMode:               "WALK",
		ComputeAlternativeRoutes: true,
		PolylineEncoding:         "ENCODED_POLYLINE",
	}

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(
		"POST",
		"https://routes.googleapis.com/directions/v2:computeRoutes",
		bytes.NewBuffer(reqBytes),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", apiKey)
	httpReq.Header.Set("X-Goog-FieldMask", "routes.distanceMeters,routes.duration,routes.polyline.encodedPolyline")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var googleResp googleRoutesResponse
	if err := json.NewDecoder(resp.Body).Decode(&googleResp); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if googleResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", googleResp.Error.Status, googleResp.Error.Message)
		}
		return nil, fmt.Errorf("status=%d", resp.StatusCode)
	}
	return googleResp.Routes, nil
}

func fetchNoiseMeasurementsForRoute(points []routePoint) ([]noiseMeasurement, error) {
	minLat, maxLat, minLng, maxLng := routeBounds(points)
	const pad = 0.003

	rows, err := db.DB.Query(
		`SELECT latitude, longitude, db
		 FROM measurements
		 WHERE latitude >= $1 AND latitude <= $2
		   AND longitude >= $3 AND longitude <= $4`,
		minLat-pad,
		maxLat+pad,
		minLng-pad,
		maxLng+pad,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	measurements := []noiseMeasurement{}
	for rows.Next() {
		var m noiseMeasurement
		if err := rows.Scan(&m.Lat, &m.Lng, &m.DB); err != nil {
			return nil, err
		}
		measurements = append(measurements, m)
	}
	return measurements, rows.Err()
}

func estimateRouteNoise(points []routePoint, measurements []noiseMeasurement) (float64, int) {
	if len(points) == 0 {
		return 60, 0
	}
	if len(measurements) == 0 {
		return 60, 0
	}

	samples := sampleRoutePoints(points, 35)
	total := 0.0
	covered := 0
	loudSpots := 0

	for _, p := range samples {
		sum := 0.0
		count := 0
		for _, m := range measurements {
			dist := haversineMeters(p.Lat, p.Lng, m.Lat, m.Lng)
			if dist <= 80 {
				sum += m.DB
				count++
			}
		}
		if count == 0 {
			total += 60
			continue
		}
		avg := sum / float64(count)
		total += avg
		covered++
		if avg >= 75 {
			loudSpots++
		}
	}

	avgDB := total / float64(len(samples))
	if covered < len(samples)/4 {
		// データが薄いルートは過信しないように、都市部の中間値へ少し寄せる
		avgDB = avgDB*0.7 + 60*0.3
	}
	return avgDB, loudSpots
}

func sampleRoutePoints(points []routePoint, maxSamples int) []routePoint {
	if len(points) <= maxSamples {
		return points
	}
	samples := make([]routePoint, 0, maxSamples)
	step := float64(len(points)-1) / float64(maxSamples-1)
	for i := 0; i < maxSamples; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx >= len(points) {
			idx = len(points) - 1
		}
		samples = append(samples, points[idx])
	}
	return samples
}

func calcQuietScore(avgDB float64, loudSpots int) int {
	score := 100 - int(math.Round((avgDB-40)*1.15)) - loudSpots*2
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func calcRouteCost(mode string, distanceM, durationSec int, avgDB float64, loudSpots int) float64 {
	noisePenalty := math.Max(avgDB-50, 0)
	switch mode {
	case "fast":
		return float64(durationSec)*1.0 + float64(distanceM)*0.05 + noisePenalty*12 + float64(loudSpots)*20
	case "balanced":
		return float64(durationSec)*0.85 + float64(distanceM)*0.08 + noisePenalty*35 + float64(loudSpots)*60
	default:
		return float64(durationSec)*0.6 + float64(distanceM)*0.05 + noisePenalty*70 + float64(loudSpots)*120
	}
}

func parseGoogleDurationSec(duration string) int {
	duration = strings.TrimSuffix(duration, "s")
	sec, err := strconv.Atoi(duration)
	if err != nil {
		return 0
	}
	return sec
}

func routeLabel(mode string) string {
	switch mode {
	case "fast":
		return "最短優先"
	case "balanced":
		return "バランス"
	default:
		return "静音優先"
	}
}

func isValidRoutePoint(p routePoint) bool {
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180 && !(p.Lat == 0 && p.Lng == 0)
}

func routeBounds(points []routePoint) (minLat, maxLat, minLng, maxLng float64) {
	minLat, maxLat = points[0].Lat, points[0].Lat
	minLng, maxLng = points[0].Lng, points[0].Lng
	for _, p := range points[1:] {
		minLat = math.Min(minLat, p.Lat)
		maxLat = math.Max(maxLat, p.Lat)
		minLng = math.Min(minLng, p.Lng)
		maxLng = math.Max(maxLng, p.Lng)
	}
	return
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

func decodePolyline(encoded string) ([]routePoint, error) {
	points := []routePoint{}
	index := 0
	lat, lng := 0, 0

	for index < len(encoded) {
		dLat, nextIndex, err := decodePolylineValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = nextIndex

		dLng, nextIndex, err := decodePolylineValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = nextIndex

		lat += dLat
		lng += dLng
		points = append(points, routePoint{
			Lat: float64(lat) / 1e5,
			Lng: float64(lng) / 1e5,
		})
	}

	return points, nil
}

func decodePolylineValue(encoded string, index int) (int, int, error) {
	result := 0
	shift := 0

	for {
		if index >= len(encoded) {
			return 0, index, fmt.Errorf("invalid encoded polyline")
		}
		b := int(encoded[index]) - 63
		index++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}

	if result&1 != 0 {
		return ^(result >> 1), index, nil
	}
	return result >> 1, index, nil
}
