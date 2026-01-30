package prepare

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/snapshot"
	"sqlrs/engine/internal/store"
)

type PsqlRunRequest struct {
	Args    []string
	Env     map[string]string
	Stdin   *string
	WorkDir string
}

type psqlRunner interface {
	Run(ctx context.Context, instance engineRuntime.Instance, req PsqlRunRequest) (string, error)
}

type containerPsqlRunner struct {
	runtime engineRuntime.Runtime
}

func (r containerPsqlRunner) Run(ctx context.Context, instance engineRuntime.Instance, req PsqlRunRequest) (string, error) {
	if r.runtime == nil {
		return "", fmt.Errorf("runtime is required")
	}
	return r.runtime.Exec(ctx, instance.ID, engineRuntime.ExecRequest{
		User:  "postgres",
		Args:  req.Args,
		Env:   req.Env,
		Dir:   req.WorkDir,
		Stdin: req.Stdin,
	})
}

type statePaths struct {
	root      string
	engine    string
	version   string
	baseDir   string
	statesDir string
	stateDir  string
}

func resolveStatePaths(root string, imageID string, stateID string) (statePaths, error) {
	if strings.TrimSpace(root) == "" {
		return statePaths{}, fmt.Errorf("state store root is required")
	}
	engineID, version := parseImageID(imageID)
	baseDir := filepath.Join(root, "engines", engineID, version, "base")
	statesDir := filepath.Join(root, "engines", engineID, version, "states")
	stateDir := ""
	if strings.TrimSpace(stateID) != "" {
		stateDir = filepath.Join(statesDir, stateID)
	}
	return statePaths{
		root:      root,
		engine:    engineID,
		version:   version,
		baseDir:   baseDir,
		statesDir: statesDir,
		stateDir:  stateDir,
	}, nil
}

func parseImageID(imageID string) (string, string) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return "unknown", "latest"
	}
	tag := ""
	digest := ""
	if at := strings.Index(imageID, "@"); at != -1 {
		if at+1 < len(imageID) {
			digest = imageID[at+1:]
		}
		imageID = imageID[:at]
	}
	if digest == "" {
		if colon := strings.LastIndex(imageID, ":"); colon != -1 && colon > strings.LastIndex(imageID, "/") {
			tag = imageID[colon+1:]
			imageID = imageID[:colon]
		}
	} else {
		tag = digest
	}
	engine := imageID
	if slash := strings.LastIndex(engine, "/"); slash != -1 {
		engine = engine[slash+1:]
	}
	engine = sanitizeSegment(engine)
	tag = sanitizeSegment(tag)
	if tag == "" {
		tag = "latest"
	}
	return engine, tag
}

func sanitizeSegment(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if r > 127 {
			b.WriteByte('_')
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "unknown"
	}
	return out
}

