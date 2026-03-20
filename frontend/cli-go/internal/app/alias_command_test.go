package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAliasLsHumanOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "alias", "ls"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "TYPE") || !strings.Contains(out, "chinook") || !strings.Contains(out, "smoke") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunAliasLsJSONOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "--output=json", "alias", "ls", "--prepare"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(entries) != 1 || entries[0]["type"] != "prepare" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunAliasCheckHumanOutputScanMode(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "good.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "bad.run.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - missing.sql\n")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "alias", "check"})
	})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(out, "INVALID") || !strings.Contains(out, "checked=2") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunAliasCheckJSONOutputSingleAliasMode(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "--output=json", "alias", "check", "--run", "smoke"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if report["checked"] != float64(1) || report["valid"] != float64(1) || report["invalid"] != float64(0) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunAliasCheckExitCodeTwoOnUsageOrSelectionError(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "alias", "check", "--from", "workspace", "smoke"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRunAliasLsFromWorkspaceAllowsCallerOutsideWorkspace(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	caller := filepath.Join(temp, "caller")
	if err := os.MkdirAll(caller, 0o700); err != nil {
		t.Fatalf("mkdir caller: %v", err)
	}
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, caller)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "alias", "ls", "--from", "workspace"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "chinook.prep.s9s.yaml") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunAliasCheckFromWorkspaceAllowsCallerOutsideWorkspace(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	caller := filepath.Join(temp, "caller")
	if err := os.MkdirAll(caller, 0o700); err != nil {
		t.Fatalf("mkdir caller: %v", err)
	}
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, caller)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "alias", "check", "--from", "workspace"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "checked=1 valid=1 invalid=0") {
		t.Fatalf("unexpected output: %q", out)
	}
}
