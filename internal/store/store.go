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

// Store owns the local codex-switch profile directory and the active Codex home.
// Methods on Store only move local files; they never call Codex network APIs.
type Store struct {
	Root      string
	CodexHome string
}

// Account is the non-secret profile state shown by list/status commands.
type Account struct {
	Name  string        `json:"name"`
	Meta  auth.Metadata `json:"meta"`
	Usage *usage.Record `json:"usage,omitempty"`
}

// New creates a Store rooted at the switcher data directory and Codex home.
func New(root, codexHome string) Store {
	return Store{Root: root, CodexHome: codexHome}
}

// DefaultRoot returns the default codex-switch profile directory.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex-auth-switcher"), nil
}

// DefaultCodexHome returns CODEX_HOME or the default Codex state directory.
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

// ValidateAccountName rejects path-like names before they are joined into store paths.
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
