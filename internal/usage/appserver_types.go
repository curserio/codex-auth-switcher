package usage

// Snapshot mirrors the rate-limit payload shape used by Codex app-server.
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

// LimitWindow accepts both snake_case and camelCase fields seen across Codex surfaces.
type LimitWindow struct {
	UsedPercent        int    `json:"used_percent,omitempty"`
	UsedPercentCamel   int    `json:"usedPercent,omitempty"`
	WindowMinutes      *int   `json:"window_minutes,omitempty"`
	ResetsAt           *int64 `json:"resets_at,omitempty"`
	ResetsAtCamel      *int64 `json:"resetsAt,omitempty"`
	WindowDurationMins *int   `json:"windowDurationMins,omitempty"`
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
