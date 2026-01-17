package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "file.txt")

	if err := AtomicWriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("atomic write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestAtomicWriteFileReturnsErrorOnInvalidDir(t *testing.T) {
	temp := t.TempDir()
	dirFile := filepath.Join(temp, "not-a-dir")
	if err := os.WriteFile(dirFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	path := filepath.Join(dirFile, "file.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected error for invalid dir")
	}
}
