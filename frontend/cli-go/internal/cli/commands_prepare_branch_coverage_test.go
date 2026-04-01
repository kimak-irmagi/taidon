package cli

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/client"
	"golang.org/x/term"
)

func TestRunWatchResolvedPrefixSecondLookupError(t *testing.T) {
	const prefix = "deadbeef"
	const fullID = "deadbeefcafebabe"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/"+prefix:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs" && r.URL.Query().Get("job") == prefix:
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"job_id":"`+fullID+`","status":"running"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/"+fullID:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	_, err := RunWatch(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}, prefix)
	if err == nil {
		t.Fatalf("expected second get status error after prefix resolution")
	}
}

func TestRunWatchRunningStreamsToCompletion(t *testing.T) {
	var statusCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			call := atomic.AddInt32(&statusCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				io.WriteString(w, `{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"image"}`)
				return
			}
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"running"}`+"\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	status, err := RunWatch(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}, "job-1")
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
}

func TestResolvePrepareJobByPrefixCoverageBranches(t *testing.T) {
	t.Run("empty prefix", func(t *testing.T) {
		id, err := resolvePrepareJobByPrefix(context.Background(), client.New("http://example.com", client.Options{}), "   ")
		if err != nil {
			t.Fatalf("resolvePrepareJobByPrefix: %v", err)
		}
		if id != "" {
			t.Fatalf("expected empty id, got %q", id)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare-jobs" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		}))
		t.Cleanup(server.Close)

		id, err := resolvePrepareJobByPrefix(context.Background(), client.New(server.URL, client.Options{Timeout: time.Second}), "abc")
		if err != nil {
			t.Fatalf("resolvePrepareJobByPrefix: %v", err)
		}
		if id != "" {
			t.Fatalf("expected empty id, got %q", id)
		}
	})

	t.Run("list error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)

		_, err := resolvePrepareJobByPrefix(context.Background(), client.New(server.URL, client.Options{Timeout: time.Second}), "abc")
		if err == nil {
			t.Fatalf("expected list error")
		}
	})
}

func TestHandlePrepareControlActionContinue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare-jobs/job-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"image"}`)
	}))
	t.Cleanup(server.Close)

	prevPrompt := promptPrepareControl
	promptPrepareControl = func(writer io.Writer, interrupts <-chan os.Signal) (prepareControlAction, error) {
		return prepareControlContinue, nil
	}
	t.Cleanup(func() { promptPrepareControl = prevPrompt })

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	tracker := newPrepareProgress(io.Discard, false)
	defer tracker.Close()

	status, err := handlePrepareControlAction(context.Background(), cliClient, "job-1", tracker, make(chan os.Signal))
	if err != nil {
		t.Fatalf("handlePrepareControlAction: %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status for continue action, got %+v", status)
	}
}

func TestHandlePrepareControlActionCancelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"image"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs/job-1/cancel":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	prevPrompt := promptPrepareControl
	promptPrepareControl = func(writer io.Writer, interrupts <-chan os.Signal) (prepareControlAction, error) {
		return prepareControlStop, nil
	}
	t.Cleanup(func() { promptPrepareControl = prevPrompt })

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	tracker := newPrepareProgress(io.Discard, false)
	defer tracker.Close()

	_, err := handlePrepareControlAction(context.Background(), cliClient, "job-1", tracker, make(chan os.Signal))
	if err == nil {
		t.Fatalf("expected cancel error")
	}
}

func TestDefaultCanUsePrepareControlPromptNonFileWriter(t *testing.T) {
	if defaultCanUsePrepareControlPrompt(&strings.Builder{}) {
		t.Fatalf("expected false for non-file writer")
	}
}

func TestEnableRawStdinCoverageBranches(t *testing.T) {
	prevIsTerminal := isTerminal
	prevMakeRaw := makeRawTerminal
	prevRestore := restoreTerminalState
	t.Cleanup(func() {
		isTerminal = prevIsTerminal
		makeRawTerminal = prevMakeRaw
		restoreTerminalState = prevRestore
	})

	t.Run("make raw error", func(t *testing.T) {
		isTerminal = func(_ int) bool { return true }
		makeRawTerminal = func(_ int) (*term.State, error) {
			return nil, errors.New("make raw failed")
		}
		calledRestore := false
		restoreTerminalState = func(_ int, _ *term.State) error {
			calledRestore = true
			return nil
		}

		restore := enableRawStdin()
		restore()
		if calledRestore {
			t.Fatalf("restore should not be called when makeRawTerminal fails")
		}
	})

	t.Run("restore on success", func(t *testing.T) {
		isTerminal = func(_ int) bool { return true }
		state := &term.State{}
		makeRawTerminal = func(_ int) (*term.State, error) {
			return state, nil
		}
		calledRestore := false
		restoreTerminalState = func(_ int, oldState *term.State) error {
			calledRestore = oldState == state
			return nil
		}

		restore := enableRawStdin()
		restore()
		if !calledRestore {
			t.Fatalf("expected restoreTerminalState to be called with returned state")
		}
	})
}

func TestPromptPrepareControlDefaultCoverageBranches(t *testing.T) {
	withTestStdin(t, "", func() {
		action, err := promptPrepareControlDefault(io.Discard, make(chan os.Signal, 1))
		if err != nil {
			t.Fatalf("promptPrepareControlDefault: %v", err)
		}
		if action != prepareControlContinue {
			t.Fatalf("expected continue for EOF, got %v", action)
		}
	})

	withTestStdin(t, "sN", func() {
		action, err := promptPrepareControlDefault(io.Discard, make(chan os.Signal, 1))
		if err != nil {
			t.Fatalf("promptPrepareControlDefault: %v", err)
		}
		if action != prepareControlContinue {
			t.Fatalf("expected continue when stop is not confirmed, got %v", action)
		}
	})

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	_ = r.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	prevIsTerminal := isTerminal
	isTerminal = func(_ int) bool { return false }
	defer func() { isTerminal = prevIsTerminal }()

	action, promptErr := promptPrepareControlDefault(io.Discard, make(chan os.Signal, 1))
	if promptErr == nil {
		t.Fatalf("expected prompt read error on closed stdin")
	}
	if action != prepareControlContinue {
		t.Fatalf("expected continue on prompt read error, got %v", action)
	}
}

func TestConfirmPrepareStopEOFBranch(t *testing.T) {
	confirmed, err := confirmPrepareStop(bufio.NewReader(strings.NewReader("")), io.Discard, make(chan os.Signal, 1))
	if err != nil {
		t.Fatalf("confirmPrepareStop: %v", err)
	}
	if confirmed {
		t.Fatalf("expected not confirmed on EOF")
	}
}
