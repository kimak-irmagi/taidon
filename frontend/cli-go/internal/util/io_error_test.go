package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileCreateTempError(t *testing.T) {
	prev := createTempFn
	createTempFn = func(string, string) (*os.File, error) {
		return nil, errors.New("create temp failed")
	}
	t.Cleanup(func() { createTempFn = prev })

	path := filepath.Join(t.TempDir(), "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected create temp error")
	}
}

func TestAtomicWriteFileWriteError(t *testing.T) {
	dir := t.TempDir()
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	})

	prevCreate := createTempFn
	prevWrite := writeFileFn
	createTempFn = func(string, string) (*os.File, error) { return temp, nil }
	writeFileFn = func(*os.File, []byte) (int, error) {
		return 0, errors.New("write failed")
	}
	t.Cleanup(func() {
		createTempFn = prevCreate
		writeFileFn = prevWrite
	})

	path := filepath.Join(dir, "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestAtomicWriteFileCloseError(t *testing.T) {
	dir := t.TempDir()
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}

	prevCreate := createTempFn
	prevClose := closeFileFn
	createTempFn = func(string, string) (*os.File, error) { return temp, nil }
	closeFileFn = func(f *os.File) error {
		_ = f.Close()
		return errors.New("close failed")
	}
	t.Cleanup(func() {
		createTempFn = prevCreate
		closeFileFn = prevClose
		_ = os.Remove(temp.Name())
	})

	path := filepath.Join(dir, "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected close error")
	}
}

func TestAtomicWriteFileChmodError(t *testing.T) {
	dir := t.TempDir()
	prev := chmodFn
	chmodFn = func(string, os.FileMode) error {
		return errors.New("chmod failed")
	}
	t.Cleanup(func() { chmodFn = prev })

	path := filepath.Join(dir, "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected chmod error")
	}
}

func TestAtomicWriteFileRenameError(t *testing.T) {
	dir := t.TempDir()
	prev := renameFn
	renameFn = func(string, string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() { renameFn = prev })

	path := filepath.Join(dir, "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected rename error")
	}
}

func TestNDJSONReaderError(t *testing.T) {
	reader := NewNDJSONReader(errorReader{})
	if _, err := reader.Next(); err == nil {
		t.Fatalf("expected reader error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
