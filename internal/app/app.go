package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/auth"
	"github.com/curserio/codex-auth-switcher/internal/store"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

type captureFunc func(context.Context) (usage.Record, error)

type runOptions struct {
	capture captureFunc
	now     func() time.Time
}

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
		if record, err := opts.capture(context.Background()); err == nil {
			_ = st.SaveUsage(args[1], record)
			_ = st.SaveCurrentAuthToProfile(args[1])
			fmt.Fprintf(stderr, "captured %s usage from %s\n", args[1], record.Source)
		} else {
			fmt.Fprintf(stderr, "warning: could not capture usage for %s: %v\n", args[1], err)
		}
	case "current":
		name, err := st.Current()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, name)
	case "list":
		return printList(stdout, st)
	case "status":
		if len(args) == 2 && args[1] == "--json" {
			return printStatusJSON(stdout, st, opts.now())
		}
		if len(args) != 1 {
			return errors.New("usage: codex-switch status [--json]")
		}
		return printStatus(stdout, st, opts.now())
	case "capture":
		name, err := activeProfileName(st)
		if err != nil {
			return err
		}
		record, err := opts.capture(context.Background())
		if err != nil {
			return err
		}
		if err := st.SaveUsage(name, record); err != nil {
			return err
		}
		if err := st.SaveCurrentAuthToProfile(name); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "captured usage for %s from %s\n", name, record.Source)
	case "prepare-login", "prepare":
		if len(args) != 1 {
			return errors.New("usage: codex-switch prepare-login")
		}
		if current, active, err := st.ActiveProfile(); err == nil && active {
			if record, err := opts.capture(context.Background()); err == nil {
				_ = st.SaveUsage(current, record)
				fmt.Fprintf(stderr, "captured %s usage from %s\n", current, record.Source)
			} else {
				fmt.Fprintf(stderr, "warning: could not capture usage for %s: %v\n", current, err)
			}
			if err := st.SaveCurrentAuthToProfile(current); err != nil {
				return err
			}
			fmt.Fprintf(stderr, "saved current auth to %s\n", current)
		}
		if err := st.PrepareLogin(); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "prepared clean Codex auth state for login")
		fmt.Fprintln(stdout, "log in with Codex, then run: codex-switch add <name>")
	case "doctor":
		if len(args) != 1 {
			return errors.New("usage: codex-switch doctor")
		}
		return runDoctor(stdout, st, opts.capture)
	case "use":
		if len(args) != 2 {
			return errors.New("usage: codex-switch use <name>")
		}
		return useAccount(stdout, stderr, st, args[1], opts.capture)
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

