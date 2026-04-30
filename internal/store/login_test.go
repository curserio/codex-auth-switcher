package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareLoginClearsAuthAndRotatesInstallationID(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}

	if err := st.PrepareLogin(); err != nil {
		t.Fatalf("PrepareLogin error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "auth.json")); !os.IsNotExist(err) {
		t.Fatalf("auth still exists, stat err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(codexHome, "installation_id"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(bytesTrimSpace(data)); got == "" || got == "first-install" {
		t.Fatalf("installation_id = %q, want new non-empty id", got)
	}
	if _, err := os.Stat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Fatalf("current hint still exists, stat err = %v", err)
	}
}
