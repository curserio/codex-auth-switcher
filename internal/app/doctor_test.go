package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/curserio/codex-auth-switcher/internal/usage"
)

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

	t.Run("invalid profile auth fails even with meta", func(t *testing.T) {
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
		if err := os.WriteFile(filepath.Join(root, "accounts", "first", "auth.json"), []byte(`{"tokens":{"id_token":"broken"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		err := runWithOptions([]string{"doctor"}, &stdout, &stderr, runOptions{capture: fixedCapture(record, nil), now: fixedNow(t)})
		if err == nil || !strings.Contains(stdout.String(), "fail profile first auth is unreadable") {
			t.Fatalf("doctor err = %v output = %s", err, stdout.String())
		}
	})
}