func useAccount(stdout, stderr io.Writer, st store.Store, target string, capture captureFunc) error {
	current, active, currentErr := st.ActiveProfile()
	if currentErr == nil && active {
		if record, err := capture(context.Background()); err == nil {
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
	if record, err := capture(context.Background()); err == nil {
		_ = st.SaveUsage(target, record)
		_ = st.SaveCurrentAuthToProfile(target)
		fmt.Fprintf(stderr, "captured %s usage from %s\n", target, record.Source)
	} else {
		if currentErr == nil && active && current != target {
			if rollbackErr := st.SwitchTo(current); rollbackErr != nil {
				return fmt.Errorf("switched to %s, but validation failed: %v; rollback to %s failed: %w", target, err, current, rollbackErr)
			}
			return fmt.Errorf("target %s auth validation failed: %v; restored %s", target, err, current)
		}
		return fmt.Errorf("target %s auth validation failed: %w", target, err)
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

type doctorSeverity int

const (
	doctorOK doctorSeverity = iota
	doctorWarn
	doctorFail
)

type doctorCheck struct {
	Severity doctorSeverity
	Message  string
}

func runDoctor(stdout io.Writer, st store.Store, capture captureFunc) error {
	checks := doctorChecks(st, capture)
	hasFailure := false
	for _, check := range checks {
		if check.Severity == doctorFail {
			hasFailure = true
		}
		fmt.Fprintf(stdout, "%s %s\n", doctorSeverityLabel(check.Severity), check.Message)
	}
	if hasFailure {
		return errors.New("doctor found failures")
	}
	return nil
}

func doctorChecks(st store.Store, capture captureFunc) []doctorCheck {
	var checks []doctorCheck
	accounts, err := st.List()
	if err != nil {
		return []doctorCheck{{Severity: doctorFail, Message: "read switch store: " + err.Error()}}
	}
	checks = append(checks, doctorCheck{Severity: doctorOK, Message: fmt.Sprintf("read switch store (%d profiles)", len(accounts))})

	seen := make(map[string]string)
	for _, acct := range accounts {
		if acct.Meta.Email == "" {
			checks = append(checks, doctorCheck{Severity: doctorFail, Message: fmt.Sprintf("profile %s auth is unreadable", acct.Name)})
			continue
		}
		checks = append(checks, doctorCheck{Severity: doctorOK, Message: fmt.Sprintf("profile %s auth parses as %s", acct.Name, acct.Meta.Email)})
		key := identityKey(acct.Meta)
		if prev, ok := seen[key]; ok {
			checks = append(checks, doctorCheck{Severity: doctorFail, Message: fmt.Sprintf("profiles %s and %s have the same identity", prev, acct.Name)})
		} else {
			seen[key] = acct.Name
		}
	}

	activeName, activeOK, activeErr := st.ActiveProfile()
	if activeErr != nil {
		if errors.Is(activeErr, os.ErrNotExist) {
			checks = append(checks, doctorCheck{Severity: doctorWarn, Message: "no active Codex auth found"})
		} else {
			checks = append(checks, doctorCheck{Severity: doctorFail, Message: "read active Codex auth: " + activeErr.Error()})
		}
		return checks
	}
	if !activeOK {
		if meta, err := st.CurrentAuthMetadata(); err == nil {
			checks = append(checks, doctorCheck{Severity: doctorWarn, Message: fmt.Sprintf("active Codex auth is unmanaged (%s)", valueOr(meta.Email, "unknown"))})
		} else {
			checks = append(checks, doctorCheck{Severity: doctorFail, Message: "read active Codex auth metadata: " + err.Error()})
		}
		return checks
	}

	checks = append(checks, doctorCheck{Severity: doctorOK, Message: "active Codex auth matches profile " + activeName})
	if _, err := capture(context.Background()); err != nil {
		checks = append(checks, doctorCheck{Severity: doctorFail, Message: "live app-server capture failed: " + err.Error()})
	} else {
		checks = append(checks, doctorCheck{Severity: doctorOK, Message: "live app-server capture works for " + activeName})
	}
	return checks
}

func doctorSeverityLabel(severity doctorSeverity) string {
	switch severity {
	case doctorOK:
		return "ok"
	case doctorWarn:
		return "warn"
	default:
		return "fail"
	}
}

func identityKey(meta auth.Metadata) string {
	if meta.Subject != "" {
		return "sub:" + meta.Subject
	}
	if meta.AccountID != "" {
		return "acct:" + meta.AccountID
	}
	return "email:" + meta.Email
}

func capture(ctx context.Context) (usage.Record, error) {
	record, err := usage.CaptureFromAppServer(ctx)
	if err == nil {
		return record, nil
	}
	return usage.Record{}, fmt.Errorf("app-server: %v", err)
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

func printStatus(stdout io.Writer, st store.Store, now time.Time) error {
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
		fmt.Fprintf(stdout, "%s %-16s %-28s 5h %3d%% left reset %-31s weekly %3d%% left reset %-31s %s\n",
			marker,
			acct.Name,
			email,
			acct.Usage.FiveHour.LeftPercent,
			formatResetAt(acct.Usage.FiveHour.ResetsAt, now),
			acct.Usage.Weekly.LeftPercent,
			formatResetAt(acct.Usage.Weekly.ResetsAt, now),
			staleLabel(*acct.Usage),
		)
	}
	return nil
}

type statusJSON struct {
	Current  string              `json:"current"`
	Accounts []statusAccountJSON `json:"accounts"`
}

type statusAccountJSON struct {
	Name   string           `json:"name"`
	Email  string           `json:"email"`
	Active bool             `json:"active"`
	Usage  *statusUsageJSON `json:"usage,omitempty"`
}

type statusUsageJSON struct {
	CapturedAt time.Time        `json:"captured_at"`
	Source     string           `json:"source"`
	PlanType   string           `json:"plan_type,omitempty"`
	FiveHour   statusWindowJSON `json:"five_hour"`
	Weekly     statusWindowJSON `json:"weekly"`
	Stale      bool             `json:"stale,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type statusWindowJSON struct {
	UsedPercent    int    `json:"used_percent"`
	LeftPercent    int    `json:"left_percent"`
	ResetsAt       *int64 `json:"resets_at,omitempty"`
	ResetInSeconds *int64 `json:"reset_in_seconds,omitempty"`
}

func printStatusJSON(stdout io.Writer, st store.Store, now time.Time) error {
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
	out := statusJSON{Current: activeLabel}
	for _, acct := range accounts {
		item := statusAccountJSON{
			Name:   acct.Name,
			Email:  valueOr(acct.Meta.Email, "unknown"),
			Active: acct.Name == activeLabel,
		}
		if acct.Usage != nil {
			item.Usage = usageToStatusJSON(*acct.Usage, now)
		}
		out.Accounts = append(out.Accounts, item)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func usageToStatusJSON(record usage.Record, now time.Time) *statusUsageJSON {
	return &statusUsageJSON{
		CapturedAt: record.CapturedAt,
		Source:     record.Source,
		PlanType:   record.PlanType,
		FiveHour:   windowToStatusJSON(record.FiveHour, now),
		Weekly:     windowToStatusJSON(record.Weekly, now),
		Stale:      record.Stale,
		Error:      record.Error,
	}
}

func windowToStatusJSON(window usage.Window, now time.Time) statusWindowJSON {
	out := statusWindowJSON{
		UsedPercent: window.UsedPercent,
		LeftPercent: window.LeftPercent,
		ResetsAt:    window.ResetsAt,
	}
	if window.ResetsAt != nil {
		remaining := time.Unix(*window.ResetsAt, 0).Sub(now)
		seconds := int64(0)
		if remaining > 0 {
			seconds = int64(remaining.Seconds())
		}
		out.ResetInSeconds = &seconds
	}
	return out
}

func formatReset(ts *int64) string {
	return formatResetAt(ts, time.Now())
}

func formatResetAt(ts *int64, now time.Time) string {
	if ts == nil {
		return "unknown"
	}
	reset := time.Unix(*ts, 0)
	return reset.Format("02 Jan 15:04") + " (" + formatDurationUntil(reset, now) + ")"
}

func formatDurationUntil(target, now time.Time) string {
	remaining := target.Sub(now).Round(time.Minute)
	if remaining <= 0 {
		return "now"
	}
	minutes := int(remaining.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%dm left", minutes)
	}
	hours := minutes / 60
	minutes = minutes % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%dm left", hours, minutes)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd%dh left", days, hours)
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
  codex-switch prepare-login
  codex-switch doctor
  codex-switch status [--json]
  codex-switch list
  codex-switch current

Environment:
  CODEX_HOME          defaults to ~/.codex
  CODEX_SWITCH_HOME  defaults to ~/.codex-auth-switcher`)
}
