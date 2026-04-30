package store

import (
	"path/filepath"
	"testing"
)

func TestCurrentFollowsManualAuthChange(t *testing.T) {
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

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	current, err := st.Current()
	if err != nil {
		t.Fatalf("Current error = %v", err)
	}
	if current != "first" {
		t.Fatalf("Current = %q, want first", current)
	}
}
