package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAccountName(t *testing.T) {
	valid := []string{"main", "work-2", "other.email", "x_y"}
	for _, name := range valid {
		if err := ValidateAccountName(name); err != nil {
			t.Fatalf("ValidateAccountName(%q) unexpected error: %v", name, err)
		}
	}
	invalid := []string{"", "../x", "a/b", "space name", "."}
	for _, name := range invalid {
		if err := ValidateAccountName(name); err == nil {
			t.Fatalf("ValidateAccountName(%q) expected error", name)
		}
	}
}

func TestAddAllowsMissingInstallationID(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
}

func TestAddRejectsDuplicateCurrentAuth(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}

	if _, err := st.Add("second"); err == nil {
		t.Fatal("Add(second) expected duplicate error")
	}
}

func TestRenameAndDelete(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	writeInstallationID(t, filepath.Join(codexHome, "installation_id"), "first-install")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if err := st.Rename("first", "main"); err != nil {
		t.Fatalf("Rename error = %v", err)
	}
	if _, err := st.ReadAccount("main"); err != nil {
		t.Fatalf("ReadAccount(main) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "accounts", "first")); !os.IsNotExist(err) {
		t.Fatalf("old account dir still exists, stat err = %v", err)
	}
	if err := st.Delete("main"); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "accounts", "main")); !os.IsNotExist(err) {
		t.Fatalf("deleted account dir still exists, stat err = %v", err)
	}
}

func TestReadAccountRejectsInvalidAuthEvenWithMeta(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "accounts", "first", "auth.json"), []byte(`{"tokens":{"id_token":"broken"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadAccount("first"); err == nil || !strings.Contains(err.Error(), "read profile auth") {
		t.Fatalf("ReadAccount error = %v, want profile auth error", err)
	}
}
