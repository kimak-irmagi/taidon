package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestNewManagerRequiresStateStoreRoot(t *testing.T) {
	if _, err := NewManager(Options{}); err == nil {
		t.Fatalf("expected error for missing state store root")
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
	if got, ok := asInt(value); !ok || got != 5 {
		t.Fatalf("expected override 5, got %#v", value)
	}
}

func TestLoadOverridesInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := loadOverrides(path); err == nil {
		t.Fatalf("expected json error")
	}
}

func TestLoadOverridesNonObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`["bad"]`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := loadOverrides(path); err == nil {
		t.Fatalf("expected non-object error")
	}
}

func TestLoadOverridesReadError(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadOverrides(dir); err == nil {
		t.Fatalf("expected read error for directory path")
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

func TestConfigGetInvalidPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Get(".bad", false); err == nil {
		t.Fatalf("expected invalid path error")
	}
}

func TestConfigGetRoot(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.json"), map[string]any{
		"features": map[string]any{
			"flag": true,
		},
	})
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	value, err := mgr.Get("", false)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	mapped, ok := value.(map[string]any)
	if !ok || mapped["features"] == nil {
		t.Fatalf("expected overrides map, got %#v", value)
	}

	effective, err := mgr.Get("", true)
	if err != nil {
		t.Fatalf("Get root effective: %v", err)
	}
	mapped, ok = effective.(map[string]any)
	if !ok || mapped["orchestrator"] == nil {
		t.Fatalf("expected defaults in effective map, got %#v", effective)
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

func TestConfigSetInvalidPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Set("", true); err == nil {
		t.Fatalf("expected invalid path error")
	}
	if _, err := mgr.Set(".bad", true); err == nil {
		t.Fatalf("expected invalid path error")
	}
}

func TestConfigSetInvalidValue(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("orchestrator.jobs.maxIdentical", "bad"); err == nil {
		t.Fatalf("expected invalid value error")
	}
	if _, err := mgr.Set("features.flag", true); err != nil {
		t.Fatalf("expected non-validated path to pass: %v", err)
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
	if got, ok := asInt(value); !ok || got != 3 {
		t.Fatalf("expected 3, got %#v", value)
	}
}

func TestConfigSetIndexAtRoot(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Set("[0]", "bad"); err == nil {
		t.Fatalf("expected error for non-object root")
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

func TestConfigRemoveMissingPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Remove("missing.path"); err == nil {
		t.Fatalf("expected missing path error")
	}
}

func TestConfigRemoveInvalidPath(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Remove(".bad"); err == nil {
		t.Fatalf("expected invalid path error")
	}
}

func TestConfigRemoveWriteError(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.json"), map[string]any{
		"features": map[string]any{
			"flag": true,
		},
	})
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
	if _, err := mgr.Remove("features.flag"); err == nil {
		t.Fatalf("expected write error")
	}
	if _, err := mgr.Get("features.flag", false); err != nil {
		t.Fatalf("expected override to remain after failed remove")
	}
}

func TestConfigRemoveFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.json"), map[string]any{
		"orchestrator": map[string]any{
			"jobs": map[string]any{
				"maxIdentical": 7,
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
	value, err := mgr.Remove("orchestrator.jobs.maxIdentical")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got, ok := asInt(value); !ok || got != 2 {
		t.Fatalf("expected default after remove, got %#v", value)
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

func TestConfigDefaultSnapshotBackend(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	value, err := mgr.Get("snapshot.backend", true)
	if err != nil {
		t.Fatalf("Get effective: %v", err)
	}
	if value != "auto" {
		t.Fatalf("expected default snapshot backend auto, got %#v", value)
	}
}

func TestConfigDefaultContainerRuntime(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(Options{
		StateStoreRoot: dir,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	value, err := mgr.Get("container.runtime", true)
	if err != nil {
		t.Fatalf("Get effective: %v", err)
	}
	if value != "auto" {
		t.Fatalf("expected default container runtime auto, got %#v", value)
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
	if _, err := mgr.Set("snapshot.backend", "bad"); err == nil {
		t.Fatalf("expected validation error for snapshot backend")
	}
	if _, err := mgr.Set("container.runtime", "bad"); err == nil {
		t.Fatalf("expected validation error for container runtime")
	}
}

func TestValidateValueVariants(t *testing.T) {
	if err := validateValue("orchestrator.jobs.maxIdentical", nil); err != nil {
		t.Fatalf("expected nil to be allowed")
	}
	if err := validateValue("orchestrator.jobs.maxIdentical", int64(2)); err != nil {
		t.Fatalf("expected valid int, got %v", err)
	}
	if err := validateValue("orchestrator.jobs.maxIdentical", -1); err == nil {
		t.Fatalf("expected negative to be rejected")
	}
	if err := validateValue("orchestrator.jobs.maxIdentical", "bad"); err == nil {
		t.Fatalf("expected invalid type to be rejected")
	}
	if err := validateValue("snapshot.backend", nil); err != nil {
		t.Fatalf("expected nil snapshot backend to be allowed")
	}
	if err := validateValue("snapshot.backend", "auto"); err != nil {
		t.Fatalf("expected snapshot backend auto to be valid")
	}
	if err := validateValue("snapshot.backend", "overlay"); err != nil {
		t.Fatalf("expected snapshot backend overlay to be valid")
	}
	if err := validateValue("snapshot.backend", "btrfs"); err != nil {
		t.Fatalf("expected snapshot backend btrfs to be valid")
	}
	if err := validateValue("snapshot.backend", "copy"); err != nil {
		t.Fatalf("expected snapshot backend copy to be valid")
	}
	if err := validateValue("snapshot.backend", "bad"); err == nil {
		t.Fatalf("expected invalid snapshot backend to be rejected")
	}
	if err := validateValue("snapshot.backend", 1); err == nil {
		t.Fatalf("expected non-string snapshot backend to be rejected")
	}
	if err := validateValue("container.runtime", nil); err != nil {
		t.Fatalf("expected nil container runtime to be allowed")
	}
	if err := validateValue("container.runtime", "auto"); err != nil {
		t.Fatalf("expected container runtime auto to be valid")
	}
	if err := validateValue("container.runtime", "docker"); err != nil {
		t.Fatalf("expected container runtime docker to be valid")
	}
	if err := validateValue("container.runtime", "podman"); err != nil {
		t.Fatalf("expected container runtime podman to be valid")
	}
	if err := validateValue("container.runtime", "bad"); err == nil {
		t.Fatalf("expected invalid container runtime to be rejected")
	}
	if err := validateValue("container.runtime", 1); err == nil {
		t.Fatalf("expected non-string container runtime to be rejected")
	}
	if err := validateValue("features.other", "ok"); err != nil {
		t.Fatalf("expected other path to pass")
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

func TestParsePathInvalidForms(t *testing.T) {
	cases := []string{
		".bad",
		"a..b",
		"a[",
		"a[]",
		"a[-1]",
		"a[1",
		"a[1x]",
	}
	for _, value := range cases {
		if _, err := parsePath(value); err == nil {
			t.Fatalf("expected invalid path for %q", value)
		}
	}
}

func TestParsePathEmpty(t *testing.T) {
	segments, err := parsePath(" ")
	if err != nil {
		t.Fatalf("parsePath: %v", err)
	}
	if len(segments) != 0 {
		t.Fatalf("expected empty segments, got %#v", segments)
	}
}

func TestParsePathIndexWithDot(t *testing.T) {
	segments, err := parsePath("a[0].b")
	if err != nil {
		t.Fatalf("parsePath: %v", err)
	}
	if len(segments) != 3 || !segments[1].isIndex || segments[2].key != "b" {
		t.Fatalf("unexpected segments: %#v", segments)
	}
}

func TestGetPathValueFailures(t *testing.T) {
	if _, ok := getPathValue(map[string]any{"a": 1}, []pathSegment{{key: "missing"}}); ok {
		t.Fatalf("expected missing key")
	}
	if _, ok := getPathValue(map[string]any{"a": 1}, []pathSegment{{key: "a"}, {key: "b"}}); ok {
		t.Fatalf("expected non-map to fail")
	}
	if _, ok := getPathValue([]any{"a"}, []pathSegment{{index: 2, isIndex: true}}); ok {
		t.Fatalf("expected index out of range")
	}
	if _, ok := getPathValue(map[string]any{"a": "nope"}, []pathSegment{{key: "a"}, {index: 0, isIndex: true}}); ok {
		t.Fatalf("expected non-list to fail")
	}
}

func TestSetPathValueListExpansion(t *testing.T) {
	root := map[string]any{}
	segments := []pathSegment{{key: "list"}, {index: 2, isIndex: true}}
	out, err := setPathValue(root, segments, "ok")
	if err != nil {
		t.Fatalf("setPathValue: %v", err)
	}
	list := out["list"].([]any)
	if len(list) != 3 || list[2] != "ok" {
		t.Fatalf("unexpected list: %#v", list)
	}
}

func TestSetPathValueNestedMap(t *testing.T) {
	root := map[string]any{}
	segments := []pathSegment{{key: "a"}, {key: "b"}}
	out, err := setPathValue(root, segments, "ok")
	if err != nil {
		t.Fatalf("setPathValue: %v", err)
	}
	if out["a"].(map[string]any)["b"] != "ok" {
		t.Fatalf("unexpected map: %#v", out)
	}
}

func TestSetValueListInRange(t *testing.T) {
	list := []any{"a"}
	updated := setValue(list, []pathSegment{{index: 0, isIndex: true}}, "b")
	got := updated.([]any)
	if got[0] != "b" {
		t.Fatalf("expected list updated, got %#v", got)
	}
}

func TestSetValueNestedMapAndList(t *testing.T) {
	updated := setValue(nil, []pathSegment{{key: "a"}, {key: "b"}}, 1)
	root := updated.(map[string]any)
	if root["a"].(map[string]any)["b"] != 1 {
		t.Fatalf("unexpected nested map: %#v", root)
	}

	updated = setValue(nil, []pathSegment{{key: "a"}, {index: 1, isIndex: true}, {key: "b"}}, "x")
	root = updated.(map[string]any)
	list := root["a"].([]any)
	if len(list) != 2 {
		t.Fatalf("unexpected list length: %#v", list)
	}
	item := list[1].(map[string]any)
	if item["b"] != "x" {
		t.Fatalf("unexpected nested list item: %#v", item)
	}
}

func TestRemovePathValueIndex(t *testing.T) {
	root := map[string]any{"list": []any{"a", "b"}}
	updated, removed := removePathValue(root, []pathSegment{{key: "list"}, {index: 0, isIndex: true}})
	if !removed {
		t.Fatalf("expected removal")
	}
	list := updated["list"].([]any)
	if len(list) != 1 || list[0] != "b" {
		t.Fatalf("unexpected list: %#v", list)
	}
}

func TestRemovePathValueMissing(t *testing.T) {
	root := map[string]any{"a": 1}
	_, removed := removePathValue(root, []pathSegment{{key: "missing"}})
	if removed {
		t.Fatalf("expected missing path to be false")
	}
}

func TestSetPathValueInvalidRoot(t *testing.T) {
	_, err := setPathValue(map[string]any{}, []pathSegment{{index: 0, isIndex: true}}, "x")
	if err == nil {
		t.Fatalf("expected error for non-map root")
	}
}

func TestCloneMapDeepCopy(t *testing.T) {
	orig := map[string]any{
		"nested": map[string]any{
			"value": 1,
		},
		"list": []any{1, 2},
	}
	clone := cloneMap(orig)
	clone["nested"].(map[string]any)["value"] = 2
	clone["list"].([]any)[0] = 9
	if orig["nested"].(map[string]any)["value"] != 1 {
		t.Fatalf("expected nested map to be cloned")
	}
	if orig["list"].([]any)[0] != 1 {
		t.Fatalf("expected list to be cloned")
	}
}

func TestCloneMapNil(t *testing.T) {
	if clone := cloneMap(nil); len(clone) != 0 {
		t.Fatalf("expected empty clone, got %#v", clone)
	}
}

func TestAtomicWriteFileAndSyncDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := atomicWriteFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected data: %s", data)
	}

	fileDir := filepath.Join(dir, "file")
	if err := os.WriteFile(fileDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file dir: %v", err)
	}
	if err := atomicWriteFile(filepath.Join(fileDir, "config.json"), []byte(`{"bad":true}`)); err == nil {
		t.Fatalf("expected error for non-directory parent")
	}

	if err := syncDir(filepath.Join(dir, "missing")); err == nil {
		t.Fatalf("expected error for missing dir")
	}
}

func TestAtomicWriteFileRenameError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := atomicWriteFile(path, []byte(`{"ok":true}`)); err == nil {
		t.Fatalf("expected rename error")
	}
}

func TestAtomicWriteFileCreateTempError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not reliably block writes on windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})
	path := filepath.Join(dir, "config.json")
	if err := atomicWriteFile(path, []byte(`{"ok":true}`)); err == nil {
		t.Fatalf("expected CreateTemp error")
	}
}

func TestAtomicWriteFileMkdirError(t *testing.T) {
	prev := osMkdirAll
	osMkdirAll = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	t.Cleanup(func() { osMkdirAll = prev })

	if err := atomicWriteFile("ignored/config.json", []byte(`{}`)); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestAtomicWriteFileWriteError(t *testing.T) {
	prevCreate := osCreateTemp
	prevWrite := fileWrite
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(dir, pattern)
	}
	fileWrite = func(*os.File, []byte) (int, error) {
		return 0, errors.New("write failed")
	}
	t.Cleanup(func() {
		osCreateTemp = prevCreate
		fileWrite = prevWrite
	})

	dir := t.TempDir()
	if err := atomicWriteFile(filepath.Join(dir, "config.json"), []byte(`{}`)); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestAtomicWriteFileSyncError(t *testing.T) {
	prevCreate := osCreateTemp
	prevSync := fileSync
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(dir, pattern)
	}
	fileSync = func(*os.File) error {
		return errors.New("sync failed")
	}
	t.Cleanup(func() {
		osCreateTemp = prevCreate
		fileSync = prevSync
	})

	dir := t.TempDir()
	if err := atomicWriteFile(filepath.Join(dir, "config.json"), []byte(`{}`)); err == nil {
		t.Fatalf("expected sync error")
	}
}

func TestAtomicWriteFileCloseError(t *testing.T) {
	prevCreate := osCreateTemp
	prevClose := fileClose
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(dir, pattern)
	}
	fileClose = func(f *os.File) error {
		_ = f.Close()
		return errors.New("close failed")
	}
	t.Cleanup(func() {
		osCreateTemp = prevCreate
		fileClose = prevClose
	})

	dir := t.TempDir()
	if err := atomicWriteFile(filepath.Join(dir, "config.json"), []byte(`{}`)); err == nil {
		t.Fatalf("expected close error")
	}
}

func TestAtomicWriteFileRenameErrorStub(t *testing.T) {
	prevCreate := osCreateTemp
	prevRename := osRename
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return os.CreateTemp(dir, pattern)
	}
	osRename = func(string, string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() {
		osCreateTemp = prevCreate
		osRename = prevRename
	})

	dir := t.TempDir()
	if err := atomicWriteFile(filepath.Join(dir, "config.json"), []byte(`{}`)); err == nil {
		t.Fatalf("expected rename error")
	}
}

