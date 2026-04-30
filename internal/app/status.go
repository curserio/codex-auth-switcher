package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"time"

	"github.com/curserio/codex-auth-switcher/internal/store"
	"github.com/curserio/codex-auth-switcher/internal/usage"
)

type statusData struct {
	Current  string
	Accounts []store.Account
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

func (a App) runStatus(args []string) error {
	if len(args) == 2 && args[1] == "--json" {
		return printStatusJSON(a.stdout, a.store, a.now())
	}
	if len(args) != 1 {
		return usageError("status [--json]")
	}
	return printStatus(a.stdout, a.store, a.now())
}

func loadStatusData(st store.Store) (statusData, error) {
	current, currentErr := st.Current()
	if currentErr != nil && !errors.Is(currentErr, fs.ErrNotExist) {
		return statusData{}, fmt.Errorf("read current profile: %w", currentErr)
	}
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
		return statusData{}, err
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	return statusData{Current: activeLabel, Accounts: accounts}, nil
}

func printStatus(stdout io.Writer, st store.Store, now time.Time) error {
	data, err := loadStatusData(st)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Current: %s\n\n", data.Current)
	for _, acct := range data.Accounts {
		marker := " "
		if acct.Name == data.Current {
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

func printStatusJSON(stdout io.Writer, st store.Store, now time.Time) error {
	data, err := loadStatusData(st)
	if err != nil {
		return err
	}
	out := statusJSON{Current: data.Current}
	for _, acct := range data.Accounts {
		item := statusAccountJSON{
			Name:   acct.Name,
			Email:  valueOr(acct.Meta.Email, "unknown"),
			Active: acct.Name == data.Current,
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
