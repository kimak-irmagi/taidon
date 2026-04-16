package app

import (
	"strings"
	"testing"
)

func TestRunRejectsPrepareRefCompositeRaw(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"prepare:psql", "--ref", "HEAD", "--", "-f", "prepare.sql", "run:psql", "--", "-c", "select 1"})
	if err == nil || !strings.Contains(err.Error(), "prepare --ref does not support composite run yet") {
		t.Fatalf("expected bounded ref composite rejection, got %v", err)
	}
}

func TestRunRejectsPrepareRefCompositeAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"prepare", "--ref", "HEAD", "chinook", "run", "smoke"})
	if err == nil || !strings.Contains(err.Error(), "prepare --ref does not support composite run yet") {
		t.Fatalf("expected bounded ref composite rejection, got %v", err)
	}
}
