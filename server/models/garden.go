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

// ユーザーレベル閾値（累計EXPで判定）
// 指数関数的に必要経験値が増加していくよう設計
//   Lv.1:    0 EXP〜
//   Lv.2:  200 EXP〜  (Stage 1→2 で1回分)
//   Lv.3:  500 EXP〜  (Stage 2→3 で1回分追加)
//   Lv.4:  900 EXP〜  (世代交代1回分追加)
//   Lv.5: 1500 EXP〜
//   Lv.6: 2300 EXP〜
//   Lv.7: 3400 EXP〜
//   Lv.8: 5000 EXP〜  (いわゆる「一人前」の目安)
//   Lv.9: 7200 EXP〜
//   Lv.10: 10000 EXP〜 (マックス)
var UserLevelThresholds = []int{
	0,     // Lv.1
	200,   // Lv.2
	500,   // Lv.3
	900,   // Lv.4
	1500,  // Lv.5
	2300,  // Lv.6
	3400,  // Lv.7
	5000,  // Lv.8
	7200,  // Lv.9
	10000, // Lv.10
}

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
	StageThreshold2 = 400  // Stage 2 開始ポイント
	StageThreshold3 = 800  // Stage 3 開始ポイント
	MaxPoints       = 1000 // 世代交代ポイント
)

// EXP加算量（段階アップ時）
// Stage 2到達: +200 EXP（Lv.1→2 に必要な量と同じ）
// Stage 3到達: +300 EXP（さらに進化への達成感を強調）
var StageExpGain = map[int]int{
	1: 0,   // Stage 1 は初期状態なのでEXP加算なし
	2: 200, // Stage 1 → Stage 2 到達
	3: 300, // Stage 2 → Stage 3 到達
}

// WorldGenEXP は世代交代時のEXP加算量（Stage3到達の後にさらに+400）
const WorldGenEXP = 400

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

