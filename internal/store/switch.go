package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/auth"
)

var authSidecarFiles = []string{
	installationIDName,
}

type switchState struct {
	authFiles     map[string][]byte
	current       []byte
	currentExists bool
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
	previous, err := s.readSwitchState()
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
		return s.restoreAfterFailedSwitch(previous, err)
	}
	if err := s.SetCurrent(name); err != nil {
		return s.restoreAfterFailedSwitch(previous, err)
	}
	if err := s.appendSwitchLog(name); err != nil {
		return nil
	}
	return nil
}

func (s Store) readSwitchState() (switchState, error) {
	files, err := s.readCurrentAuthFiles()
	if err != nil {
		return switchState{}, err
	}
	state := switchState{authFiles: files}
	if data, err := os.ReadFile(s.currentPath()); err == nil {
		state.current = data
		state.currentExists = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return switchState{}, err
	}
	return state, nil
}

func (s Store) readCurrentAuthFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	if data, err := os.ReadFile(s.codexAuthPath()); err == nil {
		files[authFileName] = data
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, file := range authSidecarFiles {
		data, err := os.ReadFile(filepath.Join(s.CodexHome, file))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read current codex %s: %w", file, err)
		}
		files[file] = data
	}
	return files, nil
}

func (s Store) restoreAfterFailedSwitch(state switchState, cause error) error {
	if restoreErr := s.writeSwitchState(state); restoreErr != nil {
		return fmt.Errorf("switch failed: %v; restore previous auth failed: %w", cause, restoreErr)
	}
	return fmt.Errorf("switch failed: %v; restored previous auth", cause)
}

func (s Store) writeSwitchState(state switchState) error {
	if err := s.writeCurrentAuthFiles(state.authFiles); err != nil {
		return err
	}
	if state.currentExists {
		if err := os.RemoveAll(s.currentPath()); err != nil {
			return err
		}
		return WriteFileAtomic(s.currentPath(), state.current, 0o600)
	}
	if err := os.RemoveAll(s.currentPath()); err != nil {
		return err
	}
	return nil
}

func (s Store) writeCurrentAuthFiles(files map[string][]byte) error {
	if data, ok := files[authFileName]; ok {
		if err := WriteFileAtomic(s.codexAuthPath(), data, 0o600); err != nil {
			return err
		}
	} else if err := os.Remove(s.codexAuthPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, file := range authSidecarFiles {
		data, ok := files[file]
		if !ok {
			if err := os.Remove(filepath.Join(s.CodexHome, file)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			continue
		}
		if err := WriteFileAtomic(filepath.Join(s.CodexHome, file), data, 0o600); err != nil {
			return err
		}
	}
	return nil
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
		data, err := os.ReadFile(filepath.Join(s.CodexHome, file))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if removeErr := os.Remove(filepath.Join(profileDir, file)); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
					return removeErr
				}
				continue
			}
			return fmt.Errorf("read current codex %s: %w", file, err)
		}
		if err := WriteFileAtomic(filepath.Join(profileDir, file), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func readProfileAuthSidecars(profileDir string) (map[string][]byte, error) {
	files := make(map[string][]byte, len(authSidecarFiles))
	for _, file := range authSidecarFiles {
		data, err := os.ReadFile(filepath.Join(profileDir, file))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read profile %s: %w", file, err)
		}
		files[file] = data
	}
	return files, nil
}

func (s Store) writeAuthSidecarFiles(files map[string][]byte) error {
	for _, file := range authSidecarFiles {
		data, ok := files[file]
		if !ok {
			continue
		}
		if err := WriteFileAtomic(filepath.Join(s.CodexHome, file), data, 0o600); err != nil {
			return err
		}
	}
	return nil
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
