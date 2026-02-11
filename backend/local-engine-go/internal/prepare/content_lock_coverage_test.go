package prepare

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLockContentFileOpenError(t *testing.T) {
	_, err := lockContentFile(filepath.Join(t.TempDir(), "missing.sql"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestLockContentFileLockError(t *testing.T) {
	path := writeTempContentFile(t, "data")
	orig := lockFileSharedFn
	lockFileSharedFn = func(*os.File) error {
		return errors.New("lock failed")
	}
	t.Cleanup(func() { lockFileSharedFn = orig })

	if _, err := lockContentFile(path); err == nil {
		t.Fatalf("expected lock error")
	}
}

func TestLockContentFilesEmptyAndError(t *testing.T) {
	locks, err := lockContentFiles([]string{"", "  "})
	if err != nil {
		t.Fatalf("lockContentFiles empty: %v", err)
	}
	if locks == nil || len(locks.files) != 0 {
		t.Fatalf("expected empty lock set, got %+v", locks)
	}

	pathA := writeTempContentFile(t, "a")
	pathB := writeTempContentFile(t, "b")
	orig := lockFileSharedFn
	calls := 0
	lockFileSharedFn = func(*os.File) error {
		calls++
		if calls == 2 {
			return errors.New("lock failed")
		}
		return nil
	}
	t.Cleanup(func() { lockFileSharedFn = orig })

	if _, err := lockContentFiles([]string{pathA, pathB}); err == nil {
		t.Fatalf("expected lockContentFiles error")
	}
}

func TestContentLockClosePaths(t *testing.T) {
	var nilLock *contentLock
	if err := nilLock.Close(); err != nil {
		t.Fatalf("expected nil Close error, got %v", err)
	}

	lock := &contentLock{
		files: map[string]*os.File{"a": nil},
		order: []string{"a"},
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("expected nil Close error, got %v", err)
	}
}

func TestContentLockCloseUnlockError(t *testing.T) {
	path := writeTempContentFile(t, "data")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	lock := &contentLock{
		files: map[string]*os.File{path: f},
		order: []string{path},
	}
	orig := unlockFileSharedFn
	unlockFileSharedFn = func(*os.File) error {
		return errors.New("unlock failed")
	}
	t.Cleanup(func() { unlockFileSharedFn = orig })

	if err := lock.Close(); err == nil {
		t.Fatalf("expected unlock error")
	}
}

func TestContentLockCloseFileCloseError(t *testing.T) {
	path := writeTempContentFile(t, "data")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	lock := &contentLock{
		files: map[string]*os.File{path: f},
		order: []string{path},
	}
	orig := unlockFileSharedFn
	unlockFileSharedFn = func(*os.File) error { return nil }
	t.Cleanup(func() { unlockFileSharedFn = orig })

	if err := lock.Close(); err == nil {
		t.Fatalf("expected close error")
	}
}

func TestContentLockReadFileBranches(t *testing.T) {
	path := writeTempContentFile(t, "payload")

	var nilLock *contentLock
	if data, err := nilLock.readFile(path); err != nil || string(data) != "payload" {
		t.Fatalf("readFile nil lock: %v data=%q", err, string(data))
	}

	lock := &contentLock{files: map[string]*os.File{}}
	if data, err := lock.readFile(path); err != nil || string(data) != "payload" {
		t.Fatalf("readFile empty lock: %v data=%q", err, string(data))
	}

	closed, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	lock = &contentLock{
		files: map[string]*os.File{path: closed},
		order: []string{path},
	}
	if _, err := lock.readFile(path); err == nil {
		t.Fatalf("expected readFile seek error")
	}

	open, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer open.Close()
	lock = &contentLock{
		files: map[string]*os.File{path: open},
		order: []string{path},
	}
	orig := readAllFn
	readAllFn = func(io.Reader) ([]byte, error) {
		return nil, os.ErrClosed
	}
	t.Cleanup(func() { readAllFn = orig })

	data, err := lock.readFile(path)
	if err != nil || string(data) != "payload" {
		t.Fatalf("readFile fallback: %v data=%q", err, string(data))
	}
}

func writeTempContentFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.sql")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}