func TestSyncDirOpenErrorStub(t *testing.T) {
	prevOpen := osOpen
	osOpen = func(string) (*os.File, error) {
		return nil, errors.New("open failed")
	}
	t.Cleanup(func() { osOpen = prevOpen })

	if err := syncDir("missing"); err == nil {
		t.Fatalf("expected open error")
	}
}

func TestMergeMapsOverridesNonMapBase(t *testing.T) {
	base := map[string]any{
		"key": "value",
	}
	overrides := map[string]any{
		"key": map[string]any{"nested": 1},
	}
	merged := mergeMaps(base, overrides)
	if nested, ok := merged["key"].(map[string]any); !ok || nested["nested"] != 1 {
		t.Fatalf("unexpected merge result: %#v", merged["key"])
	}
}

func TestMergeMapsNilOverrides(t *testing.T) {
	base := map[string]any{"a": 1}
	out := mergeMaps(base, nil)
	if out["a"] != 1 {
		t.Fatalf("unexpected base map: %#v", out)
	}
}

func TestSyncDirIgnoresSyncError(t *testing.T) {
	prevOpen := osOpen
	prevSync := fileSync
	osOpen = func(string) (*os.File, error) {
		return os.CreateTemp(t.TempDir(), "sync")
	}
	fileSync = func(*os.File) error {
		return errors.New("sync failed")
	}
	t.Cleanup(func() {
		osOpen = prevOpen
		fileSync = prevSync
	})
	if err := syncDir("ignored"); err != nil {
		t.Fatalf("expected sync error to be ignored")
	}
}

