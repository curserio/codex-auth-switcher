package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/curserio/codex-auth-switcher/internal/store"
)

func (a App) runInit(args []string, root string) error {
	if err := a.store.Init(); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "initialized %s\n", root)
	return nil
}

func (a App) runAdd(args []string) error {
	if len(args) != 2 {
		return usageError("add <name>")
	}
	meta, err := a.store.Add(args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "saved %s (%s)\n", args[1], meta.Email)
	if record, err := a.capture(context.Background()); err == nil {
		_ = a.store.SaveUsage(args[1], record)
		_ = a.store.SaveCurrentAuthToProfile(args[1])
		fmt.Fprintf(a.stderr, "captured %s usage from %s\n", args[1], record.Source)
	} else {
		fmt.Fprintf(a.stderr, "warning: could not capture usage for %s: %v\n", args[1], err)
	}
	return nil
}

func (a App) runCurrent(args []string) error {
	name, err := a.store.Current()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, name)
	return nil
}

func (a App) runList(args []string) error {
	return printList(a.stdout, a.store)
}

func (a App) runCapture(args []string) error {
	if len(args) != 1 {
		return usageError("capture")
	}
	name, err := activeProfileName(a.store)
	if err != nil {
		return err
	}
	record, err := a.capture(context.Background())
	if err != nil {
		return err
	}
	if err := a.store.SaveUsage(name, record); err != nil {
		return err
	}
	if err := a.store.SaveCurrentAuthToProfile(name); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "captured usage for %s from %s\n", name, record.Source)
	return nil
}

func (a App) runPrepareLogin(args []string) error {
	if len(args) != 1 {
		return usageError("prepare-login")
	}
	if current, active, err := a.store.ActiveProfile(); err == nil && active {
		if record, err := a.capture(context.Background()); err == nil {
			_ = a.store.SaveUsage(current, record)
			fmt.Fprintf(a.stderr, "captured %s usage from %s\n", current, record.Source)
		} else {
			fmt.Fprintf(a.stderr, "warning: could not capture usage for %s: %v\n", current, err)
		}
		if err := a.store.SaveCurrentAuthToProfile(current); err != nil {
			return err
		}
		fmt.Fprintf(a.stderr, "saved current auth to %s\n", current)
	}
	if err := a.store.PrepareLogin(); err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "prepared clean Codex auth state for login")
	fmt.Fprintln(a.stdout, "log in with Codex, then run: codex-switch add <name>")
	return nil
}

func (a App) runUse(args []string) error {
	if len(args) != 2 {
		return usageError("use <name>")
	}
	return useAccount(a.stdout, a.stderr, a.store, args[1], a.capture)
}

func (a App) runRename(args []string) error {
	if len(args) != 3 {
		return usageError("rename <old> <new>")
	}
	if err := a.store.Rename(args[1], args[2]); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "renamed %s to %s\n", args[1], args[2])
	return nil
}

func (a App) runDelete(args []string) error {
	if len(args) != 2 {
		return usageError("delete <name>")
	}
	if err := a.store.Delete(args[1]); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "deleted %s\n", args[1])
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