func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := append([]string{}, base...)
	index := map[string]int{}
	for i, entry := range out {
		key := entry
		if idx := strings.Index(entry, "="); idx != -1 {
			key = entry[:idx]
		}
		index[normalizeEnvKey(key)] = i
	}
	for key, value := range overrides {
		entry := key + "=" + value
		if idx, ok := index[normalizeEnvKey(key)]; ok {
			out[idx] = entry
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeEnvKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func (m *Manager) executeStateTask(ctx context.Context, jobID string, prepared preparedRequest, task taskState) *ErrorResponse {
	if ctx.Err() != nil {
		return errorResponse("cancelled", "task cancelled", "")
	}
	if strings.TrimSpace(task.OutputStateID) == "" {
		return errorResponse("internal_error", "missing output state id", "")
	}
	cached, err := m.isStateCached(task.OutputStateID)
	if err != nil {
		return errorResponse("internal_error", "cannot check state cache", err.Error())
	}
	if cached {
		m.logTask(jobID, task.TaskID, "cached output_state=%s", task.OutputStateID)
		return nil
	}

	imageID := prepared.effectiveImageID()
	if strings.TrimSpace(imageID) == "" {
		return errorResponse("internal_error", "resolved image id is required", "")
	}
	paths, err := resolveStatePaths(m.stateStoreRoot, imageID, task.OutputStateID)
	if err != nil {
		return errorResponse("internal_error", "cannot resolve state paths", err.Error())
	}
	if err := os.MkdirAll(paths.statesDir, 0o700); err != nil {
		return errorResponse("internal_error", "cannot create state dir", err.Error())
	}
	if m.snapshot == nil || m.snapshot.Kind() != "btrfs" {
		if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
			return errorResponse("internal_error", "cannot create state dir", err.Error())
		}
	}

	runner, ephemeral := m.runnerForJob(jobID)
	if runner == nil {
		return errorResponse("internal_error", "job runner missing", "")
	}
	if ephemeral {
		defer m.cleanupRuntime(context.Background(), runner)
	}

	var errResp *ErrorResponse
	kind := snapshotKind(m.snapshot)
	lockPath := stateBuildLockPath(paths.stateDir, kind)
	lockErr := withStateBuildLock(ctx, paths.stateDir, lockPath, kind, func() error {
		cached, err := m.isStateCached(task.OutputStateID)
		if err != nil {
			errResp = errorResponse("internal_error", "cannot check state cache", err.Error())
			return errStateBuildFailed
		}
		if cached {
			m.logTask(jobID, task.TaskID, "cached output_state=%s", task.OutputStateID)
			return nil
		}
		if kind == "btrfs" {
			if err := m.ensureCleanBtrfsStateDir(ctx, paths.stateDir); err != nil {
				errResp = errorResponse("internal_error", "cannot reset state dir", err.Error())
				return errStateBuildFailed
			}
		} else if stateBuildMarkerExists(paths.stateDir, kind) {
			if err := resetStateDir(ctx, m.snapshot, paths.stateDir); err != nil {
				errResp = errorResponse("internal_error", "cannot reset state dir", err.Error())
				return errStateBuildFailed
			}
		}

		rt, innerResp := m.ensureRuntime(ctx, jobID, prepared, task.Input, runner)
		if innerResp != nil {
			errResp = innerResp
			return errStateBuildFailed
		}

		psqlArgs, workdir, err := buildPsqlExecArgs(prepared.normalizedArgs, rt.scriptMount)
		if err != nil {
			errResp = errorResponse("internal_error", "cannot prepare psql arguments", err.Error())
			return errStateBuildFailed
		}
		m.appendLog(jobID, "psql: start")
		var sinkCalled atomic.Bool
		psqlCtx := engineRuntime.WithLogSink(ctx, func(line string) {
			sinkCalled.Store(true)
			m.appendLog(jobID, "psql: "+line)
		})
		output, err := m.psql.Run(psqlCtx, rt.instance, PsqlRunRequest{
			Args:    psqlArgs,
			Env:     map[string]string{},
			Stdin:   prepared.request.Stdin,
			WorkDir: workdir,
		})
		if !sinkCalled.Load() && strings.TrimSpace(output) != "" {
			m.appendLogLines(jobID, "psql", output)
		}
		if err != nil {
			if ctx.Err() != nil {
				errResp = errorResponse("cancelled", "task cancelled", "")
				return errStateBuildFailed
			}
			details := strings.TrimSpace(output)
			if details == "" {
				details = err.Error()
			}
			errResp = errorResponse("internal_error", "psql execution failed", details)
			return errStateBuildFailed
		}
		if ctx.Err() != nil {
			errResp = errorResponse("cancelled", "task cancelled", "")
			return errStateBuildFailed
		}

		m.appendLog(jobID, "pg_ctl: stop for snapshot")
		pgCtx := engineRuntime.WithLogSink(ctx, func(line string) {
			m.appendLog(jobID, "pg_ctl: "+line)
		})
		if err := m.dbms.PrepareSnapshot(pgCtx, rt.instance); err != nil {
			errResp = errorResponse("internal_error", "snapshot prepare failed", err.Error())
			return errStateBuildFailed
		}
		resumed := false
		defer func() {
			if resumed {
				return
			}
			_ = m.dbms.ResumeSnapshot(context.Background(), rt.instance)
		}()

		m.appendLog(jobID, "snapshot: start")
		if err := m.snapshot.Snapshot(ctx, rt.dataDir, paths.stateDir); err != nil {
			errResp = errorResponse("internal_error", "snapshot failed", err.Error())
			return errStateBuildFailed
		}
		m.appendLog(jobID, "snapshot: complete")
		m.appendLog(jobID, "pg_ctl: start after snapshot")
		pgResumeCtx := engineRuntime.WithLogSink(ctx, func(line string) {
			m.appendLog(jobID, "pg_ctl: "+line)
		})
		if err := m.dbms.ResumeSnapshot(pgResumeCtx, rt.instance); err != nil {
			errResp = errorResponse("internal_error", "snapshot resume failed", err.Error())
			return errStateBuildFailed
		}
		resumed = true

		parentID := parentStateID(task.Input)
		createdAt := m.now().UTC().Format(time.RFC3339Nano)
		entry := store.StateCreate{
			StateID:               task.OutputStateID,
			ParentStateID:         parentID,
			StateFingerprint:      task.OutputStateID,
			ImageID:               imageID,
			PrepareKind:           prepared.request.PrepareKind,
			PrepareArgsNormalized: prepared.argsNormalized,
			CreatedAt:             createdAt,
		}
		if err := m.store.CreateState(ctx, entry); err != nil {
			if ctx.Err() != nil {
				errResp = errorResponse("cancelled", "task cancelled", "")
				return errStateBuildFailed
			}
			_ = m.snapshot.Destroy(context.Background(), paths.stateDir)
			errResp = errorResponse("internal_error", "cannot store state", err.Error())
			return errStateBuildFailed
		}
		if err := writeStateBuildMarker(paths.stateDir, kind); err != nil {
			errResp = errorResponse("internal_error", "cannot write state marker", err.Error())
			return errStateBuildFailed
		}
		return nil
	})
	if errResp != nil {
		return errResp
	}
	if lockErr != nil {
		if ctx.Err() != nil {
			return errorResponse("cancelled", "task cancelled", "")
		}
		return errorResponse("internal_error", "cannot acquire state build lock", lockErr.Error())
	}
	return nil
}

func (m *Manager) createInstance(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (*Result, *ErrorResponse) {
	if ctx.Err() != nil {
		return nil, errorResponse("cancelled", "job cancelled", "")
	}
	if strings.TrimSpace(stateID) == "" {
		return nil, errorResponse("internal_error", "state id is required", "")
	}

	runner, ephemeral := m.runnerForJob(jobID)
	if runner == nil {
		return nil, errorResponse("internal_error", "job runner missing", "")
	}
	if ephemeral {
		defer m.cleanupRuntime(context.Background(), runner)
	}

	rt := runner.getRuntime()
	if rt == nil {
		var errResp *ErrorResponse
		rt, errResp = m.startRuntime(ctx, jobID, prepared, &TaskInput{Kind: "state", ID: stateID})
		if errResp != nil {
			return nil, errResp
		}
		runner.setRuntime(rt)
	}
	if rt.instance.Host == "" || rt.instance.Port == 0 {
		return nil, errorResponse("internal_error", "runtime instance is missing connection info", "")
	}

	instanceID, err := randomHex(16)
	if err != nil {
		return nil, errorResponse("internal_error", "cannot generate instance id", err.Error())
	}
	createdAt := m.now().UTC().Format(time.RFC3339Nano)
	status := store.InstanceStatusActive
	var runtimeID *string
	imageID := prepared.effectiveImageID()
	if strings.TrimSpace(imageID) == "" {
		return nil, errorResponse("internal_error", "resolved image id is required", "")
	}
	if strings.TrimSpace(rt.instance.ID) != "" {
		runtimeID = strPtr(rt.instance.ID)
	}
	var runtimeDir *string
	if strings.TrimSpace(rt.runtimeDir) != "" {
		runtimeDir = strPtr(rt.runtimeDir)
	}
	if err := m.store.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    imageID,
		CreatedAt:  createdAt,
		RuntimeID:  runtimeID,
		RuntimeDir: runtimeDir,
		Status:     &status,
	}); err != nil {
		if ctx.Err() != nil {
			return nil, errorResponse("cancelled", "job cancelled", "")
		}
		return nil, errorResponse("internal_error", "cannot store instance", err.Error())
	}
	m.appendLog(jobID, fmt.Sprintf("instance created %s", instanceID))
	result := Result{
		DSN:                   buildDSN(rt.instance.Host, rt.instance.Port),
		InstanceID:            instanceID,
		StateID:               stateID,
		ImageID:               imageID,
		PrepareKind:           prepared.request.PrepareKind,
		PrepareArgsNormalized: prepared.argsNormalized,
	}
	return &result, nil
}

