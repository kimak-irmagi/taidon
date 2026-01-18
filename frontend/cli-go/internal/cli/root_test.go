package cli

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	opts, cmd, err := ParseArgs([]string{
		"--profile", "p",
		"--endpoint", "http://example",
		"--mode", "remote",
		"--output", "json",
		"--timeout", "5s",
		"-v",
		"status",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}

	if cmd.Name != "status" {
		t.Fatalf("expected command status, got %q", cmd.Name)
	}
	if opts.Profile != "p" {
		t.Fatalf("expected profile p, got %q", opts.Profile)
	}
	if opts.Endpoint != "http://example" {
		t.Fatalf("expected endpoint override, got %q", opts.Endpoint)
	}
	if opts.Mode != "remote" {
		t.Fatalf("expected mode remote, got %q", opts.Mode)
	}
	if opts.Output != "json" {
		t.Fatalf("expected output json, got %q", opts.Output)
	}
	if opts.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %v", opts.Timeout)
	}
	if !opts.Verbose {
		t.Fatalf("expected verbose true")
	}
}

func TestParseArgsHelp(t *testing.T) {
	_, _, err := ParseArgs([]string{"--help"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestParseArgsMissingCommand(t *testing.T) {
	_, _, err := ParseArgs([]string{"--profile", "p"})
	if err == nil {
		t.Fatalf("expected missing command error")
	}
}

func TestParseArgsInvalidTimeout(t *testing.T) {
	_, _, err := ParseArgs([]string{"--timeout", "bad", "status"})
	if err == nil || !strings.Contains(err.Error(), "invalid timeout") {
		t.Fatalf("expected invalid timeout error, got %v", err)
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	_, _, err := ParseArgs([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
