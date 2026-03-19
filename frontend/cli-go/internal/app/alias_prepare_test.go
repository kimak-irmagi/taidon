package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrepareAliasPathFromStem(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writePrepareAliasFile(t, cwd, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolvePrepareAliasPath(workspace, cwd, "chinook")
	if err != nil {
		t.Fatalf("resolvePrepareAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolvePrepareAliasPathParentRefFromCWD(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples", "chinook")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writePrepareAliasFile(t, workspace, filepath.Join("examples", "chinook.prep.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolvePrepareAliasPath(workspace, cwd, "../chinook")
	if err != nil {
		t.Fatalf("resolvePrepareAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolvePrepareAliasPathExactFileEscape(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writePrepareAliasFile(t, cwd, "chinook.txt", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolvePrepareAliasPath(workspace, cwd, "chinook.txt.")
	if err != nil {
		t.Fatalf("resolvePrepareAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolvePrepareAliasPathRejectsMissingFile(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	_, err := resolvePrepareAliasPath(workspace, cwd, "missing")
	if err == nil || !strings.Contains(err.Error(), "prepare alias file not found") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestResolvePrepareAliasPathRejectsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples", "chinook")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	ref := filepath.ToSlash(filepath.Join("..", "..", "..", "outside.prep.s9s.yaml")) + "."
	_, err := resolvePrepareAliasPath(workspace, cwd, ref)
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestLoadPrepareAliasRequiresKind(t *testing.T) {
	path := writePrepareAliasFile(t, t.TempDir(), "broken.prep.s9s.yaml", "args:\n  - -c\n  - select 1\n")

	_, err := loadPrepareAlias(path)
	if err == nil || !strings.Contains(err.Error(), "prepare alias kind is required") {
		t.Fatalf("expected missing kind error, got %v", err)
	}
}

func TestLoadPrepareAliasRequiresArgs(t *testing.T) {
	path := writePrepareAliasFile(t, t.TempDir(), "broken.prep.s9s.yaml", "kind: psql\n")

	_, err := loadPrepareAlias(path)
	if err == nil || !strings.Contains(err.Error(), "prepare alias args are required") {
		t.Fatalf("expected missing args error, got %v", err)
	}
}

func TestLoadPrepareAliasRejectsUnknownKind(t *testing.T) {
	path := writePrepareAliasFile(t, t.TempDir(), "broken.prep.s9s.yaml", "kind: flyway\nargs:\n  - migrate\n")

	_, err := loadPrepareAlias(path)
	if err == nil || !strings.Contains(err.Error(), "unknown prepare alias kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}

func TestLoadPrepareAliasPreservesArgOrder(t *testing.T) {
	path := writePrepareAliasFile(t, t.TempDir(), "ordered.prep.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - prepare.sql\n  - -v\n  - ON_ERROR_STOP=1\n")

	alias, err := loadPrepareAlias(path)
	if err != nil {
		t.Fatalf("loadPrepareAlias: %v", err)
	}
	got := strings.Join(alias.Args, "|")
	if want := "-f|prepare.sql|-v|ON_ERROR_STOP=1"; got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestRunPrepareMissingAliasRef(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"prepare"})
	if err == nil || !strings.Contains(err.Error(), "missing prepare alias ref") {
		t.Fatalf("expected missing alias ref error, got %v", err)
	}
}

func TestRunPlanMissingAliasRef(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"plan"})
	if err == nil || !strings.Contains(err.Error(), "missing plan alias ref") {
		t.Fatalf("expected missing alias ref error, got %v", err)
	}
}

func TestRunPrepareAliasCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--workspace", workspace, "prepare", "chinook"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "DSN=dsn") {
		t.Fatalf("unexpected output: %s", string(data))
	}
}

func TestRunPrepareAliasNoWatchCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--workspace", workspace, "prepare", "--no-watch", "chinook"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "JOB_ID=job-1") || !strings.Contains(out, "STATUS_URL=/v1/prepare-jobs/job-1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunPrepareAliasRejectsInlineToolArgs(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")

	err := Run([]string{"--workspace", workspace, "prepare", "chinook", "--", "-c", "select 2"})
	if err == nil || !strings.Contains(err.Error(), "prepare aliases do not accept inline tool args") {
		t.Fatalf("expected inline args rejection, got %v", err)
	}
}

func TestRunPlanAliasCommandJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-c select 1","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"},{"task_id":"execute-0","type":"state_execute","input":{"kind":"image","id":"image"},"task_hash":"hash","output_state_id":"state-1","cached":false}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--workspace", workspace, "--output=json", "plan", "chinook"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "\"tasks\"") {
		t.Fatalf("unexpected output: %s", string(data))
	}
}

func TestRunPlanAliasLiquibaseCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"lb","image_id":"image","prepare_args_normalized":"update","tasks":[{"task_id":"plan","type":"plan","planner_kind":"lb"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "liquibase.prep.s9s.yaml", "kind: lb\nimage: image\nargs:\n  - update\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "--output=json", "plan", "liquibase"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRequest == nil {
		t.Fatalf("expected request payload")
	}
	if gotRequest["prepare_kind"] != "lb" {
		t.Fatalf("prepare_kind = %+v, want lb", gotRequest["prepare_kind"])
	}
	args, ok := gotRequest["liquibase_args"].([]any)
	if !ok || len(args) != 1 || args[0] != "update" {
		t.Fatalf("unexpected liquibase args: %+v", gotRequest["liquibase_args"])
	}
}

func TestRunPlanAliasRejectsInlineToolArgs(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")

	err := Run([]string{"--workspace", workspace, "plan", "chinook", "--", "-c", "select 2"})
	if err == nil || !strings.Contains(err.Error(), "plan aliases do not accept inline tool args") {
		t.Fatalf("expected inline args rejection, got %v", err)
	}
}

func TestRunPlanAliasRejectsWatchFlags(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")

	err := Run([]string{"--workspace", workspace, "plan", "--no-watch", "chinook"})
	if err == nil || !strings.Contains(err.Error(), "plan does not support --watch/--no-watch") {
		t.Fatalf("expected watch flag rejection, got %v", err)
	}
}

func writePrepareAliasFile(t *testing.T, workspace, relPath, contents string) string {
	t.Helper()
	path := filepath.Join(workspace, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir alias dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	return path
}

func writeAliasWorkspace(t *testing.T, tempRoot, endpoint string) string {
	t.Helper()
	workspace := filepath.Join(tempRoot, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	projectConfig := bytes.NewBuffer(nil)
	projectConfig.WriteString("defaultProfile: remote\n")
	projectConfig.WriteString("profiles:\n")
	projectConfig.WriteString("  remote:\n")
	projectConfig.WriteString("    mode: remote\n")
	projectConfig.WriteString("    endpoint: ")
	projectConfig.WriteString(endpoint)
	projectConfig.WriteString("\n")
	if err := os.WriteFile(filepath.Join(workspace, ".sqlrs", "config.yaml"), projectConfig.Bytes(), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	return workspace
}
