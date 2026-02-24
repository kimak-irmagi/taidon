package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/daemon"
)

func TestRunRunKindRequiredCoverage(t *testing.T) {
	_, err := RunRun(context.Background(), RunOptions{
		Kind:        "",
		InstanceRef: "inst",
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "run kind is required") {
		t.Fatalf("expected run kind required error, got %v", err)
	}
}

func TestDeleteInstanceDetailedCoverageBranches(t *testing.T) {
	if _, _, err := DeleteInstanceDetailed(context.Background(), RunOptions{}, ""); err != nil {
		t.Fatalf("expected nil for empty instance id, got %v", err)
	}

	if _, _, err := DeleteInstanceDetailed(context.Background(), RunOptions{Mode: "remote"}, "inst"); err == nil {
		t.Fatalf("expected runClient error")
	}
}

func TestRunClientLocalEmptyEndpointVerboseCoverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
	}))
	t.Cleanup(server.Close)

	stateDir := t.TempDir()
	if err := daemon.WriteEngineState(filepath.Join(stateDir, "engine.json"), daemon.EngineState{
		Endpoint:   server.URL,
		AuthToken:  "token",
		InstanceID: "inst",
	}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	if _, err := runClient(context.Background(), RunOptions{
		Mode:      "local",
		Endpoint:  "",
		Autostart: false,
		StateDir:  stateDir,
		Timeout:   time.Second,
		Verbose:   true,
	}); err != nil {
		t.Fatalf("runClient: %v", err)
	}
}

func TestRunClientRemoteVerboseCoverage(t *testing.T) {
	cliClient, err := runClient(context.Background(), RunOptions{
		Mode:     "remote",
		Endpoint: "http://127.0.0.1:1",
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil || cliClient == nil {
		t.Fatalf("runClient: client=%v err=%v", cliClient, err)
	}
}

func TestReadRunStreamWriterAndErrorBranchesCoverage(t *testing.T) {
	_, err := readRunStream(strings.NewReader(`{"type":"stdout","data":"x"}`+"\n"), errWriter{}, io.Discard)
	if err == nil {
		t.Fatalf("expected stdout writer error")
	}

	_, err = readRunStream(strings.NewReader(`{"type":"stderr","data":"x"}`+"\n"), io.Discard, errWriter{})
	if err == nil {
		t.Fatalf("expected stderr writer error")
	}

	_, err = readRunStream(strings.NewReader(`{"type":"error","error":{"message":"boom"}}`+"\n"), io.Discard, io.Discard)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected error event message, got %v", err)
	}

	_, err = readRunStream(&scannerErrReader{}, io.Discard, io.Discard)
	if err == nil {
		t.Fatalf("expected scanner error")
	}

	result, err := readRunStream(strings.NewReader(""), io.Discard, io.Discard)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("expected empty stream exitCode=0, got %+v err=%v", result, err)
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type scannerErrReader struct {
	read bool
}

func (r *scannerErrReader) Read([]byte) (int, error) {
	if !r.read {
		r.read = true
		return 0, errors.New("scan failed")
	}
	return 0, os.ErrClosed
}
