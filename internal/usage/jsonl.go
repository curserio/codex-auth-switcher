package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func CaptureFromJSONL(codexHome string) (Record, error) {
	files, err := rolloutFiles(codexHome)
	if err != nil {
		return Record{}, err
	}
	var best *Record
	for _, file := range files {
		record, ok := lastSnapshotInFile(file)
		if !ok {
			continue
		}
		if best == nil || record.CapturedAt.After(best.CapturedAt) {
			copy := record
			best = &copy
		}
	}
	if best == nil {
		return Record{}, errors.New("no rate limit snapshots found in rollout jsonl files")
	}
	return *best, nil
}

func rolloutFiles(codexHome string) ([]string, error) {
	var files []string
	for _, root := range []string{
		filepath.Join(codexHome, "sessions"),
		filepath.Join(codexHome, "archived_sessions"),
	} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) == ".jsonl" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func lastSnapshotInFile(path string) (Record, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Record{}, false
	}
	defer f.Close()

	var last Record
	found := false
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var line struct {
			Timestamp time.Time `json:"timestamp"`
			Type      string    `json:"type"`
			Payload   struct {
				Type      string          `json:"type"`
				RateLimit json.RawMessage `json:"rate_limits"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Type != "event_msg" || line.Payload.Type != "token_count" || len(line.Payload.RateLimit) == 0 {
			continue
		}
		var snapshot Snapshot
		if err := json.Unmarshal(line.Payload.RateLimit, &snapshot); err != nil {
			continue
		}
		last = Normalize(snapshot, "jsonl", line.Timestamp)
		found = true
	}
	return last, found
}