func (m *Manager) ensureRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput, runner *jobRunner) (*jobRuntime, *ErrorResponse) {
	if runner == nil {
		return nil, errorResponse("internal_error", "job runner missing", "")
	}
	if rt := runner.getRuntime(); rt != nil {
		return rt, nil
	}
	rt, errResp := m.startRuntime(ctx, jobID, prepared, input)
	if errResp != nil {
		return nil, errResp
	}
	runner.setRuntime(rt)
	return rt, nil
}

func (m *Manager) startRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput) (*jobRuntime, *ErrorResponse) {
	if ctx.Err() != nil {
		return nil, errorResponse("cancelled", "job cancelled", "")
	}
	if input == nil {
		return nil, errorResponse("internal_error", "task input is required", "")
	}

	imageID := prepared.effectiveImageID()
	if strings.TrimSpace(imageID) == "" {
		return nil, errorResponse("internal_error", "resolved image id is required", "")
	}
	ctx = engineRuntime.WithLogSink(ctx, func(line string) {
		m.appendLog(jobID, "docker: "+line)
	})
	var srcDir string
	switch input.Kind {
	case "image":
		m.appendLog(jobID, fmt.Sprintf("docker: init base %s", imageID))
		paths, err := resolveStatePaths(m.stateStoreRoot, imageID, "")
		if err != nil {
			return nil, errorResponse("internal_error", "cannot resolve state paths", err.Error())
		}
		if err := m.ensureBaseState(ctx, imageID, paths.baseDir); err != nil {
			if ctx.Err() != nil {
				return nil, errorResponse("cancelled", "job cancelled", "")
			}
			return nil, errorResponse("internal_error", "cannot initialize base state", err.Error())
		}
		srcDir = paths.baseDir
	case "state":
		if strings.TrimSpace(input.ID) == "" {
			return nil, errorResponse("internal_error", "input state id is required", "")
		}
		entry, ok, err := m.store.GetState(ctx, input.ID)
		if err != nil {
			return nil, errorResponse("internal_error", "cannot load input state", err.Error())
		}
		if !ok {
			return nil, errorResponse("internal_error", "input state not found", input.ID)
		}
		if strings.TrimSpace(entry.ImageID) != "" {
			imageID = entry.ImageID
		}
		paths, err := resolveStatePaths(m.stateStoreRoot, imageID, input.ID)
		if err != nil {
			return nil, errorResponse("internal_error", "cannot resolve state paths", err.Error())
		}
		srcDir = paths.stateDir
	default:
		return nil, errorResponse("internal_error", "unsupported task input", input.Kind)
	}

	runtimeDir := filepath.Join(m.stateStoreRoot, "jobs", jobID, "runtime")
	_ = os.RemoveAll(runtimeDir)
	if err := os.MkdirAll(filepath.Dir(runtimeDir), 0o700); err != nil {
		return nil, errorResponse("internal_error", "cannot create runtime dir", err.Error())
	}
	clone, err := m.snapshot.Clone(ctx, srcDir, runtimeDir)
	if err != nil {
		return nil, errorResponse("internal_error", "cannot clone state", err.Error())
	}

	rtScriptMount, err := scriptMountForFiles(prepared.filePaths)
	if err != nil {
		_ = clone.Cleanup()
		return nil, errorResponse("internal_error", "cannot prepare scripts", err.Error())
	}

	m.appendLog(jobID, "docker: start container")
	ctx = engineRuntime.WithLogSink(ctx, func(line string) {
		m.appendLog(jobID, "docker: "+line)
	})
	allowInitdb := strings.TrimSpace(input.Kind) == "image"
	instance, err := m.runtime.Start(ctx, engineRuntime.StartRequest{
		ImageID:      imageID,
		DataDir:      clone.MountDir,
		Name:         "sqlrs-prepare-" + jobID,
		Mounts:       runtimeMountsFrom(rtScriptMount),
		AllowInitdb:  allowInitdb,
	})
	if err != nil {
		_ = clone.Cleanup()
		if ctx.Err() != nil {
			return nil, errorResponse("cancelled", "job cancelled", "")
		}
		return nil, errorResponse("internal_error", "cannot start runtime", err.Error())
	}
	m.appendLog(jobID, fmt.Sprintf("docker: container started %s", instance.ID))
	m.logJob(jobID, "runtime started container=%s host=%s port=%d snapshot=%s", instance.ID, instance.Host, instance.Port, m.snapshot.Kind())
	m.appendLog(jobID, "docker: postgres ready")

	return &jobRuntime{
		instance:    instance,
		dataDir:     clone.MountDir,
		runtimeDir:  runtimeDir,
		cleanup:     clone.Cleanup,
		scriptMount: rtScriptMount,
	}, nil
}

