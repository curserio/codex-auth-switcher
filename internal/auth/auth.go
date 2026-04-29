package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Metadata struct {
	Email     string `json:"email"`
	Subject   string `json:"subject,omitempty"`
	AuthMode  string `json:"auth_mode,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

func MetadataFromAuthJSON(data []byte) (Metadata, error) {
	var raw struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			IDToken   string `json:"id_token"`
			AccountID string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Metadata{}, fmt.Errorf("parse auth json: %w", err)
	}
	if raw.Tokens.IDToken == "" {
		return Metadata{}, errors.New("auth json does not contain tokens.id_token")
	}

	payload, err := decodeJWTPayload(raw.Tokens.IDToken)
	if err != nil {
		return Metadata{}, err
	}

	var claims struct {
		Email string `json:"email"`
		Sub   string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Metadata{}, fmt.Errorf("parse id_token payload: %w", err)
	}
	if claims.Email == "" {
		return Metadata{}, errors.New("id_token payload does not contain email")
	}

	return Metadata{
		Email:     claims.Email,
		Subject:   claims.Sub,
		AuthMode:  raw.AuthMode,
		AccountID: raw.Tokens.AccountID,
	}, nil
}

func decodeJWTPayload(token string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("id_token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode id_token payload: %w", err)
	}
	return payload, nil
}
