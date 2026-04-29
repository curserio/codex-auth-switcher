package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestMetadataFromAuthJSON(t *testing.T) {
	token := makeJWT(map[string]any{
		"email": "user@example.com",
		"sub":   "google-oauth2|123",
	})
	authJSON := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + token + `","account_id":"acct_123"}}`)

	meta, err := MetadataFromAuthJSON(authJSON)
	if err != nil {
		t.Fatalf("MetadataFromAuthJSON() error = %v", err)
	}
	if meta.Email != "user@example.com" {
		t.Fatalf("Email = %q", meta.Email)
	}
	if meta.Subject != "google-oauth2|123" {
		t.Fatalf("Subject = %q", meta.Subject)
	}
	if meta.AuthMode != "chatgpt" {
		t.Fatalf("AuthMode = %q", meta.AuthMode)
	}
	if meta.AccountID != "acct_123" {
		t.Fatalf("AccountID = %q", meta.AccountID)
	}
}

func makeJWT(payload map[string]any) string {
	header, _ := json.Marshal(map[string]any{"alg": "none"})
	body, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body) + "."
}
