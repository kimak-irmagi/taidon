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
	if !opts.IncludeNames || !opts.IncludeInstances || opts.IncludeStates {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
}

func TestParseLsAll(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"--all"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if !opts.IncludeNames || !opts.IncludeInstances || !opts.IncludeStates {
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
	if !opts.Long {
		t.Fatalf("expected long ids enabled")
	}
}
