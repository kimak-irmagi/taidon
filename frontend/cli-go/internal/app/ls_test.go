package app

import (
	"errors"
	"testing"
)

func TestParseLsDefaults(t *testing.T) {
	opts, showHelp, err := parseLsFlags(nil)
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp false")
	}
	if !opts.IncludeNames || !opts.IncludeInstances || opts.IncludeStates || opts.IncludeJobs || opts.IncludeTasks {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
}

func TestParseLsAll(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"--all"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if !opts.IncludeNames || !opts.IncludeInstances || !opts.IncludeStates || !opts.IncludeJobs || !opts.IncludeTasks {
		t.Fatalf("expected all selectors enabled, got %+v", opts)
	}
}

func TestParseLsInvalidArgs(t *testing.T) {
	_, _, err := parseLsFlags([]string{"extra"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestParseLsLong(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"--long"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if !opts.LongIDs {
		t.Fatalf("expected long ids flag true")
	}
}

func TestParseLsJobsTasksAliases(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"-j", "-t"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if !opts.IncludeJobs || !opts.IncludeTasks {
		t.Fatalf("expected jobs/tasks selectors enabled, got %+v", opts)
	}
}

func TestParseLsJobFilter(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"--tasks", "--job", "job-1"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if opts.FilterJob != "job-1" {
		t.Fatalf("unexpected job filter: %q", opts.FilterJob)
	}
}
