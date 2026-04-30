package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/store"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

type captureFunc func(context.Context) (usage.Record, error)

type runOptions struct {
	capture captureFunc
	now     func() time.Time
}

// App wires command handlers to the profile store and live usage capture.
type App struct {
	stderr  io.Writer
	stdout  io.Writer
	store   store.Store
	capture captureFunc
	now     func() time.Time
}

// Run executes the codex-switch CLI with process-level defaults.
func Run(args []string, stdout, stderr io.Writer) error {
	return runWithOptions(args, stdout, stderr, runOptions{
		capture: capture,
		now:     time.Now,
	})
}

func runWithOptions(args []string, stdout, stderr io.Writer, opts runOptions) error {
	if opts.capture == nil {
		opts.capture = capture
	}
	if opts.now == nil {
		opts.now = time.Now
	}

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return nil
	}

	root, err := envOrDefaultPath("CODEX_SWITCH_HOME", store.DefaultRoot)
	if err != nil {
		return err
	}
	codexHome, err := envOrDefaultPath("CODEX_HOME", store.DefaultCodexHome)
	if err != nil {
		return err
	}

	app := App{
		stderr:  stderr,
		stdout:  stdout,
		store:   store.New(root, codexHome),
		capture: opts.capture,
		now:     opts.now,
	}
	return app.run(args, root)
}

func (a App) run(args []string, root string) error {
	switch args[0] {
	case "init":
		return a.runInit(args, root)
	case "add":
		return a.runAdd(args)
	case "current":
		return a.runCurrent(args)
	case "list":
		return a.runList(args)
	case "status":
		return a.runStatus(args)
	case "capture":
		return a.runCapture(args)
	case "prepare-login", "prepare":
		return a.runPrepareLogin(args)
	case "doctor":
		return a.runDoctorCommand(args)
	case "use":
		return a.runUse(args)
	case "rename":
		return a.runRename(args)
	case "delete", "remove":
		return a.runDelete(args)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func capture(ctx context.Context) (usage.Record, error) {
	record, err := usage.CaptureFromAppServer(ctx)
	if err == nil {
		return record, nil
	}
	return usage.Record{}, fmt.Errorf("app-server: %v", err)
}

func envOrDefaultPath(env string, fn func() (string, error)) (string, error) {
	if v := os.Getenv(env); v != "" {
		return filepath.Clean(v), nil
	}
	return fn()
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `codex-switch manages local Codex ChatGPT auth profiles.

Usage:
  codex-switch init
  codex-switch add <name>
  codex-switch use <name>
  codex-switch rename <old> <new>
  codex-switch delete <name>
  codex-switch capture
  codex-switch prepare-login
  codex-switch doctor
  codex-switch status [--json]
  codex-switch list
  codex-switch current

Environment:
  CODEX_HOME          defaults to ~/.codex
  CODEX_SWITCH_HOME  defaults to ~/.codex-auth-switcher
  CODEX_SWITCH_CODEX_BIN  defaults to codex`)
}

func usageError(command string) error {
	return errors.New("usage: codex-switch " + command)
}
