package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	fallback := 5 * time.Second
	out, err := ParseDuration("", fallback)
	if err != nil {
		t.Fatalf("ParseDuration: %v", err)
	}
	if out != fallback {
		t.Fatalf("expected fallback, got %v", out)
	}
	if _, err := ParseDuration("bad", fallback); err == nil {
		t.Fatalf("expected error for invalid duration")
	}
}

func TestLookupDBMSImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: postgres:15\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	image, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if !ok || image != "postgres:15" {
		t.Fatalf("unexpected image: %q (ok=%v)", image, ok)
	}
}

func TestLookupDBMSImageMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: \"\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if ok {
		t.Fatalf("expected no image")
	}
}
