package run

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/registry"
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/store/sqlite"
)

type fakeRuntime struct {
	calls         []engineRuntime.ExecRequest
	execIDs       []string
	output        []string
	errQueue      []error
	err           error
	startCalls    []engineRuntime.StartRequest
	startErr      error
	startInstance engineRuntime.Instance
}

func (f *fakeRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error { return nil }
func (f *fakeRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	return imageID, nil
}
func (f *fakeRuntime) Start(ctx context.Context, req engineRuntime.StartRequest) (engineRuntime.Instance, error) {
	f.startCalls = append(f.startCalls, req)
	if f.startErr != nil {
		return engineRuntime.Instance{}, f.startErr
	}
	if strings.TrimSpace(f.startInstance.ID) != "" {
		return f.startInstance, nil
	}
	return engineRuntime.Instance{ID: "container-2", Host: "127.0.0.1", Port: 5432}, nil
}
func (f *fakeRuntime) Stop(ctx context.Context, id string) error { return nil }
func (f *fakeRuntime) Exec(ctx context.Context, id string, req engineRuntime.ExecRequest) (string, error) {
	f.calls = append(f.calls, req)
	f.execIDs = append(f.execIDs, id)
	if len(f.output) == 0 {
		if len(f.errQueue) > 0 {
			err := f.errQueue[0]
			f.errQueue = f.errQueue[1:]
			return "", err
		}
		if f.err != nil {
			return "", f.err
		}
		return "", nil
	}
	out := f.output[0]
	f.output = f.output[1:]
	if len(f.errQueue) > 0 {
		err := f.errQueue[0]
		f.errQueue = f.errQueue[1:]
		return out, err
	}
	if f.err != nil {
		return out, f.err
	}
	return out, nil
}
func (f *fakeRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

func createInstance(t *testing.T, st store.Store, instanceID string) {
	t.Helper()
	now := timeNow()
	stateID := "state-1"
	if err := st.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-c select 1",
		CreatedAt:             now,
		StateFingerprint:      "fp-1",
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	runtimeID := "container-1"
	status := store.InstanceStatusActive
	if err := st.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    "image-1",
		CreatedAt:  now,
		RuntimeID:  &runtimeID,
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
}

func createInstanceWithRuntimeDir(t *testing.T, st store.Store, instanceID string, runtimeDir string) {
	t.Helper()
	now := timeNow()
	stateID := "state-1"
	if err := st.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-c select 1",
		CreatedAt:             now,
		StateFingerprint:      "fp-1",
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	runtimeID := "container-1"
	status := store.InstanceStatusActive
	if err := st.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    "image-1",
		CreatedAt:  now,
		RuntimeID:  &runtimeID,
		RuntimeDir: strPtr(runtimeDir),
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
}

func timeNow() string {
	return "2026-01-01T00:00:00Z"
}

func TestManagerRunStepsConcatenatesOutput(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	rt := &fakeRuntime{output: []string{"one", "two"}}
	mgr, err := NewManager(Options{
		Registry: registry.New(db),
		Runtime:  rt,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	res, err := mgr.Run(context.Background(), Request{
		InstanceRef: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:        "psql",
		Steps: []Step{
			{Args: []string{"-c", "select 1"}},
			{Args: []string{"-c", "select 2"}},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "onetwo" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if len(rt.calls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(rt.calls))
	}
	if last := rt.calls[0].Args[len(rt.calls[0].Args)-1]; !strings.Contains(last, "postgres://") {
		t.Fatalf("expected DSN arg, got %v", rt.calls[0].Args)
	}
}

func TestManagerRunStepsRejectsNonPsql(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Kind:        "pgbench",
		Steps:       []Step{{Args: []string{"-c", "select 1"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "steps") {
		t.Fatalf("expected steps error, got %v", err)
	}
}

func TestManagerRunStepsRejectsConflictingArgs(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "cccccccccccccccccccccccccccccccc")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "cccccccccccccccccccccccccccccccc",
		Kind:        "psql",
		Steps:       []Step{{Args: []string{"-h", "127.0.0.1"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestManagerRunStepsRejectsMixedArgs(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "dddddddddddddddddddddddddddddddd")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "dddddddddddddddddddddddddddddddd",
		Kind:        "psql",
		Args:        []string{"-c", "select 1"},
		Steps:       []Step{{Args: []string{"-c", "select 2"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "steps cannot be combined") {
		t.Fatalf("expected mixed args error, got %v", err)
	}
}

func TestManagerRunValidationErrors(t *testing.T) {
	db := openStore(t)
	defer db.Close()

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})

	_, err := mgr.Run(context.Background(), Request{Kind: "psql"})
	if err == nil || !strings.Contains(err.Error(), "instance_ref") {
		t.Fatalf("expected instance_ref error, got %v", err)
	}
	_, err = mgr.Run(context.Background(), Request{InstanceRef: "x", Kind: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown run kind") {
		t.Fatalf("expected kind error, got %v", err)
	}
}

func TestManagerRunMissingInstance(t *testing.T) {
	db := openStore(t)
	defer db.Close()

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{InstanceRef: "missing", Kind: "psql"})
	if err == nil || !strings.Contains(err.Error(), "instance not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestManagerRunMissingRuntimeID(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	now := timeNow()
	stateID := "state-1"
	if err := db.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-c select 1",
		CreatedAt:             now,
		StateFingerprint:      "fp-1",
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	status := store.InstanceStatusActive
	if err := db.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		StateID:    stateID,
		ImageID:    "image-1",
		CreatedAt:  now,
		RuntimeID:  nil,
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{InstanceRef: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Kind: "psql"})
	if err == nil || !strings.Contains(err.Error(), "runtime id is missing") {
		t.Fatalf("expected runtime id error, got %v", err)
	}
}

func TestManagerRunPgbenchConflictArgs(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "ffffffffffffffffffffffffffffffff")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "ffffffffffffffffffffffffffffffff",
		Kind:        "pgbench",
		Args:        []string{"-h", "127.0.0.1"},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting pgbench") {
		t.Fatalf("expected pgbench conflict, got %v", err)
	}
}

func TestManagerRunCommandOverrideAndStdin(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "abababababababababababababababab")

	stdin := "select 1;"
	rt := &fakeRuntime{output: []string{"ok"}}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "abababababababababababababababab",
		Kind:        "psql",
		Command:     strPtr("custom-psql"),
		Args:        []string{"-c", "select 1"},
		Stdin:       &stdin,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rt.calls) != 1 || rt.calls[0].Args[0] != "custom-psql" {
		t.Fatalf("unexpected exec args: %+v", rt.calls)
	}
	if rt.calls[0].Stdin == nil || *rt.calls[0].Stdin != "select 1;" {
		t.Fatalf("unexpected stdin: %+v", rt.calls[0].Stdin)
	}
}

func TestManagerRunStepsRejectsStdin(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "abababababababababababababababac")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	stdin := "data"
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "abababababababababababababababac",
		Kind:        "psql",
		Stdin:       &stdin,
		Steps:       []Step{{Args: []string{"-c", "select 1"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "steps cannot be combined with stdin") {
		t.Fatalf("expected stdin conflict, got %v", err)
	}
}

func TestManagerRunPgbenchExecArgs(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbc")

	rt := &fakeRuntime{output: []string{"ok"}}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbc",
		Kind:        "pgbench",
		Args:        []string{"-c", "10"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("expected exec call")
	}
	args := rt.calls[0].Args
	if len(args) < 7 || args[0] != "pgbench" || args[1] != "-h" {
		t.Fatalf("unexpected pgbench args: %v", args)
	}
}

func TestManagerRunPsqlConflictArgs(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbd")

	rt := &fakeRuntime{}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbd",
		Kind:        "psql",
		Args:        []string{"-h", "127.0.0.1"},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting psql") {
		t.Fatalf("expected psql conflict, got %v", err)
	}
}

func TestManagerRunExecError(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	createInstance(t, db, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbe")

	rt := &fakeRuntime{output: []string{"out"}, err: errors.New("boom")}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbe",
		Kind:        "psql",
		Args:        []string{"-c", "select 1"},
	})
	if err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected exec failed, got %v", err)
	}
}

func TestManagerRunRecreatesMissingContainerAndUpdatesRuntimeID(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	merged := filepath.Join(runtimeDir, "merged")
	if err := os.MkdirAll(merged, 0o700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	createInstanceWithRuntimeDir(t, db, "dddddddddddddddddddddddddddddddd", runtimeDir)

	rt := &fakeRuntime{
		output:        []string{"", "ok"},
		errQueue:      []error{errors.New("Error response from daemon: No such container: container-1")},
		startInstance: engineRuntime.Instance{ID: "container-2", Host: "127.0.0.1", Port: 5432},
	}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	res, err := mgr.Run(context.Background(), Request{
		InstanceRef: "dddddddddddddddddddddddddddddddd",
		Kind:        "psql",
		Args:        []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "ok" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if len(rt.startCalls) != 1 {
		t.Fatalf("expected start call, got %d", len(rt.startCalls))
	}
	if rt.startCalls[0].DataDir != merged {
		t.Fatalf("unexpected data dir: %s", rt.startCalls[0].DataDir)
	}
	entry, ok, err := db.GetInstance(context.Background(), "dddddddddddddddddddddddddddddddd")
	if err != nil || !ok {
		t.Fatalf("GetInstance: ok=%v err=%v", ok, err)
	}
	if entry.RuntimeID == nil || *entry.RuntimeID != "container-2" {
		t.Fatalf("expected runtime id updated, got %+v", entry.RuntimeID)
	}
	if len(res.Events) != 3 {
		t.Fatalf("expected 3 recovery events, got %+v", res.Events)
	}
	if res.Events[0].Type != "log" || res.Events[0].Data != "run: container missing - recreating" {
		t.Fatalf("unexpected event 0: %+v", res.Events[0])
	}
	if res.Events[1].Type != "log" || res.Events[1].Data != "run: restoring runtime" {
		t.Fatalf("unexpected event 1: %+v", res.Events[1])
	}
	if res.Events[2].Type != "log" || res.Events[2].Data != "run: container started" {
		t.Fatalf("unexpected event 2: %+v", res.Events[2])
	}
}

func TestManagerRunRecreateFailsWhenRuntimeDirMissing(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	now := timeNow()
	stateID := "state-1"
	if err := db.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-c select 1",
		CreatedAt:             now,
		StateFingerprint:      "fp-1",
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	runtimeID := "container-1"
	status := store.InstanceStatusActive
	if err := db.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		StateID:    stateID,
		ImageID:    "image-1",
		CreatedAt:  now,
		RuntimeID:  &runtimeID,
		RuntimeDir: nil,
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	rt := &fakeRuntime{
		errQueue: []error{errors.New("No such container: container-1")},
	}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Kind:        "psql",
		Args:        []string{"-c", "select 1"},
	})
	if err == nil || !strings.Contains(err.Error(), "runtime_dir") {
		t.Fatalf("expected runtime_dir error, got %v", err)
	}
	if len(rt.startCalls) != 0 {
		t.Fatalf("expected no start calls, got %d", len(rt.startCalls))
	}
}

func TestManagerRunRecreateFailsWhenStartFails(t *testing.T) {
	db := openStore(t)
	defer db.Close()
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	createInstanceWithRuntimeDir(t, db, "ffffffffffffffffffffffffffffffff", runtimeDir)

	rt := &fakeRuntime{
		errQueue: []error{errors.New("No such container: container-1")},
		startErr: errors.New("start failed"),
	}
	mgr, _ := NewManager(Options{Registry: registry.New(db), Runtime: rt})
	_, err := mgr.Run(context.Background(), Request{
		InstanceRef: "ffffffffffffffffffffffffffffffff",
		Kind:        "psql",
		Args:        []string{"-c", "select 1"},
	})
	if err == nil || !strings.Contains(err.Error(), "runtime start failed") {
		t.Fatalf("expected start failed error, got %v", err)
	}
}

func openStore(t *testing.T) *sqlite.Store {
	t.Helper()
	path := t.TempDir() + "/state.db"
	st, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	return st
}
