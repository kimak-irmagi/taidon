package prepare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/engine-local/internal/store"
)

func TestCacheExplainReturnsHitWithoutCreatingJobs(t *testing.T) {
	queueStore := newQueueStore(t)
	stateStore := &fakeStore{statesByID: map[string]store.StateEntry{}}
	mgr := newManagerWithQueue(t, stateStore, queueStore)

	scriptPath := filepath.Join(t.TempDir(), "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", scriptPath},
	}
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	outputStateID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1@sha256:resolved"})
	stateStore.statesByID[outputStateID] = store.StateEntry{
		StateID:     outputStateID,
		ImageID:     "image-1@sha256:resolved",
		PrepareKind: "psql",
	}

	result, err := mgr.CacheExplain(context.Background(), req)
	if err != nil {
		t.Fatalf("CacheExplain: %v", err)
	}
	if result.Decision != "hit" || result.ReasonCode != "exact_state_match" {
		t.Fatalf("unexpected explain result: %+v", result)
	}
	if result.MatchedStateID != outputStateID || result.Signature == "" {
		t.Fatalf("unexpected cache hit payload: %+v", result)
	}

	jobs, err := queueStore.ListJobs(context.Background(), "")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no queued jobs, got %+v", jobs)
	}
	if len(stateStore.states) != 0 || len(stateStore.instances) != 0 {
		t.Fatalf("expected no store mutations, got states=%+v instances=%+v", stateStore.states, stateStore.instances)
	}
}

func TestCacheExplainReturnsMiss(t *testing.T) {
	mgr := newManagerWithQueue(t, &fakeStore{}, newQueueStore(t))
	scriptPath := filepath.Join(t.TempDir(), "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	result, err := mgr.CacheExplain(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", scriptPath},
	})
	if err != nil {
		t.Fatalf("CacheExplain: %v", err)
	}
	if result.Decision != "miss" || result.ReasonCode != "no_matching_state" {
		t.Fatalf("unexpected explain result: %+v", result)
	}
	if result.MatchedStateID != "" || result.Signature == "" {
		t.Fatalf("unexpected miss payload: %+v", result)
	}
}

func TestCacheExplainLiquibaseUsesIsolatedTempStateRoot(t *testing.T) {
	queueStore := newQueueStore(t)
	runtime := &fakeRuntime{}
	liquibase := &fakeLiquibaseRunner{
		output: strings.Join([]string{
			"-- Changeset changelog.xml::1::dev",
			"CREATE TABLE test(id INT);",
		}, "\n"),
	}
	stateRoot := filepath.Join(t.TempDir(), "persistent-state-root")
	mgr := newManagerWithDeps(t, &fakeStore{}, queueStore, &testDeps{
		runtime:   runtime,
		liquibase: liquibase,
		stateRoot: stateRoot,
	})

	changelog := filepath.Join(t.TempDir(), "changelog.xml")
	if err := os.WriteFile(changelog, []byte("<databaseChangeLog/>"), 0o600); err != nil {
		t.Fatalf("write changelog: %v", err)
	}

	result, err := mgr.CacheExplain(context.Background(), Request{
		PrepareKind:   "lb",
		ImageID:       "image-1",
		LiquibaseArgs: []string{"update", "--changelog-file", changelog},
	})
	if err != nil {
		t.Fatalf("CacheExplain: %v", err)
	}
	if result.Decision != "miss" || result.Signature == "" {
		t.Fatalf("unexpected explain result: %+v", result)
	}
	if len(runtime.initCalls) != 1 {
		t.Fatalf("expected one base init, got %+v", runtime.initCalls)
	}
	if len(runtime.startCalls) != 1 {
		t.Fatalf("expected one runtime start, got %+v", runtime.startCalls)
	}
	if strings.HasPrefix(runtime.initCalls[0].DataDir, stateRoot) {
		t.Fatalf("base state leaked into persistent root: %q", runtime.initCalls[0].DataDir)
	}
	if strings.HasPrefix(runtime.startCalls[0].DataDir, stateRoot) {
		t.Fatalf("runtime dir leaked into persistent root: %q", runtime.startCalls[0].DataDir)
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("expected persistent root to stay untouched, stat err=%v", err)
	}

	jobs, err := queueStore.ListJobs(context.Background(), "")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no queued jobs, got %+v", jobs)
	}
}

func TestErrorFromExplainResponseKeepsInternalErrorsOutOfValidationPath(t *testing.T) {
	err := errorFromExplainResponse(&ErrorResponse{
		Code:    "internal_error",
		Message: "cannot resolve image",
		Details: "boom",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(ValidationError); ok {
		t.Fatalf("expected non-validation error, got %T", err)
	}
	if got := err.Error(); got != "cannot resolve image: boom" {
		t.Fatalf("error = %q, want %q", got, "cannot resolve image: boom")
	}
}