func TestAsIntConversions(t *testing.T) {
	cases := []struct {
		value  any
		want   int64
		wantOK bool
	}{
		{value: int(5), want: 5, wantOK: true},
		{value: int32(6), want: 6, wantOK: true},
		{value: int64(7), want: 7, wantOK: true},
		{value: float32(8), want: 8, wantOK: true},
		{value: float32(8.5), wantOK: false},
		{value: float64(9), want: 9, wantOK: true},
		{value: float64(9.2), wantOK: false},
		{value: json.Number("10"), want: 10, wantOK: true},
		{value: json.Number("1e2"), wantOK: false},
		{value: json.Number("999999999999999999999999"), wantOK: false},
		{value: "nope", wantOK: false},
	}
	for _, tc := range cases {
		got, ok := asInt(tc.value)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("asInt(%#v)=%d,%v; want %d,%v", tc.value, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestRemoveValueFromMapAndList(t *testing.T) {
	root := map[string]any{
		"nested": map[string]any{
			"value": 1,
		},
	}
	updated, removed := removeValue(root, []pathSegment{{key: "nested"}, {key: "value"}})
	if !removed {
		t.Fatalf("expected removal to succeed")
	}
	nested, ok := updated.(map[string]any)
	if !ok || nested["nested"] == nil {
		t.Fatalf("expected nested map, got %#v", updated)
	}
	inner, ok := nested["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %#v", nested["nested"])
	}
	if _, ok := inner["value"]; ok {
		t.Fatalf("expected nested key removed")
	}

	list := []any{"a", "b", "c"}
	updatedList, removed := removeValue(list, []pathSegment{{index: 1, isIndex: true}})
	if !removed {
		t.Fatalf("expected list removal")
	}
	gotList, ok := updatedList.([]any)
	if !ok || len(gotList) != 2 || gotList[0] != "a" || gotList[1] != "c" {
		t.Fatalf("unexpected list result: %#v", updatedList)
	}
}

func TestRemoveValueMissingReturnsFalse(t *testing.T) {
	if _, removed := removeValue(map[string]any{"a": 1}, []pathSegment{{key: "missing"}}); removed {
		t.Fatalf("expected no removal for missing key")
	}
	if _, removed := removeValue("nope", []pathSegment{{index: 0, isIndex: true}}); removed {
		t.Fatalf("expected no removal for non-list index")
	}
	if _, removed := removeValue([]any{"a"}, []pathSegment{{index: 2, isIndex: true}}); removed {
		t.Fatalf("expected no removal for out-of-range index")
	}
	list := []any{map[string]any{"a": 1}}
	if _, removed := removeValue(list, []pathSegment{{index: 0, isIndex: true}, {key: "missing"}}); removed {
		t.Fatalf("expected nested missing removal to be false")
	}
}

func TestMergeMapsRecursesAndOverrides(t *testing.T) {
	base := map[string]any{
		"a": map[string]any{
			"b": 1,
		},
		"list": []any{1},
	}
	overrides := map[string]any{
		"a": map[string]any{
			"c": 2,
		},
		"list": []any{2},
		"new":  "x",
	}
	merged := mergeMaps(base, overrides)
	a, ok := merged["a"].(map[string]any)
	if !ok || a["b"] != 1 || a["c"] != 2 {
		t.Fatalf("unexpected merged map: %#v", merged["a"])
	}
	if list, ok := merged["list"].([]any); !ok || len(list) != 1 || list[0] != 2 {
		t.Fatalf("unexpected list override: %#v", merged["list"])
	}
	if merged["new"] != "x" {
		t.Fatalf("expected new key, got %#v", merged["new"])
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
		"container": map[string]any{
			"runtime": "auto",
		},
		"snapshot": map[string]any{
			"backend": "auto",
		},
		"orchestrator": map[string]any{
			"jobs": map[string]any{
				"maxIdentical": 2,
			},
		},
	}
}
