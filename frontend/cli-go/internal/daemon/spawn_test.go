package daemon

import (
	"path/filepath"
	"testing"
)

func TestBuildDaemonCommand(t *testing.T) {
	runDir := filepath.Join("C:\\", "sqlrs", "run")
	statePath := filepath.Join("C:\\", "sqlrs", "engine.json")
	cmd, err := buildDaemonCommand("sqlrs-engine", runDir, statePath)
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "sqlrs-engine" {
		t.Fatalf("unexpected args: %+v", cmd.Args)
	}
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
}
