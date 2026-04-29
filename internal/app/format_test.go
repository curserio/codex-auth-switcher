package app

import (
	"testing"
	"time"
)

func TestFormatDurationUntil(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		target time.Time
		want   string
	}{
		{
			name:   "minutes",
			target: now.Add(42 * time.Minute),
			want:   "42m left",
		},
		{
			name:   "hours and minutes",
			target: now.Add(3*time.Hour + 12*time.Minute),
			want:   "3h12m left",
		},
		{
			name:   "days and hours",
			target: now.Add(6*24*time.Hour + 2*time.Hour),
			want:   "6d2h left",
		},
		{
			name:   "past",
			target: now.Add(-time.Minute),
			want:   "now",
		},
		{
			name:   "rounding noise",
			target: now.Add(29 * time.Second),
			want:   "now",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDurationUntil(tt.target, now); got != tt.want {
				t.Fatalf("formatDurationUntil() = %q, want %q", got, tt.want)
			}
		})
	}
}
