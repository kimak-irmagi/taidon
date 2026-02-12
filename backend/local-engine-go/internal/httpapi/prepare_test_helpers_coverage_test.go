package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/prepare"
	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestPrepareTestHelpersCoverage(t *testing.T) {
	rt := &fakeRuntime{}
	dir := t.TempDir()
	if err := rt.InitBase(context.Background(), "image-1", dir); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PG_VERSION")); err != nil {
		t.Fatalf("expected PG_VERSION: %v", err)
	}
	if out, err := rt.ResolveImage(context.Background(), "image-1"); err != nil || out == "" {
		t.Fatalf("ResolveImage: out=%q err=%v", out, err)
	}
	inst, err := rt.Start(context.Background(), engineRuntime.StartRequest{ImageID: "image-1"})
	if err != nil || inst.ID == "" {
		t.Fatalf("Start: inst=%+v err=%v", inst, err)
	}
	if err := rt.Stop(context.Background(), inst.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := rt.Exec(context.Background(), inst.ID, engineRuntime.ExecRequest{Args: []string{"echo", "ok"}}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if err := rt.WaitForReady(context.Background(), inst.ID, time.Second); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}

	fs := &fakeStateFS{}
	if fs.Kind() != "fake" {
		t.Fatalf("expected fake kind")
	}
	if caps := fs.Capabilities(); !caps.RequiresDBStop || !caps.SupportsWritableClone || caps.SupportsSendReceive {
		t.Fatalf("unexpected caps: %+v", caps)
	}
	if err := fs.Validate(dir); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := fs.BaseDir(dir, "image-1"); err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if _, err := fs.StatesDir(dir, "image-1"); err != nil {
		t.Fatalf("StatesDir: %v", err)
	}
	if _, err := fs.StateDir(dir, "image-1", "state-1"); err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	if _, err := fs.JobRuntimeDir(dir, "job-1"); err != nil {
		t.Fatalf("JobRuntimeDir: %v", err)
	}
	if err := fs.EnsureBaseDir(context.Background(), filepath.Join(dir, "base")); err != nil {
		t.Fatalf("EnsureBaseDir: %v", err)
	}
	if err := fs.EnsureStateDir(context.Background(), filepath.Join(dir, "state")); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}
	if _, err := fs.Clone(context.Background(), dir, filepath.Join(dir, "clone")); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := fs.Snapshot(context.Background(), dir, filepath.Join(dir, "snap")); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := fs.RemovePath(context.Background(), filepath.Join(dir, "snap")); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}

	dbms := &fakeDBMS{}
	if err := dbms.PrepareSnapshot(context.Background(), inst); err != nil {
		t.Fatalf("PrepareSnapshot: %v", err)
	}
	if err := dbms.ResumeSnapshot(context.Background(), inst); err != nil {
		t.Fatalf("ResumeSnapshot: %v", err)
	}

	psql := &fakePsqlRunner{}
	if _, err := psql.Run(context.Background(), inst, prepare.PsqlRunRequest{}); err != nil {
		t.Fatalf("Psql Run: %v", err)
	}
}
