package dbms

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sqlrs/engine-local/internal/runtime"
)

func TestPostgresConnectorPrepareSnapshotReturnsRelaxPermissionsError(t *testing.T) {
	rt := &fakeRuntime{}
	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		if containsArg(req.Args, "status") {
			return "pg_ctl: no server running\n", errors.New("exit status 3")
		}
		if len(req.Args) > 0 && req.Args[0] == "chmod" {
			return "", errors.New("chmod boom")
		}
		return "", nil
	}
	connector := NewPostgres(rt)

	if err := connector.PrepareSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "cannot relax postgres data permissions") {
		t.Fatalf("expected relax permissions error, got %v", err)
	}
}

func TestPostgresConnectorResumeSnapshotReturnsHardenPermissionsError(t *testing.T) {
	rt := &fakeRuntime{}
	rt.execFunc = func(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
		if len(req.Args) > 0 && req.Args[0] == "chmod" {
			return "", errors.New("chmod boom")
		}
		return "", nil
	}
	connector := NewPostgres(rt)

	if err := connector.ResumeSnapshot(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "cannot harden postgres data permissions") {
		t.Fatalf("expected harden permissions error, got %v", err)
	}
}

func TestPostgresConnectorPermissionHelpersReturnWrappedErrors(t *testing.T) {
	rt := &fakeRuntime{execErr: errors.New("chmod boom")}
	connector := NewPostgres(rt)

	if err := connector.relaxHostSnapshotAccess(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "cannot relax postgres data permissions") {
		t.Fatalf("expected wrapped relax error, got %v", err)
	}
	if err := connector.hardenDataDirPermissions(context.Background(), runtime.Instance{ID: "c1"}); err == nil || !strings.Contains(err.Error(), "cannot harden postgres data permissions") {
		t.Fatalf("expected wrapped harden error, got %v", err)
	}
}
