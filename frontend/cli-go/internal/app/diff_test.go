package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunDiff_PlanPsql(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	// Left: one file
	if err := writeFile(filepath.Join(left, "a.sql"), "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	// Right: same file, same content
	if err := writeFile(filepath.Join(right, "a.sql"), "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	var out bytes.Buffer
	args := []string{"--from-path", left, "--to-path", right, "plan:psql", "--", "-f", "a.sql"}
	err := runDiff(&out, nil, cwd, args, "human", false)
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected some output")
	}
	if !bytes.Contains(out.Bytes(), []byte("Summary:")) {
		t.Fatalf("expected Summary in output: %s", out.String())
	}
}

func TestRunDiff_UnsupportedWrappedCommand(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	args := []string{"--from-path", dir, "--to-path", dir, "run:psql", "--", "-c", "select 1"}
	err := runDiff(&out, nil, dir, args, "human", false)
	if err == nil {
		t.Fatal("expected error for run:psql")
	}
	var exitErr *ExitError
	if !isExitError(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRunDiff_InvalidScope(t *testing.T) {
	var out bytes.Buffer
	err := runDiff(&out, nil, t.TempDir(), []string{"--from-path", "/left", "plan:psql"}, "human", false)
	if err == nil {
		t.Fatal("expected error for missing --to-path")
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func isExitError(err error, out **ExitError) bool {
	return errors.As(err, out)
}
