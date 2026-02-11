package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputePsqlContentDigestBranches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sql"), "select 1;")
	digest, err := computePsqlContentDigest([]psqlInput{
		{kind: "command", value: "select 1;"},
		{kind: "stdin", value: "select 2;"},
		{kind: "file", value: filepath.Join(dir, "a.sql")},
	}, dir)
	if err != nil {
		t.Fatalf("computePsqlContentDigest: %v", err)
	}
	if digest.hash == "" {
		t.Fatalf("expected hash")
	}
	if len(digest.filePaths) == 0 {
		t.Fatalf("expected file paths")
	}

	if _, err := computePsqlContentDigest([]psqlInput{{kind: "unknown", value: "x"}}, dir); err == nil {
		t.Fatalf("expected error for unsupported input kind")
	}
}

func TestComputePsqlContentDigestCommandError(t *testing.T) {
	if _, err := computePsqlContentDigest([]psqlInput{{kind: "command", value: `\i missing.sql`}}, ""); err == nil {
		t.Fatalf("expected error for invalid include")
	}
}

func TestPsqlContentTrackerExpandFileErrors(t *testing.T) {
	tracker := &psqlContentTracker{
		workDir: "",
		locker:  &contentLock{files: map[string]*os.File{}},
		seen:    map[string]struct{}{},
		stack:   map[string]struct{}{},
	}
	if err := tracker.expandFile(" ", &strings.Builder{}); err == nil {
		t.Fatalf("expected error for empty path")
	}

	path := writeTempContentFile(t, "data")
	tracker.stack[path] = struct{}{}
	if err := tracker.expandFile(path, &strings.Builder{}); err == nil {
		t.Fatalf("expected recursive include error")
	}
	delete(tracker.stack, path)

	closed, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	tracker.locker.files[path] = closed
	tracker.locker.order = append(tracker.locker.order, path)
	tracker.seen[path] = struct{}{}
	if err := tracker.expandFile(path, &strings.Builder{}); err == nil {
		t.Fatalf("expected read error for closed file")
	}
}

func TestPsqlContentTrackerResolveIncludePath(t *testing.T) {
	tracker := &psqlContentTracker{
		workDir: "",
		locker:  &contentLock{files: map[string]*os.File{}},
		seen:    map[string]struct{}{},
		stack:   map[string]struct{}{},
	}
	if _, err := tracker.resolveIncludePath(`\i`, " ", ""); err == nil {
		t.Fatalf("expected error for empty include arg")
	}
	abs := filepath.Join(t.TempDir(), "a.sql")
	if out, err := tracker.resolveIncludePath(`\i`, abs, ""); err != nil || out != filepath.Clean(abs) {
		t.Fatalf("expected absolute include, got %q err=%v", out, err)
	}
	if _, err := tracker.resolveIncludePath(`\i`, "rel.sql", ""); err == nil {
		t.Fatalf("expected error for missing workdir")
	}
	workDir := t.TempDir()
	tracker.workDir = workDir
	out, err := tracker.resolveIncludePath(`\i`, "rel.sql", "")
	if err != nil || out != filepath.Join(workDir, "rel.sql") {
		t.Fatalf("unexpected include path: %q err=%v", out, err)
	}
	out, err = tracker.resolveIncludePath(`\ir`, "rel.sql", filepath.Join(workDir, "file.sql"))
	if err != nil || out != filepath.Join(workDir, "rel.sql") {
		t.Fatalf("unexpected include_relative path: %q err=%v", out, err)
	}
}

func TestPsqlContentTrackerLockFileBranches(t *testing.T) {
	path := writeTempContentFile(t, "data")
	tracker := &psqlContentTracker{
		workDir: "",
		locker:  &contentLock{},
		seen:    map[string]struct{}{},
		stack:   map[string]struct{}{},
	}
	t.Cleanup(func() {
		if tracker.locker != nil {
			_ = tracker.locker.Close()
		}
	})

	if err := tracker.lockFile(path); err != nil {
		t.Fatalf("lockFile: %v", err)
	}
	if tracker.locker.files == nil || tracker.locker.files[path] == nil {
		t.Fatalf("expected locker files to be populated")
	}
	if err := tracker.lockFile(path); err != nil {
		t.Fatalf("lockFile repeat: %v", err)
	}

	orig := lockFileSharedFn
	lockFileSharedFn = func(*os.File) error {
		return os.ErrPermission
	}
	t.Cleanup(func() { lockFileSharedFn = orig })

	path2 := writeTempContentFile(t, "data-2")
	if err := tracker.lockFile(path2); err == nil {
		t.Fatalf("expected lockFile error")
	}
}

func TestExpandContentIncludeErrors(t *testing.T) {
	tracker := &psqlContentTracker{
		workDir: "",
		locker:  &contentLock{files: map[string]*os.File{}},
		seen:    map[string]struct{}{},
		stack:   map[string]struct{}{},
	}
	builder := &strings.Builder{}
	if err := tracker.expandContent(`\i missing.sql`, "", builder); err == nil {
		t.Fatalf("expected error for missing workdir")
	}
}

func TestParsePsqlIncludeAndSplitCommand(t *testing.T) {
	if _, _, ok := parsePsqlInclude("select 1"); ok {
		t.Fatalf("expected non-include")
	}
	if _, _, ok := parsePsqlInclude(`\i`); ok {
		t.Fatalf("expected missing include arg")
	}
	if _, _, ok := parsePsqlInclude(`\notinclude file.sql`); ok {
		t.Fatalf("expected unknown command")
	}
	cmd, arg, ok := parsePsqlInclude(`\include "file.sql"`)
	if !ok || cmd == "" || arg != "file.sql" {
		t.Fatalf("unexpected include parse: %q %q %v", cmd, arg, ok)
	}

	parts := splitPsqlCommand(`\i "file name.sql"`)
	if len(parts) != 2 || parts[1] != "file name.sql" {
		t.Fatalf("unexpected split: %+v", parts)
	}
	parts = splitPsqlCommand(`\i 'file.sql' other`)
	if len(parts) != 3 {
		t.Fatalf("unexpected split count: %+v", parts)
	}
}
