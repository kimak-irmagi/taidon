package prepare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
)

type cancelOnContextPsqlRunner struct{}

func (cancelOnContextPsqlRunner) Run(ctx context.Context, instance engineRuntime.Instance, req PsqlRunRequest) (string, error) {
	<-ctx.Done()
	return "", errors.New("psql failed after cancel")
}

type cancelOnContextLiquibaseRunner struct{}

func (cancelOnContextLiquibaseRunner) Run(ctx context.Context, req LiquibaseRunRequest) (string, error) {
	<-ctx.Done()
	return "", errors.New("liquibase failed after cancel")
}

type cancelInitRuntime struct {
	fakeRuntime
}

func (r *cancelInitRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	<-ctx.Done()
	return ctx.Err()
}

type stateFlapStore struct {
	*fakeStore
	getStateCalls int
	failAfter     int
	failErr       error
}

func (s *stateFlapStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	s.getStateCalls++
	if s.failAfter > 0 && s.getStateCalls > s.failAfter {
		return store.StateEntry{}, false, s.failErr
	}
	return s.fakeStore.GetState(ctx, stateID)
}

func TestExecuteStateTaskReturnsCacheErrorWhenStoreGetFails(t *testing.T) {
	mgr := newManager(t, &fakeStore{getStateErr: errors.New("boom")})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	prepared := preparedRequest{request: Request{PrepareKind: "custom", ImageID: "image-1"}}
	_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot check state cache") {
		t.Fatalf("expected state cache error, got %+v", errResp)
	}
}

func TestExecuteStateTaskCachedUnknownKindReturnsImmediately(t *testing.T) {
	st := &fakeStore{
		statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1"},
		},
	}
	mgr := newManagerWithStateFS(t, st, &fakeStateFS{})
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	prepared := preparedRequest{request: Request{PrepareKind: "custom", ImageID: "image-1"}}
	got, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp != nil {
		t.Fatalf("unexpected executeStateTask error: %+v", errResp)
	}
	if got != "state-1" {
		t.Fatalf("expected cached state-1, got %q", got)
	}
}

func TestExecuteStateTaskCachedStateMissingPGVersionInvalidatesAndRebuilds(t *testing.T) {
	st := &fakeStore{}
	mgr := newManagerWithStateFS(t, st, &fakeStateFS{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	st.statesByID = map[string]store.StateEntry{
		outputID: {StateID: outputID, ImageID: "image-1"},
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	got, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if got != outputID {
		t.Fatalf("expected %q, got %q", outputID, got)
	}
	if len(st.deletedStates) == 0 || st.deletedStates[0] != outputID {
		t.Fatalf("expected cached state invalidation, got %+v", st.deletedStates)
	}
	if _, ok := st.statesByID[outputID]; !ok {
		t.Fatalf("expected state %q to be recreated", outputID)
	}
}

func TestExecuteStateTaskCachedStatePGVersionRuntimeFailureRebuildsWithoutInvalidation(t *testing.T) {
	mountDir := filepath.Join(t.TempDir(), "runtime-mount")
	if err := os.MkdirAll(mountDir, 0o700); err != nil {
		t.Fatalf("mkdir mount dir: %v", err)
	}
	fs := &fakeStateFS{mountDir: mountDir}
	st := &fakeStore{}
	mgr := newManagerWithStateFS(t, st, fs)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	st.statesByID = map[string]store.StateEntry{
		outputID: {StateID: outputID, ImageID: "image-1"},
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	got, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if got != outputID {
		t.Fatalf("expected %q, got %q", outputID, got)
	}
	if len(st.deletedStates) != 0 {
		t.Fatalf("expected no cached state invalidation, got %+v", st.deletedStates)
	}
	if len(fs.snapshotCalls) != 1 {
		t.Fatalf("expected forced rebuild snapshot, got %+v", fs.snapshotCalls)
	}
	if len(st.states) != 1 {
		t.Fatalf("expected rebuilt state to be stored, got %+v", st.states)
	}
}

func TestExecuteStateTaskCachedStateNonRecoverableStartErrorReturnsOriginal(t *testing.T) {
	fs := &fakeStateFS{cloneErr: errors.New("clone failed")}
	st := &fakeStore{}
	mgr := newManagerWithStateFS(t, st, fs)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	st.statesByID = map[string]store.StateEntry{
		outputID: {StateID: outputID, ImageID: "image-1"},
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot clone state") {
		t.Fatalf("expected clone error, got %+v", errResp)
	}
}

func TestExecuteStateTaskReportsLockAcquireFailure(t *testing.T) {
	store := &fakeStore{}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	lockPath := stateBuildLockPath(paths.stateDir, snapshotKind(mgr.statefs))
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("mkdir lock path as dir: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot acquire state build lock") {
		t.Fatalf("expected lock acquire error, got %+v", errResp)
	}
}

func TestStartRuntimeReturnsCannotCreateRuntimeDirWhenJobsPathIsFile(t *testing.T) {
	stateRoot := t.TempDir()
	jobsPath := filepath.Join(stateRoot, "jobs")
	if err := os.WriteFile(jobsPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write jobs file: %v", err)
	}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: stateRoot,
		runtime:   &fakeRuntime{},
		statefs:   &fakeStateFS{},
	})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || !strings.Contains(errResp.Message, "cannot create runtime dir") {
		t.Fatalf("expected runtime dir error, got %+v", errResp)
	}
}

func TestStartRuntimeRenamesRuntimeDirWhenRemoveFails(t *testing.T) {
	stateRoot := t.TempDir()
	runtimeDir := filepath.Join(stateRoot, "jobs", "job-1", "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "stale.txt"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	originalRemoveAll := removeAllFn
	removeAllFn = func(path string) error {
		if path == runtimeDir {
			return errors.New("permission denied")
		}
		return os.RemoveAll(path)
	}
	t.Cleanup(func() {
		removeAllFn = originalRemoveAll
	})

	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: stateRoot,
		runtime:   &fakeRuntime{},
		statefs:   &fakeStateFS{},
	})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp != nil {
		t.Fatalf("expected runtime start after stale dir rename, got %+v", errResp)
	}

	renamed, err := filepath.Glob(runtimeDir + ".stale-*")
	if err != nil {
		t.Fatalf("glob stale runtime dir: %v", err)
	}
	if len(renamed) != 1 {
		t.Fatalf("expected one renamed stale runtime dir, got %d", len(renamed))
	}
	if _, err := os.Stat(filepath.Join(renamed[0], "stale.txt")); err != nil {
		t.Fatalf("expected stale file in renamed dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removed from runtime dir")
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "PG_VERSION")); err != nil {
		t.Fatalf("expected cloned runtime pg_version: %v", err)
	}
}

