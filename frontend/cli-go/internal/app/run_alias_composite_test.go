package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
)

func TestCompositePrepareAliasRunRaw(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRun client.RunRequest
	server := newCompositeAliasServer(t, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "prepare", "chinook", "run:psql", "--", "-c", "select 1"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRun.InstanceRef != "inst" {
		t.Fatalf("instance_ref = %q, want inst", gotRun.InstanceRef)
	}
	if gotRun.Kind != "psql" {
		t.Fatalf("kind = %q, want psql", gotRun.Kind)
	}
}

func TestCompositePrepareRawRunAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRun client.RunRequest
	server := newCompositeAliasServer(t, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "prepare:psql", "--image", "image", "--", "-c", "select 1", "run", "smoke"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRun.InstanceRef != "inst" {
		t.Fatalf("instance_ref = %q, want inst", gotRun.InstanceRef)
	}
	if gotRun.Kind != "psql" {
		t.Fatalf("kind = %q, want psql", gotRun.Kind)
	}
	if len(gotRun.Steps) != 1 || strings.Join(gotRun.Steps[0].Args, " ") != "-c select 1" {
		t.Fatalf("unexpected run steps: %+v", gotRun.Steps)
	}
}

func TestCompositePrepareAliasRunAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRun client.RunRequest
	server := newCompositeAliasServer(t, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: pgbench\nargs:\n  - -c\n  - 10\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "prepare", "chinook", "run", "smoke"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRun.InstanceRef != "inst" {
		t.Fatalf("instance_ref = %q, want inst", gotRun.InstanceRef)
	}
	if gotRun.Kind != "pgbench" {
		t.Fatalf("kind = %q, want pgbench", gotRun.Kind)
	}
	if got := strings.Join(gotRun.Args, " "); got != "-c 10" {
		t.Fatalf("args = %q, want %q", got, "-c 10")
	}
}

func TestCompositePrepareAliasRunMissingStillCleansPreparedInstance(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	var cleanupCalls int32
	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() { return func() {} }
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		atomic.AddInt32(&cleanupCalls, 1)
		if instanceID != "inst" {
			t.Fatalf("cleanup instanceID = %q, want inst", instanceID)
		}
		return client.DeleteResult{Outcome: "deleted"}, http.StatusOK, nil
	}

	server := newCompositeAliasServer(t, nil)
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "prepare", "chinook", "run", "missing"})
	if err == nil || !strings.Contains(err.Error(), "run alias file not found") {
		t.Fatalf("expected missing run alias error, got %v", err)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestCompositePrepareAliasRunHelpStillCleansPreparedInstance(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	var cleanupCalls int32
	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() { return func() {} }
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		atomic.AddInt32(&cleanupCalls, 1)
		if instanceID != "inst" {
			t.Fatalf("cleanup instanceID = %q, want inst", instanceID)
		}
		return client.DeleteResult{Outcome: "deleted"}, http.StatusOK, nil
	}

	server := newCompositeAliasServer(t, nil)
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "chinook", "run", "--help"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "sqlrs run") {
		t.Fatalf("expected run usage output, got %q", out)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestCompositePrepareRawRunRawStillWorks(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRun client.RunRequest
	server := newCompositeAliasServer(t, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	if err := Run([]string{"--mode=remote", "--endpoint", server.URL, "prepare:psql", "--image", "image", "--", "-c", "select 1", "run:pgbench", "--", "-c", "10"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRun.InstanceRef != "inst" {
		t.Fatalf("instance_ref = %q, want inst", gotRun.InstanceRef)
	}
	if gotRun.Kind != "pgbench" {
		t.Fatalf("kind = %q, want pgbench", gotRun.Kind)
	}
}

func TestCompositeRunAliasRejectsInstanceAfterPrepare(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := newCompositeAliasServer(t, nil)
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "prepare", "chinook", "run", "smoke", "--instance", "staging"})
	if err == nil || !strings.Contains(err.Error(), "preceding prepare") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestCompositePrepareAliasNoWatchSkipsRunAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var runCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			atomic.AddInt32(&runCalls, 1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
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

	err = Run([]string{"--workspace", workspace, "prepare", "--no-watch", "chinook", "run", "smoke"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "JOB_ID=job-1") || !strings.Contains(out, "RUN_SKIPPED=prepare_not_watched") {
		t.Fatalf("unexpected output: %s", out)
	}
	if got := atomic.LoadInt32(&runCalls); got != 0 {
		t.Fatalf("expected run to be skipped, got %d calls", got)
	}
}

func TestCompositePrepareAliasDetachSkipsRunAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -c\n  - select 1\n")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(context.Context, cli.PrepareOptions) (client.PrepareJobResult, error) {
		return client.PrepareJobResult{}, &cli.PrepareDetachedError{JobID: "job-detached"}
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

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

	err = Run([]string{"--workspace", workspace, "prepare", "chinook", "run", "smoke"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "JOB_ID=job-detached") || !strings.Contains(out, "RUN_SKIPPED=prepare_detached") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func newCompositeAliasServer(t *testing.T, onRun func(client.RunRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"status\",\"ts\":\"2026-01-24T00:00:00Z\",\"status\":\"succeeded\"}\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			if onRun != nil {
				var req client.RunRequest
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &req)
				onRun(req)
			}
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"exit\",\"exit_code\":0}\n")
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dry_run":false,"outcome":"deleted","root":{"kind":"instance","id":"inst"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
