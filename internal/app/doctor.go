package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/curserio/codex-auth-switcher/internal/auth"
	"github.com/curserio/codex-auth-switcher/internal/store"
)

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

func (a App) runDoctorCommand(args []string) error {
	if len(args) != 1 {
		return usageError("doctor")
	}
	return runDoctor(a.stdout, a.store, a.capture)
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
