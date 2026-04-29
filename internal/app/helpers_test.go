package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/usage"
)

func fixedNow(t *testing.T) func() time.Time {
	t.Helper()
	now := mustTime(t, "2026-04-29T12:00:00Z")
	return func() time.Time { return now }
}

func fixedCapture(record usage.Record, err error) captureFunc {
	return func(context.Context) (usage.Record, error) {
		return record, err
	}
}

func recordWithResets(t *testing.T, capturedAt string, fiveUsed, weeklyUsed int, fiveResetSeconds, weeklyResetSeconds int64) usage.Record {
	t.Helper()
	now := mustTime(t, capturedAt)
	fiveReset := now.Add(time.Duration(fiveResetSeconds) * time.Second).Unix()
	weeklyReset := now.Add(time.Duration(weeklyResetSeconds) * time.Second).Unix()
	return usage.Record{
		CapturedAt: now,
		Source:     "test",
		FiveHour: usage.Window{
			UsedPercent: fiveUsed,
			LeftPercent: 100 - fiveUsed,
			ResetsAt:    &fiveReset,
		},
		Weekly: usage.Window{
			UsedPercent: weeklyUsed,
			LeftPercent: 100 - weeklyUsed,
			ResetsAt:    &weeklyReset,
		},
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

func writeTestAuth(t *testing.T, path, email, accountID string) {
	t.Helper()
	token := makeTestJWT(map[string]any{"email": email, "sub": "sub-" + accountID})
	data := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + token + `","account_id":"` + accountID + `"}}`)
	writeTestFile(t, path, string(data))
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func makeTestJWT(payload map[string]any) string {
	header, _ := json.Marshal(map[string]any{"alg": "none"})
	body, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body) + "."
}
