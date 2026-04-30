package store

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupTimeLayout = "20060102T150405.000000000Z"

// CleanupOptions controls retention for generated switcher artifacts.
type CleanupOptions struct {
	Days     int
	Keep     int
	LogLines int
}

// CleanupPlan describes the files that would be changed by cleanup.
type CleanupPlan struct {
	Backups []CleanupBackup
	Log     CleanupLog
}

// CleanupBackup is a single backup file selected for deletion.
type CleanupBackup struct {
	Name string
	Path string
	Time time.Time
}

// CleanupLog describes a switch.log trim operation.
type CleanupLog struct {
	Path       string
	Exists     bool
	LineCount  int
	KeepLines  int
	TrimNeeded bool
}

type cleanupBackupCandidate struct {
	name string
	path string
	when time.Time
}

// PlanCleanup returns the cleanup work for switcher-managed backups and logs.
func (s Store) PlanCleanup(options CleanupOptions, now time.Time) (CleanupPlan, error) {
	if options.Days < 0 {
		return CleanupPlan{}, errors.New("cleanup days must be non-negative")
	}
	if options.Keep < 0 {
		return CleanupPlan{}, errors.New("cleanup keep must be non-negative")
	}
	if options.LogLines < 0 {
		return CleanupPlan{}, errors.New("cleanup log-lines must be non-negative")
	}

	backups, err := s.cleanupBackups(options, now)
	if err != nil {
		return CleanupPlan{}, err
	}
	log, err := s.cleanupLog(options)
	if err != nil {
		return CleanupPlan{}, err
	}
	return CleanupPlan{Backups: backups, Log: log}, nil
}

// ApplyCleanup performs a previously planned cleanup.
func (s Store) ApplyCleanup(plan CleanupPlan) error {
	for _, backup := range plan.Backups {
		if err := os.Remove(backup.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if plan.Log.TrimNeeded {
		if err := s.trimSwitchLog(plan.Log.KeepLines); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) cleanupBackups(options CleanupOptions, now time.Time) ([]CleanupBackup, error) {
	dir := filepath.Join(s.Root, "backups")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	candidates := make([]cleanupBackupCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		when, ok := parseBackupName(entry.Name())
		if !ok {
			continue
		}
		candidates = append(candidates, cleanupBackupCandidate{
			name: entry.Name(),
			path: filepath.Join(dir, entry.Name()),
			when: when,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].when.After(candidates[j].when)
	})

	cutoff := now.AddDate(0, 0, -options.Days)
	var selected []CleanupBackup
	for i, backup := range candidates {
		if i < options.Keep {
			continue
		}
		if !backup.when.Before(cutoff) {
			continue
		}
		selected = append(selected, CleanupBackup{
			Name: backup.name,
			Path: backup.path,
			Time: backup.when,
		})
	}
	return selected, nil
}

func parseBackupName(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "auth-") || !strings.HasSuffix(name, ".json") {
		return time.Time{}, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(name, "auth-"), ".json")
	when, err := time.Parse(backupTimeLayout, value)
	if err != nil {
		return time.Time{}, false
	}
	return when, true
}

func (s Store) cleanupLog(options CleanupOptions) (CleanupLog, error) {
	path := filepath.Join(s.Root, switchLogName)
	lineCount, err := countLines(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CleanupLog{Path: path, KeepLines: options.LogLines}, nil
		}
		return CleanupLog{}, err
	}
	return CleanupLog{
		Path:       path,
		Exists:     true,
		LineCount:  lineCount,
		KeepLines:  options.LogLines,
		TrimNeeded: lineCount > options.LogLines,
	}, nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func (s Store) trimSwitchLog(keepLines int) error {
	path := filepath.Join(s.Root, switchLogName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if keepLines < len(lines) {
		lines = lines[len(lines)-keepLines:]
	}
	return WriteFileAtomic(path, []byte(strings.Join(lines, "")), 0o600)
}
