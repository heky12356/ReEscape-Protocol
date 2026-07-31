package affection

import "time"

type State struct {
	UserID      int64          `json:"user_id"`
	Score       int            `json:"score"`
	Stage       string         `json:"stage"`
	LastReason  string         `json:"last_reason,omitempty"`
	LastUpdated time.Time      `json:"last_updated"`
	DailyDelta  map[string]int `json:"daily_delta,omitempty"`
	Changes     []ChangeRecord `json:"changes,omitempty"`
}

type ChangeRecord struct {
	RequestedDelta int       `json:"requested_delta"`
	AppliedDelta   int       `json:"applied_delta"`
	Reason         string    `json:"reason"`
	Confidence     float64   `json:"confidence"`
	Tags           []string  `json:"tags,omitempty"`
	Policy         string    `json:"policy,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type UpdateRequest struct {
	Delta      int      `json:"delta"`
	Reason     string   `json:"reason"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
}

type UpdateResult struct {
	State          State  `json:"state"`
	RequestedDelta int    `json:"requested_delta"`
	AppliedDelta   int    `json:"applied_delta"`
	Policy         string `json:"policy"`
}
