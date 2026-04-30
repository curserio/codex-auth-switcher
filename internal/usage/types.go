package usage

import "time"

type Record struct {
	CapturedAt time.Time `json:"captured_at"`
	Source     string    `json:"source"`
	PlanType   string    `json:"plan_type,omitempty"`
	FiveHour   Window    `json:"five_hour"`
	Weekly     Window    `json:"weekly"`
	Stale      bool      `json:"stale,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type Window struct {
	UsedPercent int    `json:"used_percent"`
	LeftPercent int    `json:"left_percent"`
	ResetsAt    *int64 `json:"resets_at,omitempty"`
}
