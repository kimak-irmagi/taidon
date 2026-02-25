package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
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

func TestParseRmUnicodeDashHint(t *testing.T) {
	_, _, err := parseRmFlags([]string{"—force", "abc"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(exitErr.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash hint, got %v", exitErr)
	}
	if !strings.Contains(exitErr.Error(), "--force") {
		t.Fatalf("expected normalized suggestion, got %v", exitErr)
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

func TestRunRmBlockedJobReturnsExitCode4(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"img"}]`)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"dry_run":false,"outcome":"blocked","root":{"kind":"job","id":"job-1","blocked":"active_tasks"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	err := runRm(io.Discard, cli.RmOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}, []string{"job-1"}, "json")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 4 {
		t.Fatalf("expected exit code 4, got %v", err)
	}
}

func TestSplitRmArgsHonorsDoubleDash(t *testing.T) {
	flags, positionals := splitRmArgs([]string{"--force", "--", "--not-flag", "abc12345"})
	if len(flags) != 1 || flags[0] != "--force" {
		t.Fatalf("unexpected flags: %+v", flags)
	}
	if len(positionals) != 2 || positionals[0] != "--not-flag" || positionals[1] != "abc12345" {
		t.Fatalf("unexpected positionals: %+v", positionals)
	}
}
