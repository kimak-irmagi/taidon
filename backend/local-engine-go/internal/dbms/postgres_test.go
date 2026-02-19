package dbms

import (
	"context"
	"errors"
	"strings"
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
	if len(rt.execCalls) != 3 {
		t.Fatalf("expected 3 exec calls, got %d", len(rt.execCalls))
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
	chmodArgs := rt.execCalls[2].Args
	if len(chmodArgs) == 0 || chmodArgs[0] != "chmod" {
		t.Fatalf("unexpected chmod args: %v", chmodArgs)
	}
	if !containsArg(chmodArgs, "a+rX") {
		t.Fatalf("expected a+rX chmod args: %v", chmodArgs)
	}
}

func TestPostgresConnectorResumeSnapshot(t *testing.T) {
	rt := &fakeRuntime{}
	connector := NewPostgres(rt)
	if err := connector.ResumeSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err != nil {
		t.Fatalf("ResumeSnapshot: %v", err)
	}
	if len(rt.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(rt.execCalls))
	}
	chmodArgs := rt.execCalls[0].Args
	if len(chmodArgs) == 0 || chmodArgs[0] != "chmod" {
		t.Fatalf("unexpected harden args: %v", chmodArgs)
	}
	if !containsArg(chmodArgs, "0700") {
		t.Fatalf("expected 0700 chmod args: %v", chmodArgs)
	}
	args := rt.execCalls[1].Args
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

func TestPostgresConnectorLogInfoEnabled(t *testing.T) {
	connector := &PostgresConnector{}
	if !connector.logInfoEnabled() {
		t.Fatalf("expected default log level to be enabled")
	}
	connector = NewPostgres(&fakeRuntime{}, WithLogLevel(func() string { return "warn" }))
	if connector.logInfoEnabled() {
		t.Fatalf("expected warn to disable info logging")
	}
	connector = NewPostgres(&fakeRuntime{}, WithLogLevel(func() string { return "INFO" }))
	if !connector.logInfoEnabled() {
		t.Fatalf("expected info to enable logging")
	}
}

func TestPostgresConnectorVerifyStoppedBranches(t *testing.T) {
	rt := &fakeRuntime{}
	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		return "", nil
	}
	connector := NewPostgres(rt)
	if err := connector.verifyStopped(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "pg_ctl status returned running") {
		t.Fatalf("expected running error, got %v", err)
	}

	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		return "pg_ctl: no server running\n", errors.New("exit status 3")
	}
	if err := connector.verifyStopped(context.Background(), runtime.Instance{ID: "c1"}); err != nil {
		t.Fatalf("expected no server running to succeed, got %v", err)
	}

	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		return "", errors.New("boom")
	}
	if err := connector.verifyStopped(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include exec error, got %v", err)
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
