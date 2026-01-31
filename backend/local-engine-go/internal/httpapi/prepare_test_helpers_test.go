package httpapi

import (
	"path/filepath"
	"testing"
)

func TestFakeStateFSJobRuntimeDir(t *testing.T) {
	root := t.TempDir()
	fs := &fakeStateFS{}
	dir, err := fs.JobRuntimeDir(root, "job-1")
	if err != nil {
		t.Fatalf("JobRuntimeDir: %v", err)
	}
	expected := filepath.Join(root, "jobs", "job-1", "runtime")
	if dir != expected {
		t.Fatalf("expected job runtime dir %s, got %s", expected, dir)
	}
}