func (m *Manager) ensureBaseState(ctx context.Context, imageID string, baseDir string) error {
	if strings.TrimSpace(baseDir) == "" {
		return fmt.Errorf("base dir is required")
	}
	if initMarkerExists(baseDir) {
		return nil
	}
	if err := ensureBaseDir(ctx, m.snapshot, baseDir); err != nil {
		return err
	}
	if err := withInitLock(ctx, baseDir, func() error {
		if initMarkerExists(baseDir) {
			return nil
		}
		if ok, err := hasPGVersion(baseDir); ok {
			return writeInitMarker(baseDir)
		} else if err != nil {
			return writeInitMarker(baseDir)
		}
		if err := resetBaseDirContents(baseDir); err != nil {
			return err
		}
		if err := m.runtime.InitBase(ctx, imageID, baseDir); err != nil {
			return err
		}
		return writeInitMarker(baseDir)
	}); err != nil {
		return err
	}
	return nil
}

const (
	baseInitMarkerName = ".init.ok"
	baseInitLockName   = ".init.lock"
)

func initMarkerExists(baseDir string) bool {
	_, err := os.Stat(filepath.Join(baseDir, baseInitMarkerName))
	return err == nil
}

func writeInitMarker(baseDir string) error {
	path := filepath.Join(baseDir, baseInitMarkerName)
	return os.WriteFile(path, []byte("ok"), 0o600)
}

