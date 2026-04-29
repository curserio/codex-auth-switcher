package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureFromJSONLUsesNewestSnapshot(t *testing.T) {
	home := t.TempDir()
	sessionDir := filepath.Join(home, "sessions", "2026", "04", "29")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-04-29T10:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":{"used_percent":10,"window_minutes":300,"resets_at":1000},"secondary":{"used_percent":20,"window_minutes":10080,"resets_at":2000},"plan_type":"plus"}}}
{"timestamp":"2026-04-29T11:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":{"used_percent":30,"window_minutes":300,"resets_at":3000},"secondary":{"used_percent":40,"window_minutes":10080,"resets_at":4000},"plan_type":"plus"}}}
`
	if err := os.WriteFile(filepath.Join(sessionDir, "rollout-test.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	record, err := CaptureFromJSONL(home)
	if err != nil {
		t.Fatalf("CaptureFromJSONL() error = %v", err)
	}
	if record.FiveHour.UsedPercent != 30 || record.FiveHour.LeftPercent != 70 {
		t.Fatalf("FiveHour = %+v", record.FiveHour)
	}
	if record.Weekly.UsedPercent != 40 || record.Weekly.LeftPercent != 60 {
		t.Fatalf("Weekly = %+v", record.Weekly)
	}
	if record.PlanType != "plus" {
		t.Fatalf("PlanType = %q", record.PlanType)
	}
}

func TestNormalizeCamelCaseSnapshot(t *testing.T) {
	reset := int64(1234)
	plan := "plus"
	record := Normalize(snapshotFromCamel(camelSnapshot{
		PlanType: &plan,
		Primary: &LimitWindow{
			UsedPercentCamel: 25,
			ResetsAtCamel:    &reset,
		},
		Secondary: &LimitWindow{
			UsedPercentCamel: 50,
			ResetsAtCamel:    &reset,
		},
	}), "app-server", mustTime(t, "2026-04-29T10:00:00Z"))

	if record.FiveHour.LeftPercent != 75 {
		t.Fatalf("FiveHour.LeftPercent = %d", record.FiveHour.LeftPercent)
	}
	if record.FiveHour.ResetsAt == nil || *record.FiveHour.ResetsAt != reset {
		t.Fatalf("FiveHour.ResetsAt = %v", record.FiveHour.ResetsAt)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
