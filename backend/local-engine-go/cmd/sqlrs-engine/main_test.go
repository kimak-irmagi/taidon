package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRandomHex(t *testing.T) {
	out, err := randomHex(16)
	if err != nil {
		t.Fatalf("randomHex: %v", err)
	}
	if len(out) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(out))
	}
	if _, err := hex.DecodeString(out); err != nil {
		t.Fatalf("expected valid hex, got %q", out)
	}
}

func TestWriteEngineState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	state := EngineState{
		Endpoint:   "127.0.0.1:1234",
		PID:        42,
		StartedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		AuthToken:  "token",
		Version:    "test",
		InstanceID: "instance",
	}

	if err := writeEngineState(path, state); err != nil {
		t.Fatalf("writeEngineState: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read engine.json: %v", err)
	}

	var got EngineState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal engine.json: %v", err)
	}

	if got.Endpoint != state.Endpoint || got.PID != state.PID || got.InstanceID != state.InstanceID {
		t.Fatalf("unexpected engine.json contents: %+v", got)
	}
}

func TestRemoveEngineState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	removeEngineState(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got err=%v", err)
	}
}
