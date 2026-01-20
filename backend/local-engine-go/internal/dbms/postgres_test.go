package dbms

import (
	"context"
	"testing"
	"time"

	"sqlrs/engine/internal/runtime"
)

type fakeRuntime struct {
	execCalls []runtime.ExecRequest
	execErr   error
}

func (f *fakeRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	return nil
}

func (f *fakeRuntime) Start(ctx context.Context, req runtime.StartRequest) (runtime.Instance, error) {
	return runtime.Instance{}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRuntime) Exec(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
	f.execCalls = append(f.execCalls, req)
	return "", f.execErr
}

func (f *fakeRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

func TestPostgresConnectorPrepareSnapshot(t *testing.T) {
	rt := &fakeRuntime{}
	connector := NewPostgres(rt)
	if err := connector.PrepareSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err != nil {
		t.Fatalf("PrepareSnapshot: %v", err)
	}
	if len(rt.execCalls) != 1 {
		t.Fatalf("expected exec call, got %d", len(rt.execCalls))
	}
	args := rt.execCalls[0].Args
	if len(args) == 0 || args[0] != "pg_ctl" {
		t.Fatalf("unexpected exec args: %v", args)
	}
}

func TestPostgresConnectorResumeSnapshot(t *testing.T) {
	rt := &fakeRuntime{}
	connector := NewPostgres(rt)
	if err := connector.ResumeSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err != nil {
		t.Fatalf("ResumeSnapshot: %v", err)
	}
	if len(rt.execCalls) != 1 {
		t.Fatalf("expected exec call, got %d", len(rt.execCalls))
	}
	args := rt.execCalls[0].Args
	if len(args) == 0 || args[0] != "pg_ctl" {
		t.Fatalf("unexpected exec args: %v", args)
	}
}

func TestPostgresConnectorRequiresRuntime(t *testing.T) {
	connector := &PostgresConnector{}
	if err := connector.PrepareSnapshot(context.Background(), runtime.Instance{}); err == nil {
		t.Fatalf("expected error without runtime")
	}
	if err := connector.ResumeSnapshot(context.Background(), runtime.Instance{}); err == nil {
		t.Fatalf("expected error without runtime")
	}
}
