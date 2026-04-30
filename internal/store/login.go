package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func (s Store) Init() error {
	if err := os.MkdirAll(filepath.Join(s.Root, "accounts"), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "backups"), 0o700); err != nil {
		return err
	}
	return nil
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
