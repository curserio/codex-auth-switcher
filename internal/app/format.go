package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/usage"
)

func formatReset(ts *int64) string {
	return formatResetAt(ts, time.Now())
}

func formatResetAt(ts *int64, now time.Time) string {
	if ts == nil {
		return "unknown"
	}
	reset := time.Unix(*ts, 0)
	return reset.Format("02 Jan 15:04") + " (" + formatDurationUntil(reset, now) + ")"
}

func formatDurationUntil(target, now time.Time) string {
	remaining := target.Sub(now).Round(time.Minute)
	if remaining <= 0 {
		return "now"
	}
	minutes := int(remaining.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%dm left", minutes)
	}
	hours := minutes / 60
	minutes = minutes % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%dm left", hours, minutes)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd%dh left", days, hours)
}

func staleLabel(record usage.Record) string {
	age := time.Since(record.CapturedAt).Round(time.Minute)
	parts := []string{record.Source, "seen " + age.String() + " ago"}
	if record.Stale {
		parts = append(parts, "stale")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
