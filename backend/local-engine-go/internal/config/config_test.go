package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigLoadDefaultsWhenMissingFile(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	value, err := mgr.Get("orchestrator.jobs.maxIdentical", true)
	if err != nil {
		t.Fatalf("Get effective: %v", err)
	}
	if value != 2 {
		t.Fatalf("expected default maxIdentical=2, got %#v", value)
	}

	if _, err := mgr.Get("orchestrator.jobs.maxIdentical", false); err == nil {
		t.Fatalf("expected missing override error")
	}
}

func TestConfigLoadOverridesFromFile(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.json"), map[string]any{
		"orchestrator": map[string]any{
			"jobs": map[string]any{
				"maxIdentical": 5,
			},
		},
	})

	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	value, err := mgr.Get("orchestrator.jobs.maxIdentical", false)
	if err != nil {
		t.Fatalf("Get override: %v", err)
	}
	if value != 5 {
		t.Fatalf("expected override 5, got %#v", value)
	}
}

func TestConfigGetPathMissing(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Get("feature.flags.missing", false); err == nil {
		t.Fatalf("expected missing path error")
	}
}

func TestConfigSetCreatesNestedPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = mgr.Set("features.beta", true)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, err := mgr.Get("features.beta", false)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != true {
		t.Fatalf("expected true, got %#v", value)
	}
}

func TestConfigSetArrayIndex(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = mgr.Set("limits.jobs[1].maxIdentical", 3)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, err := mgr.Get("limits.jobs[1].maxIdentical", false)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != 3 {
		t.Fatalf("expected 3, got %#v", value)
	}
}

func TestConfigRemoveKey(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("features.flag", true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := mgr.Remove("features.flag"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := mgr.Get("features.flag", false); err == nil {
		t.Fatalf("expected missing path after remove")
	}
}

func TestConfigSetNullValue(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("features.nullable", nil); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, err := mgr.Get("features.nullable", false)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil, got %#v", value)
	}
}

func TestConfigEffectiveMerged(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	value, err := mgr.Get("orchestrator.jobs.maxIdentical", true)
	if err != nil {
		t.Fatalf("Get effective: %v", err)
	}
	if value != 2 {
		t.Fatalf("expected default value 2, got %#v", value)
	}
}

func TestConfigSchemaValidationRejects(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("orchestrator.jobs.maxIdentical", -1); err == nil {
		t.Fatalf("expected validation error for negative maxIdentical")
	}
}

func TestConfigAtomicWriteCommit(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Set("features.flag", true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode config.json: %v", err)
	}
	if payload["features"] == nil {
		t.Fatalf("expected features in config.json")
	}
}

func TestConfigAtomicWriteFailureNoCommit(t *testing.T) {
	dir := t.TempDir()
	writeErr := errors.New("write failed")
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
		WriteFile: func(string, []byte) error {
			return writeErr
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("features.flag", true); err == nil {
		t.Fatalf("expected write error")
	}
	if _, err := mgr.Get("features.flag", false); err == nil {
		t.Fatalf("expected no in-memory commit on failure")
	}
}

func TestConfigConcurrentSetSerialized(t *testing.T) {
	dir := t.TempDir()
	var inFlight int32
	writeCalled := make(chan struct{}, 2)
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
		WriteFile: func(string, []byte) error {
			if atomic.AddInt32(&inFlight, 1) != 1 {
				t.Errorf("write file called concurrently")
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			writeCalled <- struct{}{}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = mgr.Set("features.a", true)
	}()
	go func() {
		defer wg.Done()
		_, _ = mgr.Set("features.b", false)
	}()
	wg.Wait()
	close(writeCalled)
}

func TestConfigUsesDefaultsAndSchema(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	value, err := mgr.Get("orchestrator.jobs.maxIdentical", true)
	if err != nil {
		t.Fatalf("Get default: %v", err)
	}
	if value != 2 {
		t.Fatalf("expected default value 2, got %#v", value)
	}
	schema, ok := mgr.Schema().(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("unexpected schema: %#v", schema)
	}
}

func writeConfigFile(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func testDefaults() map[string]any {
	return map[string]any{
		"orchestrator": map[string]any{
			"jobs": map[string]any{
				"maxIdentical": 2,
			},
		},
	}
}
