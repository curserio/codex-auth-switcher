package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

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

type authSidecarFile struct {
	Name              string
	RemoveWhenMissing bool
}

var authSidecarFiles = []authSidecarFile{
	{Name: installationIDName},
}

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

func (s Store) Init() error {
	if err := os.MkdirAll(filepath.Join(s.Root, "accounts"), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "backups"), 0o700); err != nil {
		return err
	}
	return nil
}

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

func (s Store) SetCurrent(name string) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	return WriteFileAtomic(s.currentPath(), []byte(name+"\n"), 0o600)
}

func (s Store) CurrentAuthMetadata() (auth.Metadata, error) {
	data, err := os.ReadFile(s.codexAuthPath())
	if err != nil {
		return auth.Metadata{}, fmt.Errorf("read current codex auth: %w", err)
	}
	return auth.MetadataFromAuthJSON(data)
}

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

func (s Store) FindProfileByMetadata(meta auth.Metadata) (string, bool, error) {
	accounts, err := s.List()
	if err != nil {
		return "", false, err
	}
	for _, acct := range accounts {
		if sameIdentity(meta, acct.Meta) {
			return acct.Name, true, nil
		}
	}
	return "", false, nil
}

func (s Store) List() ([]Account, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "accounts"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var accounts []Account
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if ValidateAccountName(name) != nil {
			continue
		}
		acct, err := s.ReadAccount(name)
		if err != nil {
			accounts = append(accounts, Account{Name: name})
			continue
		}
		accounts = append(accounts, acct)
	}
	return accounts, nil
}

func (s Store) ReadAccount(name string) (Account, error) {
	if err := ValidateAccountName(name); err != nil {
		return Account{}, err
	}
	dir := s.accountDir(name)
	var meta auth.Metadata
	if data, err := os.ReadFile(filepath.Join(dir, metaFileName)); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	if meta.Email == "" {
		if data, err := os.ReadFile(filepath.Join(dir, authFileName)); err == nil {
			if parsed, err := auth.MetadataFromAuthJSON(data); err == nil {
				meta = parsed
			}
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

func (s Store) SaveUsage(name string, record usage.Record) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	return WriteJSONAtomic(filepath.Join(s.accountDir(name), usageFileName), record, 0o600)
}

func (s Store) PrepareLogin() error {
	if err := s.Init(); err != nil {
		return err
	}
	if err := s.backupCurrentAuth(); err != nil {
		return err
	}
	if err := os.Remove(s.codexAuthPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	id, err := newInstallationID()
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(filepath.Join(s.CodexHome, installationIDName), []byte(id+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Remove(s.currentPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

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

func (s Store) SaveCurrentAuthToProfile(name string) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	data, err := os.ReadFile(s.codexAuthPath())
	if err != nil {
		return fmt.Errorf("read current codex auth: %w", err)
	}
	dir := s.accountDir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := WriteFileAtomic(filepath.Join(dir, authFileName), data, 0o600); err != nil {
		return err
	}
	if err := s.saveAuthSidecarFiles(dir); err != nil {
		return err
	}
	if meta, err := auth.MetadataFromAuthJSON(data); err == nil {
		_ = WriteJSONAtomic(filepath.Join(dir, metaFileName), meta, 0o600)
	}
	return nil
}

func (s Store) SwitchTo(name string) error {
	if err := ValidateAccountName(name); err != nil {
		return err
	}
	profileAuth := filepath.Join(s.accountDir(name), authFileName)
	data, err := os.ReadFile(profileAuth)
	if err != nil {
		return fmt.Errorf("read profile auth: %w", err)
	}
	if _, err := auth.MetadataFromAuthJSON(data); err != nil {
		return fmt.Errorf("validate profile auth: %w", err)
	}
	sidecars, err := readProfileAuthSidecars(s.accountDir(name))
	if err != nil {
		return err
	}
	if err := s.backupCurrentAuth(); err != nil {
		return err
	}
	if err := WriteFileAtomic(s.codexAuthPath(), data, 0o600); err != nil {
		return err
	}
	if err := s.writeAuthSidecarFiles(sidecars); err != nil {
		return err
	}
	if err := s.SetCurrent(name); err != nil {
		return err
	}
	return s.appendSwitchLog(name)
}

func (s Store) backupCurrentAuth() error {
	data, err := os.ReadFile(s.codexAuthPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "backups"), 0o700); err != nil {
		return err
	}
	name := "auth-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".json"
	return WriteFileAtomic(filepath.Join(s.Root, "backups", name), data, 0o600)
}

func (s Store) saveAuthSidecarFiles(profileDir string) error {
	for _, file := range authSidecarFiles {
		data, err := os.ReadFile(filepath.Join(s.CodexHome, file.Name))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if removeErr := os.Remove(filepath.Join(profileDir, file.Name)); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
					return removeErr
				}
				continue
			}
			return fmt.Errorf("read current codex %s: %w", file.Name, err)
		}
		if err := WriteFileAtomic(filepath.Join(profileDir, file.Name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func readProfileAuthSidecars(profileDir string) (map[string][]byte, error) {
	files := make(map[string][]byte, len(authSidecarFiles))
	for _, file := range authSidecarFiles {
		data, err := os.ReadFile(filepath.Join(profileDir, file.Name))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read profile %s: %w", file.Name, err)
		}
		files[file.Name] = data
	}
	return files, nil
}

func (s Store) writeAuthSidecarFiles(files map[string][]byte) error {
	for _, file := range authSidecarFiles {
		data, ok := files[file.Name]
		if !ok {
			if !file.RemoveWhenMissing {
				continue
			}
			if err := os.Remove(filepath.Join(s.CodexHome, file.Name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			continue
		}
		if err := WriteFileAtomic(filepath.Join(s.CodexHome, file.Name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func newInstallationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}

func (s Store) appendSwitchLog(name string) error {
	line := fmt.Sprintf("%s switched_to=%s\n", time.Now().Format(time.RFC3339), name)
	f, err := os.OpenFile(filepath.Join(s.Root, switchLogName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
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

func sameIdentity(a, b auth.Metadata) bool {
	if a.Subject != "" && b.Subject != "" {
		return a.Subject == b.Subject
	}
	if a.AccountID != "" && b.AccountID != "" {
		return a.AccountID == b.AccountID
	}
	return a.Email != "" && b.Email != "" && a.Email == b.Email
}
