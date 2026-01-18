package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

func TestRunPrepareRemoteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunPrepare: %v", err)
	}
	if result.DSN != "dsn" || result.InstanceID != "inst" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunPrepareRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunPrepare(context.Background(), PrepareOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunPrepareLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stateDir := t.TempDir()
	if err := daemon.WriteEngineState(filepath.Join(stateDir, "engine.json"), daemon.EngineState{
		Endpoint:   server.URL,
		AuthToken:  "token",
		InstanceID: "inst",
	}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	result, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "local",
		Endpoint: "",
		StateDir: stateDir,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunPrepare: %v", err)
	}
	if result.DSN != "dsn" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunPrepareFailedWithErrorDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"failed","error":{"message":"boom","details":"details"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "boom: details") {
		t.Fatalf("expected error details, got %v", err)
	}
}

func TestRunPrepareJobIDMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"","status_url":"/v1/prepare-jobs/job-1"}`)
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "prepare job id missing") {
		t.Fatalf("expected job id error, got %v", err)
	}
}

func TestRunPrepareJobNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "prepare job not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRunPrepareSucceededWithoutResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "succeeded without result") {
		t.Fatalf("expected missing result error, got %v", err)
	}
}

func TestRunPrepareFailedWithoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"failed"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "prepare job failed") {
		t.Fatalf("expected failed error, got %v", err)
	}
}

func TestRunPrepareRejectsPlanOnly(t *testing.T) {
	_, err := RunPrepare(context.Background(), PrepareOptions{PlanOnly: true})
	if err == nil || !strings.Contains(err.Error(), "plan-only is not supported") {
		t.Fatalf("expected plan-only error, got %v", err)
	}
}

func TestRunPrepareCreateJobError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPrepareLocalAutostartDisabled(t *testing.T) {
	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:      "local",
		Endpoint:  "",
		StateDir:  t.TempDir(),
		Autostart: false,
		ImageID:   "image",
		PsqlArgs:  []string{"-c", "select 1"},
		Timeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "local engine is not running") {
		t.Fatalf("expected autostart error, got %v", err)
	}
}

func TestRunPlanRemoteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-c select 1","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.PrepareKind != "psql" || result.ImageID != "image" || len(result.Tasks) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunPlanJobIDMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"","status_url":"/v1/prepare-jobs/job-1"}`)
	}))
	defer server.Close()

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "prepare job id missing") {
		t.Fatalf("expected job id error, got %v", err)
	}
}

func TestRunPlanRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunPlan(context.Background(), PrepareOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunPlanNotPlanOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":false}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "not plan-only") {
		t.Fatalf("expected not plan-only error, got %v", err)
	}
}

func TestRunPlanCreateJobError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunPlanNoTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-c select 1","tasks":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "without tasks") {
		t.Fatalf("expected missing tasks error, got %v", err)
	}
}

func TestRunPlanFailedWithMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"failed","error":{"message":"boom"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error message, got %v", err)
	}
}

func TestWaitForPrepareCanceled(t *testing.T) {
	ready := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs/job-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		select {
		case ready <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"job_id":"job-1","status":"running"}`)
	}))
	defer server.Close()

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ready
		cancel()
	}()
	_, err := waitForPrepare(ctx, cli, "job-1")
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestWaitForPrepareRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := waitForPrepare(context.Background(), cli, "job-1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForPrepareTicks(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls == 0 {
			calls++
			io.WriteString(w, `{"job_id":"job-1","status":"running"}`)
			return
		}
		io.WriteString(w, `{"job_id":"job-1","status":"succeeded"}`)
	}))
	defer server.Close()

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	status, err := waitForPrepare(context.Background(), cli, "job-1")
	if err != nil {
		t.Fatalf("waitForPrepare: %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
}
