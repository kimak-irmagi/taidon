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