func hasPGVersion(baseDir string) (bool, error) {
	for _, path := range pgVersionPaths(baseDir) {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			if os.IsPermission(err) {
				return false, err
			}
			return false, err
		}
	}
	return false, nil
}

func pgVersionPaths(baseDir string) []string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil
	}
	paths := []string{filepath.Join(baseDir, "PG_VERSION")}
	pgDataDir := pgDataHostDir(baseDir)
	if pgDataDir != "" && pgDataDir != baseDir {
		paths = append(paths, filepath.Join(pgDataDir, "PG_VERSION"))
	}
	return paths
}

func pgDataHostDir(baseDir string) string {
	rel := strings.TrimPrefix(engineRuntime.PostgresDataDir, engineRuntime.PostgresDataDirRoot)
	rel = strings.TrimPrefix(rel, "/")
	if strings.TrimSpace(rel) == "" {
		return baseDir
	}
	return filepath.Join(baseDir, filepath.FromSlash(rel))
}

func resetBaseDirContents(baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	keep := map[string]bool{baseInitLockName: true}
	hasOther := false
	for _, entry := range entries {
		if keep[entry.Name()] {
			continue
		}
		hasOther = true
		break
	}
	if !hasOther {
		return nil
	}
	for _, entry := range entries {
		if keep[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(baseDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func withInitLock(ctx context.Context, baseDir string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("lock callback is required")
	}
	lockPath := filepath.Join(baseDir, baseInitLockName)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if errors.Is(err, os.ErrExist) {
			if initMarkerExists(baseDir) {
				_ = os.Remove(lockPath)
				return nil
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return err
	}
}

const (
	stateBuildMarkerName = ".build.ok"
	stateBuildLockName   = ".build.lock"
)

var errStateBuildFailed = errors.New("state build failed")

func stateBuildMarkerExists(stateDir string, kind string) bool {
	_, err := os.Stat(stateBuildMarkerPath(stateDir, kind))
	return err == nil
}

func writeStateBuildMarker(stateDir string, kind string) error {
	path := stateBuildMarkerPath(stateDir, kind)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("ok"), 0o600)
}

const stateBuildLockDirName = ".build"

func snapshotKind(snap snapshot.Manager) string {
	if snap == nil {
		return ""
	}
	return snap.Kind()
}

func stateBuildMarkerPath(stateDir string, kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "btrfs" {
		base := filepath.Dir(stateDir)
		stateID := filepath.Base(stateDir)
		return filepath.Join(base, stateBuildLockDirName, stateID+".ok")
	}
	return filepath.Join(stateDir, stateBuildMarkerName)
}

func stateBuildLockPath(stateDir string, kind string) string {
	if kind == "btrfs" {
		base := filepath.Dir(stateDir)
		stateID := filepath.Base(stateDir)
		return filepath.Join(base, stateBuildLockDirName, stateID+".lock")
	}
	return filepath.Join(stateDir, stateBuildLockName)
}

func withStateBuildLock(ctx context.Context, stateDir string, lockPath string, kind string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("lock callback is required")
	}
	if strings.TrimSpace(lockPath) == "" {
		return fmt.Errorf("lock path is required")
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if errors.Is(err, os.ErrExist) || isLockBusyError(err, lockPath) {
			if stateBuildMarkerExists(stateDir, kind) {
				_ = os.Remove(lockPath)
				return nil
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return err
	}
}

func isLockBusyError(err error, lockPath string) bool {
	if err == nil || !os.IsPermission(err) {
		return false
	}
	if _, statErr := os.Stat(lockPath); statErr == nil {
		return true
	}
	return false
}

type subvolumeEnsurer interface {
	EnsureSubvolume(ctx context.Context, path string) error
}

type subvolumeChecker interface {
	IsSubvolume(ctx context.Context, path string) (bool, error)
}

func ensureBaseDir(ctx context.Context, snap snapshot.Manager, baseDir string) error {
	if snap != nil {
		if ensurer, ok := snap.(subvolumeEnsurer); ok {
			return ensurer.EnsureSubvolume(ctx, baseDir)
		}
	}
	return os.MkdirAll(baseDir, 0o700)
}

func resetStateDir(ctx context.Context, snap snapshot.Manager, stateDir string) error {
	if snap != nil && snap.Kind() == "btrfs" {
		return snap.Destroy(ctx, stateDir)
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return err
	}
	return os.MkdirAll(stateDir, 0o700)
}

func (m *Manager) ensureCleanBtrfsStateDir(ctx context.Context, stateDir string) error {
	if strings.TrimSpace(stateDir) == "" {
		return nil
	}
	info, err := os.Stat(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info == nil {
		return nil
	}
	if checker, ok := m.snapshot.(subvolumeChecker); ok {
		isSub, err := checker.IsSubvolume(ctx, stateDir)
		if err != nil {
			return err
		}
		if isSub {
			return m.snapshot.Destroy(ctx, stateDir)
		}
	}
	return os.RemoveAll(stateDir)
}

func (m *Manager) cleanupRuntime(ctx context.Context, runner *jobRunner) {
	if runner == nil {
		return
	}
	rt := runner.getRuntime()
	if rt == nil {
		return
	}
	runner.setRuntime(nil)
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := m.runtime.Stop(stopCtx, rt.instance.ID); err != nil {
		_ = m.runtime.Stop(context.Background(), rt.instance.ID)
	}
	if rt.cleanup != nil {
		_ = rt.cleanup()
	}
}

func (m *Manager) runnerForJob(jobID string) (*jobRunner, bool) {
	if strings.TrimSpace(jobID) != "" {
		if runner := m.getRunner(jobID); runner != nil {
			return runner, false
		}
	}
	return &jobRunner{}, true
}

func parentStateID(input *TaskInput) *string {
	if input == nil || input.Kind != "state" || strings.TrimSpace(input.ID) == "" {
		return nil
	}
	return strPtr(input.ID)
}
