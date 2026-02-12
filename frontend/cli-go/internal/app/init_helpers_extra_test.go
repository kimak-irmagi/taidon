package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSplitInitMode(t *testing.T) {
	mode, rest, err := splitInitMode(nil)
	if err != nil || mode != "local" || len(rest) != 0 {
		t.Fatalf("expected default local, got mode=%s rest=%v err=%v", mode, rest, err)
	}
	mode, rest, err = splitInitMode([]string{"-h"})
	if err != nil || mode != "local" || len(rest) != 1 {
		t.Fatalf("expected local for flag start, got mode=%s rest=%v err=%v", mode, rest, err)
	}
	mode, rest, err = splitInitMode([]string{"remote", "--url", "x"})
	if err != nil || mode != "remote" || len(rest) != 2 {
		t.Fatalf("expected remote mode, got mode=%s rest=%v err=%v", mode, rest, err)
	}
	if _, _, err := splitInitMode([]string{"unknown"}); err == nil {
		t.Fatalf("expected error for unknown mode")
	}
}

func TestSnapshotAndStoreTypeHelpers(t *testing.T) {
	if normalizeSnapshot("  BTRFS ") != "btrfs" {
		t.Fatalf("unexpected normalizeSnapshot")
	}
	if normalizeStoreType("  IMAGE ") != "image" {
		t.Fatalf("unexpected normalizeStoreType")
	}
	if !isKnownSnapshot("auto") || !isKnownSnapshot("overlay") || isKnownSnapshot("nope") {
		t.Fatalf("unexpected isKnownSnapshot results")
	}
	if !isKnownStoreType("dir") || !isKnownStoreType("image") || isKnownStoreType("bad") {
		t.Fatalf("unexpected isKnownStoreType results")
	}
}

func TestReadConfigMapEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	raw, err := readConfigMap(path)
	if err != nil {
		t.Fatalf("readConfigMap: %v", err)
	}
	if raw == nil || len(raw) != 0 {
		t.Fatalf("expected empty map, got %#v", raw)
	}
}

func TestParseStoreSizeGB(t *testing.T) {
	if _, err := parseStoreSizeGB(""); err == nil {
		t.Fatalf("expected empty size error")
	}
	if _, err := parseStoreSizeGB("10"); err == nil {
		t.Fatalf("expected missing suffix error")
	}
	if _, err := parseStoreSizeGB("gb"); err == nil {
		t.Fatalf("expected invalid size error")
	}
	if _, err := parseStoreSizeGB("0GB"); err == nil {
		t.Fatalf("expected non-positive error")
	}
	if size, err := parseStoreSizeGB("12GB"); err != nil || size != 12 {
		t.Fatalf("expected size 12, got %d err=%v", size, err)
	}
}

func TestResolveStoreTypeBranches(t *testing.T) {
	if got, _ := resolveStoreType("copy", ""); got != "dir" {
		t.Fatalf("expected dir for copy, got %q", got)
	}
	if got, _ := resolveStoreType("overlay", ""); got != "dir" {
		t.Fatalf("expected dir for overlay, got %q", got)
	}
	if runtime.GOOS == "windows" {
		if got, _ := resolveStoreType("btrfs", ""); got != "image" {
			t.Fatalf("expected image for windows btrfs, got %q", got)
		}
		if got, _ := resolveStoreType("auto", ""); got != "image" {
			t.Fatalf("expected image for windows auto, got %q", got)
		}
	}
	if got, _ := resolveStoreType("auto", "dir"); got != "dir" {
		t.Fatalf("expected explicit dir, got %q", got)
	}
}

func TestDefaultStoreRootEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", root)
	if out, err := defaultStoreRoot(); err != nil || out != root {
		t.Fatalf("expected env root, got %q err=%v", out, err)
	}
}
