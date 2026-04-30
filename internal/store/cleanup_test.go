package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanCleanupSelectsOldBackupsButKeepsNewest(t *testing.T) {
	root := t.TempDir()
	st := New(root, t.TempDir())
	backupDir := filepath.Join(root, "backups")

	for _, name := range []string{
		"auth-20260430T000000.000000000Z.json",
		"auth-20260420T000000.000000000Z.json",
		"auth-20260410T000000.000000000Z.json",
		"auth-20260401T000000.000000000Z.json",
		"notes.txt",
		"auth-invalid.json",
	} {
		writeCleanupFile(t, filepath.Join(backupDir, name), "backup")
	}

	plan, err := st.PlanCleanup(CleanupOptions{Days: 14, Keep: 2, LogLines: 1000}, mustCleanupTime(t, "2026-04-30T12:00:00Z"))
	if err != nil {
		t.Fatalf("PlanCleanup error = %v", err)
	}
	got := cleanupBackupNames(plan.Backups)
	want := []string{
		"auth-20260410T000000.000000000Z.json",
		"auth-20260401T000000.000000000Z.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup backups = %v, want %v", got, want)
	}
}

func TestApplyCleanupDeletesBackupsAndTrimsLog(t *testing.T) {
	root := t.TempDir()
	st := New(root, t.TempDir())

	oldBackup := filepath.Join(root, "backups", "auth-20260401T000000.000000000Z.json")
	newBackup := filepath.Join(root, "backups", "auth-20260430T000000.000000000Z.json")
	otherFile := filepath.Join(root, "backups", "notes.txt")
	writeCleanupFile(t, oldBackup, "old")
	writeCleanupFile(t, newBackup, "new")
	writeCleanupFile(t, otherFile, "notes")
	writeCleanupFile(t, filepath.Join(root, switchLogName), "one\ntwo\nthree\n")

	plan, err := st.PlanCleanup(CleanupOptions{Days: 14, Keep: 1, LogLines: 2}, mustCleanupTime(t, "2026-04-30T12:00:00Z"))
	if err != nil {
		t.Fatalf("PlanCleanup error = %v", err)
	}
	if err := st.ApplyCleanup(plan); err != nil {
		t.Fatalf("ApplyCleanup error = %v", err)
	}

	if _, err := os.Stat(oldBackup); !os.IsNotExist(err) {
		t.Fatalf("old backup stat err = %v, want not exist", err)
	}
	for _, path := range []string{newBackup, otherFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to remain: %v", path, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, switchLogName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two\nthree\n" {
		t.Fatalf("trimmed log = %q, want last two lines", data)
	}
}

func TestPlanCleanupDoesNotMutateFiles(t *testing.T) {
	root := t.TempDir()
	st := New(root, t.TempDir())
	backup := filepath.Join(root, "backups", "auth-20260401T000000.000000000Z.json")
	writeCleanupFile(t, backup, "old")

	if _, err := st.PlanCleanup(CleanupOptions{Days: 14, Keep: 0, LogLines: 1000}, mustCleanupTime(t, "2026-04-30T12:00:00Z")); err != nil {
		t.Fatalf("PlanCleanup error = %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup changed during plan: %v", err)
	}
}

func TestPlanCleanupRejectsInvalidOptions(t *testing.T) {
	st := New(t.TempDir(), t.TempDir())
	tests := []CleanupOptions{
		{Days: -1, Keep: 10, LogLines: 1000},
		{Days: 30, Keep: -1, LogLines: 1000},
		{Days: 30, Keep: 10, LogLines: -1},
	}
	for _, tt := range tests {
		if _, err := st.PlanCleanup(tt, time.Now()); err == nil {
			t.Fatalf("PlanCleanup(%+v) error = nil, want error", tt)
		}
	}
}

func writeCleanupFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func cleanupBackupNames(backups []CleanupBackup) []string {
	names := make([]string, len(backups))
	for i, backup := range backups {
		names[i] = backup.Name
	}
	return names
}

func mustCleanupTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestTrimSwitchLogKeepsPartialLastLine(t *testing.T) {
	root := t.TempDir()
	st := New(root, t.TempDir())
	writeCleanupFile(t, filepath.Join(root, switchLogName), "one\ntwo\nthree")

	plan, err := st.PlanCleanup(CleanupOptions{Days: 30, Keep: 10, LogLines: 2}, time.Now())
	if err != nil {
		t.Fatalf("PlanCleanup error = %v", err)
	}
	if err := st.ApplyCleanup(plan); err != nil {
		t.Fatalf("ApplyCleanup error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, switchLogName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "two\nthree") {
		t.Fatalf("trimmed log = %q, want last partial line preserved", data)
	}
}
