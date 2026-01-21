package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupDBMSImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: postgres:17\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	image, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if !ok || image != "postgres:17" {
		t.Fatalf("unexpected lookup result: ok=%v image=%q", ok, image)
	}
}

func TestLookupDBMSImageMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("client:\n  timeout: 1s\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if ok {
		t.Fatalf("expected missing image to return ok=false")
	}
}

func TestLookupDBMSImageInvalidType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: 123\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if ok {
		t.Fatalf("expected invalid type to return ok=false")
	}
}

func TestLookupDBMSImageEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: \"   \"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if ok {
		t.Fatalf("expected empty image to return ok=false")
	}
}
