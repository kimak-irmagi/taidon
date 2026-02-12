package prepare

import (
	"context"
	"errors"
	"testing"

	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare/queue"
)

type stringerVal struct{}

func (stringerVal) String() string { return "INFO" }

func TestNewManagerDefaults(t *testing.T) {
	mgr, err := NewManager(Options{
		Store:          &fakeStore{},
		Queue:          newQueueStore(t),
		Runtime:        &fakeRuntime{},
		StateFS:        &fakeStateFS{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr.psql == nil || mgr.liquibase == nil || mgr.now == nil || mgr.idGen == nil || mgr.validateStore == nil {
		t.Fatalf("expected defaults to be set")
	}
}

func TestRecoverQueueError(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		listJobsByStatus: func(context.Context, []string) ([]queue.JobRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if err := mgr.Recover(context.Background()); err == nil {
		t.Fatalf("expected recover error")
	}
}

func TestSubmitIDGenError(t *testing.T) {
	mgr, err := NewManager(Options{
		Store:          &fakeStore{},
		Queue:          newQueueStore(t),
		Runtime:        &fakeRuntime{},
		StateFS:        &fakeStateFS{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: t.TempDir(),
		IDGen: func() (string, error) {
			return "", errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err == nil {
		t.Fatalf("expected submit error")
	}
}

func TestGetQueueErrors(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{}, false, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if _, ok := mgr.Get("job-1"); ok {
		t.Fatalf("expected missing job")
	}
}

func TestGetListTasksError(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{JobID: "job-1", Status: StatusQueued, PrepareKind: "psql", ImageID: "image-1", CreatedAt: "2026-01-01T00:00:00Z"}, true, nil
		},
		listTasks: func(context.Context, string) ([]queue.TaskRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	status, ok := mgr.Get("job-1")
	if !ok || status.JobID != "job-1" {
		t.Fatalf("expected job status, got %+v ok=%v", status, ok)
	}
	if len(status.Tasks) != 0 {
		t.Fatalf("expected empty tasks on error")
	}
}

func TestDeleteBlockedTask(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{JobID: "job-1"}, true, nil
		},
		listTasks: func(context.Context, string) ([]queue.TaskRecord, error) {
			return []queue.TaskRecord{{Status: StatusRunning}}, nil
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	result, ok := mgr.Delete("job-1", deletion.DeleteOptions{})
	if !ok || result.Outcome != deletion.OutcomeBlocked {
		t.Fatalf("expected blocked delete, got %+v ok=%v", result, ok)
	}
}

func TestConfigValueToStringAndLogLevel(t *testing.T) {
	if out, ok := configValueToString("debug"); !ok || out != "debug" {
		t.Fatalf("unexpected string conversion: %q", out)
	}
	if out, ok := configValueToString([]byte("warn")); !ok || out != "warn" {
		t.Fatalf("unexpected byte conversion: %q", out)
	}
	if out, ok := configValueToString(stringerVal{}); !ok || out != "INFO" {
		t.Fatalf("unexpected stringer conversion: %q", out)
	}
	if _, ok := configValueToString(123); ok {
		t.Fatalf("expected unsupported type")
	}

	level := logLevelFromConfig(&fakeConfigStore{value: "  INFO  "})
	if level != "info" {
		t.Fatalf("unexpected log level: %q", level)
	}
	if logLevelFromConfig(&fakeConfigStore{value: ""}) != "debug" {
		t.Fatalf("expected default log level")
	}
	if logLevelFromConfig(&fakeConfigStore{value: 123}) != "debug" {
		t.Fatalf("expected default log level for non-string")
	}
	if logLevelFromConfig(&fakeConfigStore{err: errors.New("boom")}) != "debug" {
		t.Fatalf("expected default log level on error")
	}
	if logLevelFromConfig(nil) != "debug" {
		t.Fatalf("expected default log level for nil config")
	}
}

func TestLogLevelAllowsInfo(t *testing.T) {
	if !logLevelAllowsInfo("debug") || !logLevelAllowsInfo("info") {
		t.Fatalf("expected debug/info to allow")
	}
	if logLevelAllowsInfo("warn") {
		t.Fatalf("expected warn to disallow")
	}
}

func TestLogInfoJobRespectsLevel(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	mgr.config = &fakeConfigStore{value: "error"}
	mgr.logInfoJob("job-1", "should not log")
	mgr.config = &fakeConfigStore{value: "info"}
	mgr.logInfoJob("job-1", "should log")
}
