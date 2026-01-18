package app

import (
	"errors"
	"testing"
)

func TestParseRmMissingPrefix(t *testing.T) {
	_, _, err := parseRmFlags([]string{})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestParseRmHelp(t *testing.T) {
	_, showHelp, err := parseRmFlags([]string{"--help"})
	if err != nil || !showHelp {
		t.Fatalf("expected help, err=%v help=%v", err, showHelp)
	}
}

func TestParseRmTooManyArgs(t *testing.T) {
	_, _, err := parseRmFlags([]string{"abc", "def"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
}

func TestParseRmEmptyPrefix(t *testing.T) {
	_, _, err := parseRmFlags([]string{"  "})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
}

func TestParseRmUnknownFlag(t *testing.T) {
	_, _, err := parseRmFlags([]string{"--unknown"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
}

func TestParseRmFlags(t *testing.T) {
	opts, _, err := parseRmFlags([]string{"-r", "-f", "--dry-run", "abc12345"})
	if err != nil {
		t.Fatalf("parseRmFlags: %v", err)
	}
	if !opts.Recurse || !opts.Force || !opts.DryRun {
		t.Fatalf("unexpected rm flags: %+v", opts)
	}
	if opts.IDPrefix != "abc12345" {
		t.Fatalf("unexpected id prefix: %q", opts.IDPrefix)
	}
}

func TestParseRmFlagsAfterPrefix(t *testing.T) {
	opts, _, err := parseRmFlags([]string{"abc12345", "--force", "--recurse"})
	if err != nil {
		t.Fatalf("parseRmFlags: %v", err)
	}
	if !opts.Recurse || !opts.Force {
		t.Fatalf("unexpected rm flags: %+v", opts)
	}
	if opts.IDPrefix != "abc12345" {
		t.Fatalf("unexpected id prefix: %q", opts.IDPrefix)
	}
}
