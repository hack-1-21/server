package models

import "time"

// Garden は箱庭の1世代を表すモデル
type Garden struct {
	ID          int        `json:"id"`
	UserID      string     `json:"user_id"`
	Generation  int        `json:"generation"`
	Points      int        `json:"points"`
	Stage       int        `json:"stage"`
	ImageURL    string     `json:"image_url"`
	IsActive    bool       `json:"is_active"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// MeasurementResponse は POST /measurements のレスポンス
type MeasurementResponse struct {
	ID         int64  `json:"id"`
	Message    string `json:"message"`
	// 箱庭情報
	Points     int    `json:"points"`
	Stage      int    `json:"stage"`
	ImageURL   string `json:"image_url"`
	Generation int    `json:"generation"`
	StageUp    bool   `json:"stage_up"`
	// 世代交代
	GenerationUp bool `json:"generation_up"`
	// ユーザー情報
	Level    int `json:"level"`
	TotalExp int `json:"total_exp"`
}

// ProfileResponse は GET /users/{user_id}/profile のレスポンス
type ProfileResponse struct {
	User   User    `json:"user"`
	Garden *Garden `json:"garden"`
}

// ユーザーレベル閾値（累計EXP）
var UserLevelThresholds = []int{0, 100, 300, 600, 1000}

// CalcUserLevel は累計EXPからユーザーレベルを計算する
func CalcUserLevel(totalExp int) int {
	level := 1
	for i, threshold := range UserLevelThresholds {
		if totalExp >= threshold {
			level = i + 1
		}
	}
	return level
}

// 箱庭段階閾値
const (
	StageThreshold2 = 400  // Lv.2 開始ポイント
	StageThreshold3 = 800  // Lv.3 開始ポイント
	MaxPoints       = 1000 // 世代交代ポイント
)

// EXP加算量（段階アップ時）
var StageExpGain = map[int]int{
	1: 10,  // Lv.1 到達（初期）
	2: 30,  // Lv.2 到達
	3: 60,  // Lv.3 到達
}

// WorldGenEXP は世代交代時のEXP加算量
const WorldGenEXP = 100

// CalcStage はポイントから箱庭の段階を返す
func CalcStage(points int) int {
	switch {
	case points >= StageThreshold3:
		return 3
	case points >= StageThreshold2:
		return 2
	default:
		return 1
	}
}
