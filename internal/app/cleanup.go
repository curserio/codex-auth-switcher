package app

import (
	"flag"
	"fmt"
	"io"

	"github.com/curserio/codex-auth-switcher/internal/store"
)

const (
	defaultCleanupDays     = 30
	defaultCleanupKeep     = 10
	defaultCleanupLogLines = 1000
)

func (a App) runCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	apply := fs.Bool("apply", false, "apply cleanup changes")
	days := fs.Int("days", defaultCleanupDays, "delete backups older than this many days")
	keep := fs.Int("keep", defaultCleanupKeep, "always keep this many newest backups")
	logLines := fs.Int("log-lines", defaultCleanupLogLines, "keep this many switch.log lines")
	if err := fs.Parse(args[1:]); err != nil {
		return usageError("cleanup [--apply] [--days N] [--keep N] [--log-lines N]")
	}
	if fs.NArg() != 0 {
		return usageError("cleanup [--apply] [--days N] [--keep N] [--log-lines N]")
	}

	options := store.CleanupOptions{
		Days:     *days,
		Keep:     *keep,
		LogLines: *logLines,
	}
	plan, err := a.store.PlanCleanup(options, a.now())
	if err != nil {
		return err
	}
	if !*apply {
		printCleanupPlan(a.stdout, plan, false)
		return nil
	}
	if err := a.store.ApplyCleanup(plan); err != nil {
		return err
	}
	printCleanupPlan(a.stdout, plan, true)
	return nil
}

func printCleanupPlan(stdout io.Writer, plan store.CleanupPlan, applied bool) {
	if !applied {
		fmt.Fprintln(stdout, "cleanup dry-run")
	}

	changed := false
	for _, backup := range plan.Backups {
		changed = true
		if applied {
			fmt.Fprintf(stdout, "deleted backup %s\n", backup.Name)
		} else {
			fmt.Fprintf(stdout, "would delete backup %s\n", backup.Name)
		}
	}
	if plan.Log.TrimNeeded {
		changed = true
		if applied {
			fmt.Fprintf(stdout, "trimmed switch.log from %d to %d lines\n", plan.Log.LineCount, plan.Log.KeepLines)
		} else {
			fmt.Fprintf(stdout, "would trim switch.log from %d to %d lines\n", plan.Log.LineCount, plan.Log.KeepLines)
		}
	}
	if !changed {
		fmt.Fprintln(stdout, "nothing to clean")
	}
	if !applied {
		fmt.Fprintln(stdout, "run codex-switch cleanup --apply to make changes")
		return
	}
	fmt.Fprintln(stdout, "cleanup complete")
}
