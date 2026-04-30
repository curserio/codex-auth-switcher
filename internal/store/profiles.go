package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/curserio/codex-auth-switcher/internal/auth"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

// Add saves the current Codex auth as a named profile and marks it current.
func (s Store) Add(name string) (auth.Metadata, error) {
	if err := ValidateAccountName(name); err != nil {
		return auth.Metadata{}, err
	}
	if err := s.Init(); err != nil {
		return auth.Metadata{}, err
	}

	data, err := os.ReadFile(s.codexAuthPath())
	if err != nil {
		return auth.Metadata{}, fmt.Errorf("read current codex auth: %w", err)
	}
	meta, err := auth.MetadataFromAuthJSON(data)
	if err != nil {
		return auth.Metadata{}, err
	}

	if existing, ok, err := s.FindProfileByMetadata(meta); err != nil {
		return auth.Metadata{}, err
	} else if ok && existing != name {
		return auth.Metadata{}, fmt.Errorf("current auth is already saved as %q", existing)
	}

	dir := s.accountDir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return auth.Metadata{}, err
	}
	if err := WriteFileAtomic(filepath.Join(dir, authFileName), data, 0o600); err != nil {
		return auth.Metadata{}, err
	}
	if err := s.saveAuthSidecarFiles(dir); err != nil {
		return auth.Metadata{}, err
	}
	if err := WriteJSONAtomic(filepath.Join(dir, metaFileName), meta, 0o600); err != nil {
		return auth.Metadata{}, err
	}
	if err := s.SetCurrent(name); err != nil {
		return auth.Metadata{}, err
	}
	return meta, nil
}

// FindProfileByMetadata matches profiles by stable identity, not by profile name.
func (s Store) FindProfileByMetadata(meta auth.Metadata) (string, bool, error) {
	names, err := s.ProfileNames()
	if err != nil {
		return "", false, err
	}
	for _, name := range names {
		acct, err := s.ReadAccount(name)
		if err != nil {
			return "", false, err
		}
		if sameIdentity(meta, acct.Meta) {
			return acct.Name, true, nil
		}
	}
	return "", false, nil
}

// List returns all readable profiles.
// A corrupt profile is returned as an error so doctor/status cannot hide it.
func (s Store) List() ([]Account, error) {
	names, err := s.ProfileNames()
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(names))
	for _, name := range names {
		acct, err := s.ReadAccount(name)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acct)
	}
	return accounts, nil
}

// ProfileNames returns valid profile directory names without parsing profile contents.
func (s Store) ProfileNames() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "accounts"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if ValidateAccountName(name) != nil {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// ReadAccount parses profile auth and cached usage without returning token material.
// meta.json is treated as a cache; auth.json remains the source of truth.
func (s Store) ReadAccount(name string) (Account, error) {
	if err := ValidateAccountName(name); err != nil {
		return Account{}, err
	}
	dir := s.accountDir(name)
	authData, err := os.ReadFile(filepath.Join(dir, authFileName))
	if err != nil {
		return Account{}, fmt.Errorf("read profile auth: %w", err)
	}
	meta, err := auth.MetadataFromAuthJSON(authData)
	if err != nil {
		return Account{}, fmt.Errorf("read profile auth: %w", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, metaFileName)); err == nil {
		var cached auth.Metadata
		if json.Unmarshal(data, &cached) == nil && sameIdentity(meta, cached) {
			meta.AuthMode = valueOr(meta.AuthMode, cached.AuthMode)
			meta.AccountID = valueOr(meta.AccountID, cached.AccountID)
		}
	}
	var usageRecord *usage.Record
	if data, err := os.ReadFile(filepath.Join(dir, usageFileName)); err == nil {
		var record usage.Record
		if json.Unmarshal(data, &record) == nil {
			usageRecord = &record
		}
	}
	return Account{Name: name, Meta: meta, Usage: usageRecord}, nil
}

// SaveUsage stores the latest non-secret usage snapshot for a profile.
func (s Store) SaveUsage(name string, record usage.Record) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	return WriteJSONAtomic(filepath.Join(s.accountDir(name), usageFileName), record, 0o600)
}

// Rename moves a profile directory and updates the stored current hint when needed.
func (s Store) Rename(oldName, newName string) error {
	if err := ValidateAccountName(oldName); err != nil {
		return err
	}
	if err := ValidateAccountName(newName); err != nil {
		return err
	}
	if oldName == newName {
		return nil
	}
	oldDir := s.accountDir(oldName)
	newDir := s.accountDir(newName)
	if _, err := os.Stat(oldDir); err != nil {
		return fmt.Errorf("read profile %q: %w", oldName, err)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("profile %q already exists", newName)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	if current, err := s.storedCurrent(); err == nil && current == oldName {
		return s.SetCurrent(newName)
	}
	return nil
}

// Delete removes a profile and clears the current hint if it pointed at that profile.
func (s Store) Delete(name string) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	if err := os.RemoveAll(s.accountDir(name)); err != nil {
		return err
	}
	if current, err := s.storedCurrent(); err == nil && current == name {
		if err := os.Remove(s.currentPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func sameIdentity(a, b auth.Metadata) bool {
	if a.Subject != "" && b.Subject != "" {
		return a.Subject == b.Subject
	}
	if a.AccountID != "" && b.AccountID != "" {
		return a.AccountID == b.AccountID
	}
	return a.Email != "" && b.Email != "" && a.Email == b.Email
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
