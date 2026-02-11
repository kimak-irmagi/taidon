package dbms

import (
	"context"
	"errors"
	"testing"
	"time"

	"sqlrs/engine/internal/runtime"
)

type fakeRuntime struct {
	execCalls []runtime.ExecRequest
	execErr   error
	execFunc  func(ctx context.Context, id string, req runtime.ExecRequest) (string, error)
}

func (f *fakeRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	return nil
}

func (f *fakeRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	return imageID, nil
}

func (f *fakeRuntime) Start(ctx context.Context, req runtime.StartRequest) (runtime.Instance, error) {
	return runtime.Instance{}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRuntime) Exec(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
	f.execCalls = append(f.execCalls, req)
	if f.execFunc != nil {
		return f.execFunc(ctx, id, req)
	}
	return "", f.execErr
}

func (f *fakeRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

func TestPostgresConnectorPrepareSnapshot(t *testing.T) {
	rt := &fakeRuntime{}
	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		if len(req.Args) > 0 && req.Args[0] == "pg_ctl" && hasArgs(req.Args, "-D", runtime.PostgresDataDir) {
			for _, arg := range req.Args {
				if arg == "status" {
					return "pg_ctl: no server running\n", errors.New("exit status 3")
				}
			}
		}
		return "", nil
	}
	connector := NewPostgres(rt)
	if err := connector.PrepareSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err != nil {
		t.Fatalf("PrepareSnapshot: %v", err)
	}
	if len(rt.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(rt.execCalls))
	}
	args := rt.execCalls[0].Args
	if len(args) == 0 || args[0] != "pg_ctl" {
		t.Fatalf("unexpected exec args: %v", args)
	}
	if !hasArgs(args, "-D", runtime.PostgresDataDir) {
		t.Fatalf("expected pgdata path %q in args: %v", runtime.PostgresDataDir, args)
	}
	verifyArgs := rt.execCalls[1].Args
	if len(verifyArgs) == 0 || verifyArgs[0] != "pg_ctl" {
		t.Fatalf("unexpected verify args: %v", verifyArgs)
	}
	if !containsArg(verifyArgs, "status") {
		t.Fatalf("expected status in verify args: %v", verifyArgs)
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
	if !hasArgs(args, "-D", runtime.PostgresDataDir) {
		t.Fatalf("expected pgdata path %q in args: %v", runtime.PostgresDataDir, args)
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

func TestPostgresConnectorPrepareSnapshotVerifyFails(t *testing.T) {
	rt := &fakeRuntime{}
	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		if len(req.Args) > 0 && req.Args[0] == "pg_ctl" && containsArg(req.Args, "status") {
			return "", errors.New("pid present")
		}
		return "", nil
	}
	connector := NewPostgres(rt)
	if err := connector.PrepareSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err == nil {
		t.Fatalf("expected error")
	}
	if len(rt.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(rt.execCalls))
	}
	verifyArgs := rt.execCalls[1].Args
	if len(verifyArgs) == 0 || verifyArgs[0] != "pg_ctl" {
		t.Fatalf("unexpected verify args: %v", verifyArgs)
	}
	if !containsArg(verifyArgs, "status") {
		t.Fatalf("expected status in verify args: %v", verifyArgs)
	}
}

func hasArgs(args []string, flag string, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
