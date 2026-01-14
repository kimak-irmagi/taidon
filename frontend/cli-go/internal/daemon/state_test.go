package daemon

import (
	"errors"
	"path/filepath"
	"testing"

	"sqlrs/cli/internal/client"
)

func TestEngineStateReadWrite(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "engine.json")

	state := EngineState{
		Endpoint:   "http://127.0.0.1:17654",
		PID:        1234,
		StartedAt:  "2026-01-11T12:34:56Z",
		AuthToken:  "token",
		Version:    "0.1.0",
		InstanceID: "instance",
	}

	if err := WriteEngineState(path, state); err != nil {
		t.Fatalf("write engine state: %v", err)
	}

	read, err := ReadEngineState(path)
	if err != nil {
		t.Fatalf("read engine state: %v", err)
	}

	if read.Endpoint != state.Endpoint || read.AuthToken != state.AuthToken || read.InstanceID != state.InstanceID {
		t.Fatalf("unexpected read state: %#v", read)
	}
}

func TestEngineStateStaleRules(t *testing.T) {
	state := EngineState{InstanceID: "abc", PID: 10}

	if IsEngineStateStale(state, client.HealthResponse{InstanceID: "abc"}, nil, true) {
		t.Fatalf("expected state to be fresh")
	}

	if !IsEngineStateStale(state, client.HealthResponse{}, errors.New("down"), false) {
		t.Fatalf("expected state to be stale on health error")
	}

	if !IsEngineStateStale(state, client.HealthResponse{InstanceID: "def"}, nil, true) {
		t.Fatalf("expected state to be stale on instance mismatch")
	}
}
