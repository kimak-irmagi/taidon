package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
)

func TestParseStatusFlagsCache(t *testing.T) {
	opts, showHelp, err := parseStatusFlags([]string{"--cache"})
	if err != nil {
		t.Fatalf("parseStatusFlags: %v", err)
	}
	if showHelp {
		t.Fatalf("expected no help")
	}
	if !opts.CacheDetails {
		t.Fatalf("expected CacheDetails=true")
	}
}

func TestParseStatusFlagsHelp(t *testing.T) {
	_, showHelp, err := parseStatusFlags([]string{"--help"})
	if err != nil {
		t.Fatalf("parseStatusFlags: %v", err)
	}
	if !showHelp {
		t.Fatalf("expected help flag to be recognized")
	}

	_, showHelp, err = parseStatusFlags([]string{"-h"})
	if err != nil {
		t.Fatalf("parseStatusFlags short help: %v", err)
	}
	if !showHelp {
		t.Fatalf("expected short help flag to be recognized")
	}
}

func TestParseStatusFlagsInvalidFlag(t *testing.T) {
	_, _, err := parseStatusFlags([]string{"--bad"})
	if err == nil {
		t.Fatalf("expected invalid flag error")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestRunStatusHelp(t *testing.T) {
	var out strings.Builder
	if err := runStatus(&out, cli.StatusOptions{}, t.TempDir(), "text", []string{"--help"}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	if !strings.Contains(out.String(), "sqlrs status [--cache]") {
		t.Fatalf("expected status usage, got %q", out.String())
	}
}
