package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/curserio/codex-auth-switcher/internal/auth"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

const (
	authFileName       = "auth.json"
	installationIDName = "installation_id"
	metaFileName       = "meta.json"
	usageFileName      = "usage.json"
	currentFileName    = "current"
	switchLogName      = "switch.log"
)

var accountNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Store struct {
	Root      string
	CodexHome string
}

type Account struct {
	Name  string        `json:"name"`
	Meta  auth.Metadata `json:"meta"`
	Usage *usage.Record `json:"usage,omitempty"`
}

func New(root, codexHome string) Store {
	return Store{Root: root, CodexHome: codexHome}
}

func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex-auth-switcher"), nil
}

func DefaultCodexHome() (string, error) {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

func ValidateAccountName(name string) error {
	if name == "" {
		return errors.New("account name is required")
	}
	if !accountNamePattern.MatchString(name) {
		return errors.New("account name must contain only ASCII letters, digits, '_', '-' or '.'")
	}
	if name == "." || name == ".." {
		return errors.New("account name cannot be '.' or '..'")
	}
	return nil
}

func (s Store) codexAuthPath() string {
	return filepath.Join(s.CodexHome, authFileName)
}

func (s Store) currentPath() string {
	return filepath.Join(s.Root, currentFileName)
}

func (s Store) accountDir(name string) string {
	return filepath.Join(s.Root, "accounts", name)
}

func bytesTrimSpace(data []byte) []byte {
	start, end := 0, len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}
