package store

import (
	"encoding/base64"
	"encoding/json"
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

func TestAddAllowsMissingInstallationID(t *testing.T) {
	root := t.TempDir()
	codexHome := t.TempDir()
	st := New(root, codexHome)

	writeAuth(t, filepath.Join(codexHome, "auth.json"), "first@example.com")
	if _, err := st.Add("first"); err != nil {
		t.Fatalf("Add(first) error = %v", err)
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

func writeAuth(t *testing.T, path, email string) {
	t.Helper()
	token := makeJWT(map[string]any{"email": email, "sub": "sub-" + email})
	data := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + token + `","account_id":"acct-` + email + `"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeInstallationID(t *testing.T, path, id string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestMeta(data []byte) (string, error) {
	var raw struct {
		Tokens struct {
			IDToken string `json:"id_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	parts := strings.Split(raw.Tokens.IDToken, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	return claims.Email, nil
}

func makeJWT(payload map[string]any) string {
	header, _ := json.Marshal(map[string]any{"alg": "none"})
	body, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body) + "."
}
