package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndSwitchTo(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	meta, err := st.Add("first")
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if meta.Email != "first@example.com" {
		t.Fatalf("first email = %q", meta.Email)
	}

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "second@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "second-install")
	if _, err := st.Add("second"); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}

	if err := st.SwitchTo("first"); err != nil {
		t.Fatalf("SwitchTo(first) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(codexHome, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta, err := readTestMeta(data); err != nil || meta != "first@example.com" {
		t.Fatalf("switched auth email = %q, err = %v", meta, err)
	}
	info, err := os.Stat(filepath.Join(codexHome, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("auth permissions = %v", info.Mode().Perm())
	}
	installationID, err := os.ReadFile(filepath.Join(codexHome, "installation_id"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bytesTrimSpace(installationID)) != "first-install" {
		t.Fatalf("switched installation_id = %s, want first-install", installationID)
	}
}

func TestSwitchToLeavesInstallationIDWhenProfileHasNone(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, "accounts", "first", "installation_id")); err != nil {
		t.Fatal(err)
	}
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "active-install")

	if err := st.SwitchTo("first"); err != nil {
		t.Fatalf("SwitchTo(first) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(codexHome, "installation_id"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bytesTrimSpace(data)) != "active-install" {
		t.Fatalf("installation_id = %s, want active-install", data)
	}
}

func TestRestoreAfterFailedSwitchRestoresAuthAndCurrent(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	writeAuth(t, filepath.Join(codexHome, "auth.json"), "second@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "second-install")
	if _, err := st.Add("second"); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}

	previous, err := st.readSwitchState()
	if err != nil {
		t.Fatal(err)
	}
	firstAuth, err := os.ReadFile(filepath.Join(root, "accounts", "first", "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(filepath.Join(codexHome, "auth.json"), firstAuth, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCurrent("first"); err != nil {
		t.Fatal(err)
	}

	err = st.restoreAfterFailedSwitch(previous, errors.New("write current"))
	if err == nil || !strings.Contains(err.Error(), "restored previous auth") {
		t.Fatalf("restore error = %v, want restored previous auth", err)
	}
	data, err := os.ReadFile(filepath.Join(codexHome, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta, err := readTestMeta(data); err != nil || meta != "second@example.com" {
		t.Fatalf("restored auth email = %q, err = %v", meta, err)
	}
	current, err := st.storedCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if current != "second" {
		t.Fatalf("current = %q, want second", current)
	}
}
