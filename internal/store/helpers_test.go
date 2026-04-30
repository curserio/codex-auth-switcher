package store

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAuth(t *testing.T, path, email string) {
	t.Helper()
	token := makeJWT(map[string]any{"email": email, "sub": "sub-" + email})
	data := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + token + `","account_id":"acct-` + email + `"}}`)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeInstallationID(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
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
