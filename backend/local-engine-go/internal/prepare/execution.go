package prepare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
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

	runner, ephemeral := m.runnerForJob(jobID)
	if runner == nil {
		return errorResponse("internal_error", "job runner missing", "")
	}
	if ephemeral {
		defer m.cleanupRuntime(context.Background(), runner)
	}

	rt, errResp := m.ensureRuntime(ctx, jobID, prepared, task.Input, runner)
	if errResp != nil {
		return errResp
	}

	psqlArgs, workdir, err := buildPsqlExecArgs(prepared.normalizedArgs, rt.scriptMount)
	if err != nil {
		return errorResponse("internal_error", "cannot prepare psql arguments", err.Error())
	}
	output, err := m.psql.Run(ctx, rt.instance, PsqlRunRequest{
		Args:    psqlArgs,
		Env:     map[string]string{},
		Stdin:   prepared.request.Stdin,
		WorkDir: workdir,
	})
	if err != nil {
		if ctx.Err() != nil {
			return errorResponse("cancelled", "task cancelled", "")
		}
		details := strings.TrimSpace(output)
		if details == "" {
			details = err.Error()
		}
		return errorResponse("internal_error", "psql execution failed", details)
	}
	if ctx.Err() != nil {
		return errorResponse("cancelled", "task cancelled", "")
	}

	if err := m.dbms.PrepareSnapshot(ctx, rt.instance); err != nil {
		return errorResponse("internal_error", "snapshot prepare failed", err.Error())
	}
	resumed := false
	defer func() {
		if resumed {
			return
		}
		_ = m.dbms.ResumeSnapshot(context.Background(), rt.instance)
	}()

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
	if err := m.snapshot.Snapshot(ctx, rt.dataDir, paths.stateDir); err != nil {
		return errorResponse("internal_error", "snapshot failed", err.Error())
	}
	if err := m.dbms.ResumeSnapshot(ctx, rt.instance); err != nil {
		return errorResponse("internal_error", "snapshot resume failed", err.Error())
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
			return errorResponse("cancelled", "task cancelled", "")
		}
		_ = m.snapshot.Destroy(context.Background(), paths.stateDir)
		return errorResponse("internal_error", "cannot store state", err.Error())
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
	if err := m.store.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    imageID,
		CreatedAt:  createdAt,
		RuntimeID:  runtimeID,
		Status:     &status,
	}); err != nil {
		if ctx.Err() != nil {
			return nil, errorResponse("cancelled", "job cancelled", "")
		}
		return nil, errorResponse("internal_error", "cannot store instance", err.Error())
	}
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
	var srcDir string
	switch input.Kind {
	case "image":
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

	instance, err := m.runtime.Start(ctx, engineRuntime.StartRequest{
		ImageID: imageID,
		DataDir: clone.MountDir,
		Name:    "sqlrs-prepare-" + jobID,
		Mounts:  runtimeMountsFrom(rtScriptMount),
	})
	if err != nil {
		_ = clone.Cleanup()
		if ctx.Err() != nil {
			return nil, errorResponse("cancelled", "job cancelled", "")
		}
		return nil, errorResponse("internal_error", "cannot start runtime", err.Error())
	}
	m.logJob(jobID, "runtime started container=%s host=%s port=%d snapshot=%s", instance.ID, instance.Host, instance.Port, m.snapshot.Kind())

	return &jobRuntime{
		instance:    instance,
		dataDir:     clone.MountDir,
		cleanup:     clone.Cleanup,
		scriptMount: rtScriptMount,
	}, nil
}

func (m *Manager) ensureBaseState(ctx context.Context, imageID string, baseDir string) error {
	if strings.TrimSpace(baseDir) == "" {
		return fmt.Errorf("base dir is required")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "PG_VERSION")); err == nil {
		return nil
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return err
	}
	return m.runtime.InitBase(ctx, imageID, baseDir)
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
