package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/curserio/codex-auth-switcher/internal/auth"
)

// Current returns the active profile by inspecting auth.json, falling back to the current hint.
func (s Store) Current() (string, error) {
	if name, ok, err := s.ActiveProfile(); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	} else if ok {
		return name, nil
	}

	return s.storedCurrent()
}

func (s Store) storedCurrent() (string, error) {
	data, err := os.ReadFile(s.currentPath())
	if err != nil {
		return "", err
	}
	name := string(bytesTrimSpace(data))
	if err := ValidateAccountName(name); err != nil {
		return "", err
	}
	return name, nil
}

// SetCurrent writes the best-effort current profile hint used when auth.json is absent.
func (s Store) SetCurrent(name string) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	return WriteFileAtomic(s.currentPath(), []byte(name+"\n"), 0o600)
}

// CurrentAuthMetadata parses the active Codex auth.json without exposing token data.
func (s Store) CurrentAuthMetadata() (auth.Metadata, error) {
	data, err := os.ReadFile(s.codexAuthPath())
	if err != nil {
		return auth.Metadata{}, fmt.Errorf("read current codex auth: %w", err)
	}
	return auth.MetadataFromAuthJSON(data)
}

// ActiveProfile resolves the active auth.json to a saved profile identity.
// The boolean is false when auth.json is valid but unmanaged by this store.
func (s Store) ActiveProfile() (string, bool, error) {
	meta, err := s.CurrentAuthMetadata()
	if err != nil {
		return "", false, err
	}
	name, ok, err := s.FindProfileByMetadata(meta)
	if err != nil {
		return "", false, err
	}
	return name, ok, nil
}
