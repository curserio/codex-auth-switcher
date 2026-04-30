package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupDryRunDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	backup := filepath.Join(root, "backups", "auth-20260401T000000.000000000Z.json")
	writeTestFile(t, backup, "old")

	var stdout, stderr strings.Builder
	err := runWithOptions([]string{"cleanup", "--days", "14", "--keep", "0"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)})
	if err != nil {
		t.Fatalf("cleanup dry-run error = %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup changed during dry-run: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "cleanup dry-run") || !strings.Contains(got, "would delete backup auth-20260401T000000.000000000Z.json") {
		t.Fatalf("stdout = %q, want dry-run deletion", got)
	}
}

func TestCleanupApplyDeletesAndTrims(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	backup := filepath.Join(root, "backups", "auth-20260401T000000.000000000Z.json")
	writeTestFile(t, backup, "old")
	writeTestFile(t, filepath.Join(root, "switch.log"), "one\ntwo\nthree\n")

	var stdout, stderr strings.Builder
	err := runWithOptions([]string{"cleanup", "--apply", "--days", "14", "--keep", "0", "--log-lines", "1"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)})
	if err != nil {
		t.Fatalf("cleanup apply error = %v", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("backup stat err = %v, want not exist", err)
	}
	logData, err := os.ReadFile(filepath.Join(root, "switch.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(logData) != "three\n" {
		t.Fatalf("switch.log = %q, want last line", logData)
	}
	if got := stdout.String(); !strings.Contains(got, "deleted backup") || !strings.Contains(got, "cleanup complete") {
		t.Fatalf("stdout = %q, want applied cleanup", got)
	}
}

func TestCleanupRejectsInvalidFlags(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_SWITCH_HOME", root)
	t.Setenv("CODEX_HOME", codexHome)

	var stdout, stderr strings.Builder
	err := runWithOptions([]string{"cleanup", "--days", "-1"}, &stdout, &stderr, runOptions{capture: fixedCapture(recordWithResets(t, "2026-04-29T12:00:00Z", 10, 20, 3600, 7200), nil), now: fixedNow(t)})
	if err == nil || !strings.Contains(err.Error(), "cleanup days") {
		t.Fatalf("cleanup error = %v, want cleanup days validation", err)
	}
}
