package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/store"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

func Run(args []string, stdout, stderr io.Writer) error {
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
	st := store.New(root, codexHome)

	switch args[0] {
	case "init":
		if err := st.Init(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "initialized %s\n", root)
	case "add":
		if len(args) != 2 {
			return errors.New("usage: codex-switch add <name>")
		}
		meta, err := st.Add(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "saved %s (%s)\n", args[1], meta.Email)
	case "current":
		name, err := st.Current()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, name)
	case "list":
		return printList(stdout, st)
	case "status":
		return printStatus(stdout, st)
	case "capture":
		name, err := activeProfileName(st)
		if err != nil {
			return err
		}
		record, err := capture(context.Background(), codexHome)
		if err != nil {
			return err
		}
		if err := st.SaveUsage(name, record); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "captured usage for %s from %s\n", name, record.Source)
	case "use":
		if len(args) != 2 {
			return errors.New("usage: codex-switch use <name>")
		}
		return useAccount(stdout, stderr, st, codexHome, args[1])
	case "rename":
		if len(args) != 3 {
			return errors.New("usage: codex-switch rename <old> <new>")
		}
		if err := st.Rename(args[1], args[2]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "renamed %s to %s\n", args[1], args[2])
	case "delete", "remove":
		if len(args) != 2 {
			return errors.New("usage: codex-switch delete <name>")
		}
		if err := st.Delete(args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "deleted %s\n", args[1])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func useAccount(stdout, stderr io.Writer, st store.Store, codexHome, target string) error {
	current, active, currentErr := st.ActiveProfile()
	if currentErr == nil && active {
		if record, err := capture(context.Background(), codexHome); err == nil {
			_ = st.SaveUsage(current, record)
			fmt.Fprintf(stderr, "captured %s usage from %s\n", current, record.Source)
		} else {
			fmt.Fprintf(stderr, "warning: could not capture usage for %s: %v\n", current, err)
		}
		if err := st.SaveCurrentAuthToProfile(current); err != nil {
			return err
		}
	} else if currentErr == nil {
		meta, err := st.CurrentAuthMetadata()
		if err == nil {
			fmt.Fprintf(stderr, "warning: current Codex auth (%s) is not a saved profile; not overwriting stored profiles\n", valueOr(meta.Email, "unknown"))
		}
	} else if !errors.Is(currentErr, os.ErrNotExist) {
		return currentErr
	}
	if err := st.SwitchTo(target); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "switched to %s\n", target)
	fmt.Fprintln(stdout, "restart/resume Codex or reload VS Code if an existing process keeps the old token in memory")
	return nil
}

func activeProfileName(st store.Store) (string, error) {
	if name, ok, err := st.ActiveProfile(); err != nil {
		return "", err
	} else if ok {
		return name, nil
	}
	meta, err := st.CurrentAuthMetadata()
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("current Codex auth (%s) is not a saved profile; run codex-switch add <name> first", valueOr(meta.Email, "unknown"))
}

func capture(ctx context.Context, codexHome string) (usage.Record, error) {
	record, err := usage.CaptureFromAppServer(ctx)
	if err == nil {
		return record, nil
	}
	fallback, fallbackErr := usage.CaptureFromJSONL(codexHome)
	if fallbackErr != nil {
		return usage.Record{}, fmt.Errorf("app-server: %v; jsonl fallback: %v", err, fallbackErr)
	}
	fallback.Stale = true
	fallback.Error = err.Error()
	return fallback, nil
}

func printList(stdout io.Writer, st store.Store) error {
	accounts, err := st.List()
	if err != nil {
		return err
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	for _, acct := range accounts {
		email := acct.Meta.Email
		if email == "" {
			email = "unknown"
		}
		fmt.Fprintf(stdout, "%s\t%s\n", acct.Name, email)
	}
	return nil
}

func printStatus(stdout io.Writer, st store.Store) error {
	current, _ := st.Current()
	activeName, activeOK, activeErr := st.ActiveProfile()
	activeLabel := valueOr(current, "unknown")
	if activeErr == nil && activeOK {
		activeLabel = activeName
	} else if activeErr == nil {
		if meta, err := st.CurrentAuthMetadata(); err == nil {
			activeLabel = "unmanaged (" + valueOr(meta.Email, "unknown") + ")"
		}
	}
	accounts, err := st.List()
	if err != nil {
		return err
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	fmt.Fprintf(stdout, "Current: %s\n\n", activeLabel)
	for _, acct := range accounts {
		marker := " "
		if acct.Name == activeLabel {
			marker = "*"
		}
		email := valueOr(acct.Meta.Email, "unknown")
		if acct.Usage == nil {
			fmt.Fprintf(stdout, "%s %-16s %-28s usage unknown\n", marker, acct.Name, email)
			continue
		}
		fmt.Fprintf(stdout, "%s %-16s %-28s 5h %3d%% left reset %-18s weekly %3d%% left reset %-18s %s\n",
			marker,
			acct.Name,
			email,
			acct.Usage.FiveHour.LeftPercent,
			formatReset(acct.Usage.FiveHour.ResetsAt),
			acct.Usage.Weekly.LeftPercent,
			formatReset(acct.Usage.Weekly.ResetsAt),
			staleLabel(*acct.Usage),
		)
	}
	return nil
}

func formatReset(ts *int64) string {
	if ts == nil {
		return "unknown"
	}
	return time.Unix(*ts, 0).Format("02 Jan 15:04")
}

func staleLabel(record usage.Record) string {
	age := time.Since(record.CapturedAt).Round(time.Minute)
	parts := []string{record.Source, "seen " + age.String() + " ago"}
	if record.Stale {
		parts = append(parts, "stale")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
  codex-switch status
  codex-switch list
  codex-switch current

Environment:
  CODEX_HOME          defaults to ~/.codex
  CODEX_SWITCH_HOME  defaults to ~/.codex-auth-switcher`)
}
