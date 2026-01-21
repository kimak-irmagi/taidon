package httpapi

import (
	"context"
	"testing"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestFakeRuntimeExec(t *testing.T) {
	rt := &fakeRuntime{}
	if _, err := rt.Exec(context.Background(), "container-1", engineRuntime.ExecRequest{}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestFakeRuntimeWaitForReady(t *testing.T) {
	rt := &fakeRuntime{}
	if err := rt.WaitForReady(context.Background(), "container-1", time.Second); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
}