func TestStartRuntimeReturnsDirtyRuntimeDataDirWhenCloneMountHasPostmasterPID(t *testing.T) {
	mountDir := filepath.Join(t.TempDir(), "runtime-mount")
	if err := os.MkdirAll(mountDir, 0o700); err != nil {
		t.Fatalf("mkdir mount dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mountDir, "postmaster.pid"), []byte("1"), 0o600); err != nil {
		t.Fatalf("write postmaster.pid: %v", err)
	}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		runtime: &fakeRuntime{},
		statefs: &fakeStateFS{mountDir: mountDir},
	})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || !strings.Contains(errResp.Message, "runtime data dir is dirty") {
		t.Fatalf("expected dirty runtime dir error, got %+v", errResp)
	}
}

func TestStartRuntimeReturnsInspectRuntimePGVersionError(t *testing.T) {
	stateRoot := t.TempDir()
	store := &fakeStore{
		statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1"},
		},
	}
	mgr := newManagerWithDeps(t, store, newQueueStore(t), &testDeps{
		stateRoot: stateRoot,
		runtime:   &fakeRuntime{},
		statefs:   &fakeStateFS{mountDir: "bad\x00path"},
	})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	paths, err := resolveStatePaths(stateRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: "state-1"})
	if errResp == nil || !strings.Contains(errResp.Message, "cannot inspect runtime PG_VERSION") {
		t.Fatalf("expected runtime PG_VERSION inspect error, got %+v", errResp)
	}
}

