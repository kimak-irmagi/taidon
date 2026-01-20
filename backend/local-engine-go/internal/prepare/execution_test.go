package prepare

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseImageID(t *testing.T) {
	engine, version := parseImageID("postgres:17")
	if engine != "postgres" || version != "17" {
		t.Fatalf("unexpected parse result: %s %s", engine, version)
	}

	engine, version = parseImageID("ghcr.io/sqlrs/postgres:15")
	if engine != "postgres" || version != "15" {
		t.Fatalf("unexpected parse result: %s %s", engine, version)
	}

	engine, version = parseImageID("postgres")
	if engine != "postgres" || version != "latest" {
		t.Fatalf("unexpected parse result: %s %s", engine, version)
	}

	engine, version = parseImageID("pg@sha256:deadbeef")
	if engine != "pg" || version != "sha256_deadbeef" {
		t.Fatalf("unexpected digest parse result: %s %s", engine, version)
	}

	engine, version = parseImageID("")
	if engine != "unknown" || version != "latest" {
		t.Fatalf("unexpected empty parse result: %s %s", engine, version)
	}
}

func TestResolveStatePaths(t *testing.T) {
	root := filepath.Join("tmp", "state-store")
	paths, err := resolveStatePaths(root, "postgres:17", "state-1")
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if paths.baseDir != filepath.Join(root, "engines", "postgres", "17", "base") {
		t.Fatalf("unexpected base dir: %s", paths.baseDir)
	}
	if paths.statesDir != filepath.Join(root, "engines", "postgres", "17", "states") {
		t.Fatalf("unexpected states dir: %s", paths.statesDir)
	}
	if paths.stateDir != filepath.Join(root, "engines", "postgres", "17", "states", "state-1") {
		t.Fatalf("unexpected state dir: %s", paths.stateDir)
	}
}

func TestResolveStatePathsEmptyRoot(t *testing.T) {
	if _, err := resolveStatePaths("", "postgres:17", "state-1"); err == nil {
		t.Fatalf("expected error for empty root")
	}
}

func TestMergeEnvOverrides(t *testing.T) {
	base := []string{"FOO=bar", "BAZ=1"}
	merged := mergeEnv(base, map[string]string{
		"FOO": "next",
		"NEW": "ok",
	})
	found := map[string]string{}
	for _, entry := range merged {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				found[entry[:i]] = entry[i+1:]
				break
			}
		}
	}
	if found["FOO"] != "next" {
		t.Fatalf("expected FOO override, got %q", found["FOO"])
	}
	if found["NEW"] != "ok" {
		t.Fatalf("expected NEW override, got %q", found["NEW"])
	}
	if found["BAZ"] != "1" {
		t.Fatalf("expected BAZ preserved, got %q", found["BAZ"])
	}
}

func TestMergeEnvCaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only env behavior")
	}
	base := []string{"Path=foo"}
	merged := mergeEnv(base, map[string]string{
		"PATH": "bar",
	})
	found := map[string]string{}
	for _, entry := range merged {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				found[entry[:i]] = entry[i+1:]
				break
			}
		}
	}
	if found["PATH"] != "bar" && found["Path"] != "bar" {
		t.Fatalf("expected PATH override, got %+v", found)
	}
}
