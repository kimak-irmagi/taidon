package prepare

import (
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/statefs"
)

func TestFakeStateFSCapabilities(t *testing.T) {
	caps := statefsCapabilities()
	fs := &fakeStateFS{caps: caps}
	got := fs.Capabilities()
	if got != caps {
		t.Fatalf("expected capabilities %+v, got %+v", caps, got)
	}
}

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

func statefsCapabilities() statefs.Capabilities {
	return statefs.Capabilities{
		RequiresDBStop:        true,
		SupportsWritableClone: true,
		SupportsSendReceive:   true,
	}
}
