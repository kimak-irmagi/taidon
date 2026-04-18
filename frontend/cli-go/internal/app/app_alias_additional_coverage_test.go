package app

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureRunStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() {
		_ = r.Close()
	}()
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	runErr := fn()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	captured := string(data)
	t.Cleanup(func() {
		if !t.Failed() || captured == "" {
			return
		}
		fmt.Fprintf(oldStdout, "\n[%s] captured stdout:\n%s\n", t.Name(), captured)
	})
	return captured, runErr
}

func TestRunAliasModeHelpOutputsUsage(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "prepare", args: []string{"prepare", "--help"}, want: "sqlrs prepare"},
		{name: "plan", args: []string{"plan", "--help"}, want: "sqlrs plan"},
		{name: "run", args: []string{"run", "--help"}, want: "sqlrs run"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureRunStdout(t, func() error { return Run(tc.args) })
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("expected usage output containing %q, got %q", tc.want, out)
			}
		})
	}
}

func TestRunAliasModeReportsPreparePathAndLoadErrors(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "prepare", "missing"})
	if err == nil || !strings.Contains(err.Error(), "prepare alias file not found") {
		t.Fatalf("expected missing alias file error, got %v", err)
	}

	writePrepareAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: [\n")
	err = Run([]string{"--workspace", workspace, "prepare", "broken"})
	if err == nil || !strings.Contains(err.Error(), "read prepare alias") {
		t.Fatalf("expected invalid alias error, got %v", err)
	}
}

func TestRunAliasModeReportsPlanPathAndLoadErrors(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "plan", "missing"})
	if err == nil || !strings.Contains(err.Error(), "prepare alias file not found") {
		t.Fatalf("expected missing alias file error, got %v", err)
	}

	writePrepareAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: [\n")
	err = Run([]string{"--workspace", workspace, "plan", "broken"})
	if err == nil || !strings.Contains(err.Error(), "read prepare alias") {
		t.Fatalf("expected invalid alias error, got %v", err)
	}
}

func TestRunAliasModeReportsRunPathAndLoadErrors(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "run", "missing", "--instance", "dev"})
	if err == nil || !strings.Contains(err.Error(), "run alias file not found") {
		t.Fatalf("expected missing alias file error, got %v", err)
	}

	writeRunAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: [\n")
	err = Run([]string{"--workspace", workspace, "run", "broken", "--instance", "dev"})
	if err == nil || !strings.Contains(err.Error(), "read run alias") {
		t.Fatalf("expected invalid alias error, got %v", err)
	}
}

func TestRunAliasModeReturnsExitCodeTwoForInvalidPrepareAndPlanAliasDefinitions(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	writePrepareAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: flyway\nargs:\n  - migrate\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "prepare",
			args: []string{"--workspace", workspace, "prepare", "broken"},
			want: "unknown prepare alias kind",
		},
		{
			name: "plan",
			args: []string{"--workspace", workspace, "plan", "broken"},
			want: "unknown prepare alias kind",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args)
			var exitErr *ExitError
			if !errors.As(err, &exitErr) || exitErr.Code != 2 {
				t.Fatalf("expected exit code 2, got %v", err)
			}
			if !strings.Contains(exitErr.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, exitErr)
			}
		})
	}
}

func TestRunAliasModeReturnsExitCodeTwoForInvalidRunAliasDefinitions(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	writeRunAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: psql\nimage: postgres:17\nargs:\n  - -c\n  - select 1\n")

	err := Run([]string{"--workspace", workspace, "run", "broken", "--instance", "dev"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
	if !strings.Contains(exitErr.Error(), "run alias does not support image") {
		t.Fatalf("expected invalid run alias error, got %v", exitErr)
	}
}

func TestRunAliasModeReturnsExitCodeTwoForMalformedAliasYAML(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	writePrepareAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: [\n")
	writeRunAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: [\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "prepare",
			args: []string{"--workspace", workspace, "prepare", "broken"},
			want: "read prepare alias",
		},
		{
			name: "plan",
			args: []string{"--workspace", workspace, "plan", "broken"},
			want: "read prepare alias",
		},
		{
			name: "run",
			args: []string{"--workspace", workspace, "run", "broken", "--instance", "dev"},
			want: "read run alias",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args)
			var exitErr *ExitError
			if !errors.As(err, &exitErr) || exitErr.Code != 2 {
				t.Fatalf("expected exit code 2, got %v", err)
			}
			if !strings.Contains(exitErr.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, exitErr)
			}
		})
	}
}

func TestRunAliasModeReturnsExitCodeTwoForMalformedExactAliasFiles(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	writePrepareAliasFile(t, workspace, "broken.txt", "kind: [\n")
	writeRunAliasFile(t, workspace, "broken.txt", "kind: [\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "prepare exact file",
			args: []string{"--workspace", workspace, "prepare", "broken.txt."},
			want: "read prepare alias",
		},
		{
			name: "plan exact file",
			args: []string{"--workspace", workspace, "plan", "broken.txt."},
			want: "read prepare alias",
		},
		{
			name: "run exact file",
			args: []string{"--workspace", workspace, "run", "broken.txt.", "--instance", "dev"},
			want: "read run alias",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args)
			var exitErr *ExitError
			if !errors.As(err, &exitErr) || exitErr.Code != 2 {
				t.Fatalf("expected exit code 2, got %v", err)
			}
			if !strings.Contains(exitErr.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, exitErr)
			}
		})
	}
}

func TestRunPrepareLiquibaseAliasCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-lb","status_url":"/v1/prepare-jobs/job-lb","events_url":"/v1/prepare-jobs/job-lb/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-lb/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"status\",\"ts\":\"2026-01-24T00:00:00Z\",\"status\":\"succeeded\"}\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-lb":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-lb","status":"succeeded","result":{"dsn":"lb-dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"lb","prepare_args_normalized":"update --changelog-file changelog.xml"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	if err := os.WriteFile(filepath.Join(workspace, "changelog.xml"), []byte("<databaseChangeLog/>"), 0o600); err != nil {
		t.Fatalf("write changelog: %v", err)
	}
	writePrepareAliasFile(t, workspace, "liquibase.prep.s9s.yaml", "kind: lb\nimage: image\nargs:\n  - update\n  - --changelog-file\n  - changelog.xml\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { restoreWorkingDirForTest(cwd) })

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "liquibase"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "DSN=lb-dsn") {
		t.Fatalf("unexpected output: %q", out)
	}
}
