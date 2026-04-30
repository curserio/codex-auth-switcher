package usage

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

func CaptureFromAppServer(ctx context.Context) (Record, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "app-server", "--listen", "stdio://")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Record{}, err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Record{}, err
	}
	if err := cmd.Start(); err != nil {
		return Record{}, err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	enc := json.NewEncoder(stdin)
	dec := json.NewDecoder(stdout)
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"clientInfo": map[string]any{
				"name":    "codex-switch",
				"title":   "Codex Account",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{"experimentalApi": true},
		},
	}); err != nil {
		return Record{}, err
	}
	if _, err := readResponse(dec, 1); err != nil {
		return Record{}, err
	}
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "account/rateLimits/read",
	}); err != nil {
		return Record{}, err
	}
	raw, err := readResponse(dec, 2)
	if err != nil {
		return Record{}, err
	}

	var response struct {
		Result struct {
			RateLimits camelSnapshot `json:"rateLimits"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return Record{}, err
	}
	if response.Error != nil {
		return Record{}, errors.New(response.Error.Message)
	}
	return Normalize(snapshotFromCamel(response.Result.RateLimits), "app-server", time.Now()), nil
}

func readResponse(dec *json.Decoder, id int) (json.RawMessage, error) {
	for {
		var line json.RawMessage
		if err := dec.Decode(&line); err != nil {
			return nil, err
		}
		var header struct {
			ID    *int `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(line, &header); err != nil {
			continue
		}
		if header.ID != nil && *header.ID == id {
			if header.Error != nil {
				return nil, errors.New(header.Error.Message)
			}
			return line, nil
		}
	}
}
