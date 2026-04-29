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

type Snapshot struct {
	LimitID              *string      `json:"limit_id,omitempty"`
	LimitName            *string      `json:"limit_name,omitempty"`
	Primary              *LimitWindow `json:"primary"`
	Secondary            *LimitWindow `json:"secondary"`
	Credits              any          `json:"credits,omitempty"`
	PlanType             *string      `json:"plan_type,omitempty"`
	RateLimitReachedType *string      `json:"rate_limit_reached_type,omitempty"`
}

type camelSnapshot struct {
	LimitID              *string      `json:"limitId,omitempty"`
	LimitName            *string      `json:"limitName,omitempty"`
	Primary              *LimitWindow `json:"primary"`
	Secondary            *LimitWindow `json:"secondary"`
	Credits              any          `json:"credits,omitempty"`
	PlanType             *string      `json:"planType,omitempty"`
	RateLimitReachedType *string      `json:"rateLimitReachedType,omitempty"`
}

type LimitWindow struct {
	UsedPercent        int    `json:"used_percent,omitempty"`
	UsedPercentCamel   int    `json:"usedPercent,omitempty"`
	WindowMinutes      *int   `json:"window_minutes,omitempty"`
	ResetsAt           *int64 `json:"resets_at,omitempty"`
	ResetsAtCamel      *int64 `json:"resetsAt,omitempty"`
	WindowDurationMins *int   `json:"windowDurationMins,omitempty"`
}

func Normalize(snapshot Snapshot, source string, capturedAt time.Time) Record {
	record := Record{CapturedAt: capturedAt, Source: source}
	if snapshot.PlanType != nil {
		record.PlanType = *snapshot.PlanType
	}
	if snapshot.Primary != nil {
		record.FiveHour = normalizeWindow(*snapshot.Primary)
	}
	if snapshot.Secondary != nil {
		record.Weekly = normalizeWindow(*snapshot.Secondary)
	}
	return record
}

func normalizeWindow(window LimitWindow) Window {
	used := window.UsedPercent
	if used == 0 && window.UsedPercentCamel != 0 {
		used = window.UsedPercentCamel
	}
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	resetsAt := window.ResetsAt
	if resetsAt == nil {
		resetsAt = window.ResetsAtCamel
	}
	return Window{
		UsedPercent: used,
		LeftPercent: 100 - used,
		ResetsAt:    resetsAt,
	}
}

func snapshotFromCamel(in camelSnapshot) Snapshot {
	return Snapshot{
		LimitID:              in.LimitID,
		LimitName:            in.LimitName,
		Primary:              in.Primary,
		Secondary:            in.Secondary,
		Credits:              in.Credits,
		PlanType:             in.PlanType,
		RateLimitReachedType: in.RateLimitReachedType,
	}
}
