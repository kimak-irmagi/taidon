package prepare

import (
	"context"
	"strings"
	"testing"

	engineRuntime "github.com/sqlrs/engine-local/internal/runtime"
)

func TestIfaceAddrsDefaultHelper(t *testing.T) {
	ifaces, err := listNetInterfaces()
	if err != nil {
		t.Fatalf("Interfaces: %v", err)
	}
	if len(ifaces) == 0 {
		t.Skip("no network interfaces available")
	}
	if _, err := ifaceAddrs(ifaces[0]); err != nil {
		t.Fatalf("ifaceAddrs: %v", err)
	}
}

func TestExecuteStateTaskReturnsStepLookupErrorWithEphemeralRunner(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.executor.executeStateTask(context.Background(), "job-1", prepared, taskState{
		PlanTask: PlanTask{
			TaskID:        "missing-step",
			Input:         &TaskInput{Kind: "image", ID: prepared.effectiveImageID()},
			OutputStateID: "state-1",
		},
	})
	if errResp == nil || !strings.Contains(errResp.Message, "cannot resolve psql step") {
		t.Fatalf("expected missing psql step error, got %+v", errResp)
	}
}

func TestExecutePsqlStepReturnsTaskLookupError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	runner := mgr.registerRunner("job-1", func() {})
	defer func() {
		close(runner.done)
		mgr.unregisterRunner("job-1")
	}()

	errResp := mgr.executor.executePsqlStep(context.Background(), "job-1", prepared, &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}, taskState{
		PlanTask: PlanTask{TaskID: "missing-step"},
	})
	if errResp == nil || !strings.Contains(errResp.Message, "cannot resolve psql step") {
		t.Fatalf("expected missing psql step error, got %+v", errResp)
	}
}

func TestCreateInstanceRequiresStateID(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	if _, errResp := mgr.executor.createInstance(context.Background(), "job-1", prepared, " "); errResp == nil || !strings.Contains(errResp.Message, "state id is required") {
		t.Fatalf("expected missing state id error, got %+v", errResp)
	}
}
