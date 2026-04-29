package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/curserio/codex-auth-switcher/internal/usage"
)

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