func TestExecutePsqlStepReturnsCancelledWhenRunFailsAfterCancel(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		psql: &cancelOnContextPsqlRunner{},
	})
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1"}}
	prepared := preparedRequest{
		request:        Request{PrepareKind: "psql"},
		normalizedArgs: []string{"-c", "select 1"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	errResp := mgr.executePsqlStep(ctx, "job-1", prepared, rt, taskState{})
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestExecuteLiquibaseStepReturnsCancelledWhenRunFailsAfterCancel(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		liquibase: &cancelOnContextLiquibaseRunner{},
	})
	rt := &jobRuntime{
		instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432},
	}
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb"},
		normalizedArgs: []string{"update"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	errResp := mgr.executeLiquibaseStep(ctx, "job-1", prepared, rt, taskState{})
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestStartRuntimeReturnsCancelledWhenInitBaseCancelled(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state-store")
	mgr, err := NewPrepareService(Options{
		Store:          &fakeStore{},
		Queue:          newQueueStore(t),
		Runtime:        &cancelInitRuntime{},
		StateFS:        &fakeStateFS{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("NewPrepareService: %v", err)
	}
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, errResp := mgr.startRuntime(ctx, "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestCreateInstanceReturnsRuntimeConnectionInfoError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), nil)
	runner := mgr.registerRunner("job-1", func() {})
	runner.setRuntime(&jobRuntime{instance: engineRuntime.Instance{ID: "container-1"}})
	t.Cleanup(func() { mgr.unregisterRunner("job-1") })

	prepared := preparedRequest{request: Request{PrepareKind: "psql", ImageID: "image-1"}}
	_, errResp := mgr.createInstance(context.Background(), "job-1", prepared, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "runtime instance is missing connection info") {
		t.Fatalf("expected runtime connection info error, got %+v", errResp)
	}
}

type failStateRuntimeStart struct {
	fakeRuntime
}

func (r *failStateRuntimeStart) Start(ctx context.Context, req engineRuntime.StartRequest) (engineRuntime.Instance, error) {
	r.startCalls = append(r.startCalls, req)
	if !req.AllowInitdb {
		return engineRuntime.Instance{}, errors.New("runtime data dir missing PG_VERSION")
	}
	instance := r.instance
	if !r.noDefaults {
		if instance.ID == "" {
			instance.ID = "container-1"
		}
		if instance.Host == "" {
			instance.Host = "127.0.0.1"
		}
		if instance.Port == 0 {
			instance.Port = 5432
		}
	}
	return instance, nil
}

func TestExecuteStateTaskLiquibaseCachedStateRuntimeFailureRebuildsWithFreshRuntime(t *testing.T) {
	fs := &fakeStateFS{}
	st := &fakeStore{}
	runtime := &failStateRuntimeStart{}
	liquibaseOutput := strings.Join([]string{
		"-- Changeset config/liquibase/master.xml::1::dev",
		"CREATE TABLE test(id INT);",
	}, "\n")
	liquibase := &fakeLiquibaseRunner{output: liquibaseOutput}

	mgr := newManagerWithStateFS(t, st, fs)
	mgr.runtime = runtime
	mgr.liquibase = liquibase

	workspace := t.TempDir()
	changelog := filepath.Join(workspace, "config", "liquibase", "master.xml")
	if err := os.MkdirAll(filepath.Dir(changelog), 0o700); err != nil {
		t.Fatalf("mkdir changelog dir: %v", err)
	}
	if err := os.WriteFile(changelog, []byte("<databaseChangeLog/>"), 0o600); err != nil {
		t.Fatalf("write changelog: %v", err)
	}

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind:   "lb",
		ImageID:       "image-1",
		WorkDir:       workspace,
		LiquibaseArgs: []string{"update", "--changelog-file", changelog},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	changesets, err := parseLiquibaseUpdateSQL(liquibaseOutput)
	if err != nil {
		t.Fatalf("parseLiquibaseUpdateSQL: %v", err)
	}
	if len(changesets) == 0 {
		t.Fatalf("expected changesets")
	}
	taskHash := liquibaseFingerprint("image-1", []LiquibaseChangeset{changesets[0]})
	outputID, errResp := mgr.computeOutputStateID("image", "image-1", taskHash)
	if errResp != nil {
		t.Fatalf("computeOutputStateID: %+v", errResp)
	}

	st.statesByID = map[string]store.StateEntry{
		outputID: {StateID: outputID, ImageID: "image-1"},
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}

	task := taskState{PlanTask: PlanTask{
		TaskID:        "execute-0",
		Type:          "state_execute",
		OutputStateID: outputID,
		Input:         &TaskInput{Kind: "image", ID: "image-1"},
	}}
	got, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if got != outputID {
		t.Fatalf("expected %q, got %q", outputID, got)
	}
	if len(runtime.startCalls) != 3 {
		t.Fatalf("expected 3 runtime starts (image/state/image), got %+v", runtime.startCalls)
	}
	if !runtime.startCalls[0].AllowInitdb || runtime.startCalls[1].AllowInitdb || !runtime.startCalls[2].AllowInitdb {
		t.Fatalf("unexpected runtime start sequence: %+v", runtime.startCalls)
	}
	if len(st.states) != 1 {
		t.Fatalf("expected rebuilt state to be stored, got %+v", st.states)
	}
}
