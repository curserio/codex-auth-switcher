package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestStatusActiveLabel(t *testing.T) {
	t.Run("saved active profile", func(t *testing.T) {
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
		if err := runWithOptions([]string{"status"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("status error = %v", err)
		}
		if !strings.Contains(stdout.String(), "Current: first") || !strings.Contains(stdout.String(), "* first") {
			t.Fatalf("status output = %s", stdout.String())
		}
	})

	t.Run("unmanaged active auth", func(t *testing.T) {
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
		if err := runWithOptions([]string{"status"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("status error = %v", err)
		}
		if !strings.Contains(stdout.String(), "Current: unmanaged (other@example.com)") {
			t.Fatalf("status output = %s", stdout.String())
		}
	})

	t.Run("missing active auth uses stored current", func(t *testing.T) {
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
		if err := os.Remove(filepath.Join(codexHome, "auth.json")); err != nil {
			t.Fatal(err)
		}

		stdout.Reset()
		if err := runWithOptions([]string{"status"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("status error = %v", err)
		}
		if !strings.Contains(stdout.String(), "Current: first") {
			t.Fatalf("status output = %s", stdout.String())
		}
	})

	t.Run("missing active auth and stored current is unknown", func(t *testing.T) {
		root := t.TempDir()
		codexHome := t.TempDir()
		t.Setenv("CODEX_SWITCH_HOME", root)
		t.Setenv("CODEX_HOME", codexHome)

		var stdout, stderr strings.Builder
		if err := runWithOptions([]string{"init"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("init error = %v", err)
		}

		stdout.Reset()
		if err := runWithOptions([]string{"status"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)}); err != nil {
			t.Fatalf("status error = %v", err)
		}
		if !strings.Contains(stdout.String(), "Current: unknown") {
			t.Fatalf("status output = %s", stdout.String())
		}
	})
}

func TestStatusFailsOnInvalidStoredCurrent(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	writeTestFile(t, filepath.Join(root, "current"), "../bad\n")
	var stdout, stderr strings.Builder
	err := runWithOptions([]string{"status"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)})
	if err == nil || !strings.Contains(err.Error(), "current") {
		t.Fatalf("status error = %v, want current error", err)
	}
}
