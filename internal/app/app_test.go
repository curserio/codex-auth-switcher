package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/usage"
)

func TestFormatReset(t *testing.T) {
	if got := formatReset(nil); got != "unknown" {
		t.Fatalf("formatReset(nil) = %q, want unknown", got)
	}
}

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

func TestStatusJSON(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
	writeTestFile(t, filepath.Join(codexHome, "installation_id"), "install-first\n")
	capture := fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 30, 40, 3600, 7200), nil)
	var stdout, stderr strings.Builder
	if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: capture, now: fixedNow(t)}); err != nil {
		t.Fatalf("add error = %v", err)
	}

	stdout.Reset()
	if err := runWithOptions([]string{"status", "--json"}, &stdout, &stderr, runOptions{capture: capture, now: fixedNow(t)}); err != nil {
		t.Fatalf("status --json error = %v", err)
	}
	if strings.Contains(stdout.String(), "id_token") || strings.Contains(stdout.String(), "access_token") {
		t.Fatalf("status json leaked auth data: %s", stdout.String())
	}
	var out statusJSON
	if err := json.Unmarshal([]byte(stdout.String()), &out); err != nil {
		t.Fatalf("parse status json: %v\n%s", err, stdout.String())
	}
	if out.Current != "first" || len(out.Accounts) != 1 || !out.Accounts[0].Active {
		t.Fatalf("unexpected status json: %+v", out)
	}
	if out.Accounts[0].Usage == nil || out.Accounts[0].Usage.FiveHour.ResetInSeconds == nil || *out.Accounts[0].Usage.FiveHour.ResetInSeconds != 3600 {
		t.Fatalf("unexpected reset seconds: %+v", out.Accounts[0].Usage)
	}
}

func TestPrepareLoginClearsAuthAndRotatesInstall(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
	writeTestFile(t, filepath.Join(codexHome, "installation_id"), "install-first\n")
	capture := fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil)
	var stdout, stderr strings.Builder
	if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: capture, now: fixedNow(t)}); err != nil {
		t.Fatalf("add error = %v", err)
	}
	if err := runWithOptions([]string{"prepare-login"}, &stdout, &stderr, runOptions{capture: capture, now: fixedNow(t)}); err != nil {
		t.Fatalf("prepare-login error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "auth.json")); !os.IsNotExist(err) {
		t.Fatalf("auth.json still exists, stat err = %v", err)
	}
	install, err := os.ReadFile(filepath.Join(codexHome, "installation_id"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(install)); got == "" || got == "install-first" {
		t.Fatalf("installation_id = %q, want new non-empty id", got)
	}
}

func TestUseRollsBackWhenTargetValidationFails(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	record := recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200)
	var stdout, stderr strings.Builder
	writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
	if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
		t.Fatalf("add first error = %v", err)
	}
	writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "second@example.com", "acct-second")
	if err := runWithOptions([]string{"add", "second"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
		t.Fatalf("add second error = %v", err)
	}

	calls := 0
	capture := func(context.Context) (usage.Record, error) {
		calls++
		if calls == 1 {
			return record, nil
		}
		return usage.Record{}, errors.New("token_revoked")
	}
	err := runWithOptions([]string{"use", "first"}, &stdout, &stderr, runOptions{capture: capture, now: fixedNow(t)})
	if err == nil || !strings.Contains(err.Error(), "restored second") {
		t.Fatalf("use error = %v, want rollback", err)
	}
	stdout.Reset()
	if err := runWithOptions([]string{"current"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
		t.Fatalf("current error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "second" {
		t.Fatalf("current = %q, want second", stdout.String())
	}
}

func TestDoctor(t *testing.T) {
	t.Run("ok active live", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_SWITCH_HOME", root)
		t.Setenv("CODEX_HOME", codexHome)
		record := recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200)
		writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
		var stdout, stderr strings.Builder
		if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("add error = %v", err)
		}
		stdout.Reset()
		if err := runWithOptions([]string{"doctor"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("doctor error = %v\n%s", err, stdout.String())
		}
		if !strings.Contains(stdout.String(), "ok live app-server capture works for first") {
			t.Fatalf("doctor output = %s", stdout.String())
		}
	})

	t.Run("duplicate identity fails", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_SWITCH_HOME", root)
		t.Setenv("CODEX_HOME", codexHome)
		record := recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200)
		writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
		var stdout, stderr strings.Builder
		if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("add error = %v", err)
		}
		writeTestAuth(t, filepath.Join(root, "accounts", "dup", "auth.json"), "first@example.com", "acct-first")
		stdout.Reset()
		err := runWithOptions([]string{"doctor"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)})
		if err == nil || !strings.Contains(stdout.String(), "fail profiles") {
			t.Fatalf("doctor err = %v output = %s", err, stdout.String())
		}
	})

	t.Run("unmanaged active warns", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_SWITCH_HOME", root)
		t.Setenv("CODEX_HOME", codexHome)
		record := recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200)
		writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
		var stdout, stderr strings.Builder
		if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("add error = %v", err)
		}
		writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "other@example.com", "acct-other")
		stdout.Reset()
		if err := runWithOptions([]string{"doctor"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("doctor error = %v output = %s", err, stdout.String())
		}
		if !strings.Contains(stdout.String(), "warn active Codex auth is unmanaged") {
			t.Fatalf("doctor output = %s", stdout.String())
		}
	})

	t.Run("live capture failure fails", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_SWITCH_HOME", root)
		t.Setenv("CODEX_HOME", codexHome)
		record := recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200)
		writeTestAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com", "acct-first")
		var stdout, stderr strings.Builder
		if err := runWithOptions([]string{"add", "first"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("add error = %v", err)
		}
		stdout.Reset()
		err := runWithOptions([]string{"doctor"}, &stdout, &stderr, runOptions{capture: fixedCapture(usage.Record{}, errors.New("boom")), now: fixedNow(t)})
		if err == nil || !strings.Contains(stdout.String(), "fail live app-server capture failed") {
			t.Fatalf("doctor err = %v output = %s", err, stdout.String())
		}
	})
}

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
