package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare"
	"github.com/sqlrs/engine-local/internal/prepare/queue"
	"github.com/sqlrs/engine-local/internal/registry"
	"github.com/sqlrs/engine-local/internal/store/sqlite"
)

func TestPrepareJobsListIncludesPrepareArgsAndResolvedImageID(t *testing.T) {
	server, cleanup, queueStore := newListSurfaceServer(t)
	defer cleanup()

	args := "-f prepare.sql -X -v ON_ERROR_STOP=1"
	reqJSON := mustMarshalPrepareRequest(t, prepare.Request{
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PsqlArgs:    []string{"-f", "prepare.sql", "-X", "-v", "ON_ERROR_STOP=1"},
		PlanOnly:    true,
	})
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:                 "job-1",
		Status:                prepare.StatusSucceeded,
		PrepareKind:           "psql",
		ImageID:               "postgres:17",
		PlanOnly:              true,
		PrepareArgsNormalized: strPtr(args),
		RequestJSON:           strPtr(reqJSON),
		CreatedAt:             "2026-03-10T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:           "job-1",
			TaskID:          "resolve-image",
			Position:        0,
			Type:            "resolve_image",
			Status:          prepare.StatusSucceeded,
			ImageID:         strPtr("postgres:17"),
			ResolvedImageID: strPtr("postgres@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var entries []prepare.JobEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one job entry, got %+v", entries)
	}
	if entries[0].PrepareArgsNormalized != args {
		t.Fatalf("expected prepare args %q, got %+v", args, entries[0])
	}
	if entries[0].ResolvedImageID == "" {
		t.Fatalf("expected resolved image id, got %+v", entries[0])
	}
}

func TestPrepareJobsListLeavesResolvedImageIDEmptyWithoutTaskMetadata(t *testing.T) {
	server, cleanup, queueStore := newListSurfaceServer(t)
	defer cleanup()

	reqJSON := mustMarshalPrepareRequest(t, prepare.Request{
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      prepare.StatusSucceeded,
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PlanOnly:    true,
		RequestJSON: strPtr(reqJSON),
		CreatedAt:   "2026-03-10T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	var entries []prepare.JobEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one job entry, got %+v", entries)
	}
	if entries[0].ResolvedImageID != "" {
		t.Fatalf("expected empty resolved image id, got %+v", entries[0])
	}
	if entries[0].ImageID != "postgres:17" {
		t.Fatalf("expected requested image id preserved, got %+v", entries[0])
	}
}

func TestTasksListIncludesArgsSummaryAndLiquibaseChangesetFields(t *testing.T) {
	server, cleanup, queueStore := newListSurfaceServer(t)
	defer cleanup()

	psqlReqJSON := mustMarshalPrepareRequest(t, prepare.Request{
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PsqlArgs:    []string{"-f", "prepare.sql", "-c", "select 1"},
		PlanOnly:    true,
	})
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-psql",
		Status:      prepare.StatusSucceeded,
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PlanOnly:    true,
		RequestJSON: strPtr(psqlReqJSON),
		CreatedAt:   "2026-03-10T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob psql: %v", err)
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-psql", []queue.TaskRecord{
		{
			JobID:         "job-psql",
			TaskID:        "execute-1",
			Position:      1,
			Type:          "state_execute",
			Status:        prepare.StatusSucceeded,
			InputKind:     strPtr("state"),
			InputID:       strPtr("state-1"),
			OutputStateID: strPtr("state-2"),
		},
		{
			JobID:    "job-psql",
			TaskID:   "prepare-instance",
			Position: 2,
			Type:     "prepare_instance",
			Status:   prepare.StatusSucceeded,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks psql: %v", err)
	}

	lbReqJSON := mustMarshalPrepareRequest(t, prepare.Request{
		PrepareKind:   "lb",
		ImageID:       "postgres:17",
		LiquibaseArgs: []string{"--changelog-file", "config/liquibase/master.xml", "update"},
		PlanOnly:      true,
	})
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-lb",
		Status:      prepare.StatusSucceeded,
		PrepareKind: "lb",
		ImageID:     "postgres:17",
		PlanOnly:    true,
		RequestJSON: strPtr(lbReqJSON),
		CreatedAt:   "2026-03-10T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob lb: %v", err)
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-lb", []queue.TaskRecord{
		{
			JobID:           "job-lb",
			TaskID:          "execute-0",
			Position:        0,
			Type:            "state_execute",
			Status:          prepare.StatusSucceeded,
			InputKind:       strPtr("image"),
			InputID:         strPtr("postgres@sha256:resolved"),
			OutputStateID:   strPtr("state-lb-1"),
			ChangesetID:     strPtr("1"),
			ChangesetAuthor: strPtr("dev"),
			ChangesetPath:   strPtr("config/liquibase/master.xml"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks lb: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/tasks", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	var entries []prepare.TaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected three task entries, got %+v", entries)
	}

	byTaskID := make(map[string]prepare.TaskEntry, len(entries))
	for _, entry := range entries {
		byTaskID[entry.TaskID] = entry
	}
	if byTaskID["execute-1"].ArgsSummary != "-c select 1" {
		t.Fatalf("expected psql args summary, got %+v", byTaskID["execute-1"])
	}
	if byTaskID["prepare-instance"].ArgsSummary != "" {
		t.Fatalf("expected empty args summary for prepare-instance, got %+v", byTaskID["prepare-instance"])
	}
	lbTask := byTaskID["execute-0"]
	if lbTask.JobID != "job-lb" {
		t.Fatalf("expected liquibase task, got %+v", lbTask)
	}
	if lbTask.ArgsSummary != "1::dev::config/liquibase/master.xml" {
		t.Fatalf("expected liquibase args summary, got %+v", lbTask)
	}
	if lbTask.ChangesetID != "1" || lbTask.ChangesetAuthor != "dev" || lbTask.ChangesetPath != "config/liquibase/master.xml" {
		t.Fatalf("expected liquibase changeset fields, got %+v", lbTask)
	}
}

func newListSurfaceServer(t *testing.T) (*httptest.Server, func(), queue.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	queueStore, err := queue.Open(dbPath)
	if err != nil {
		st.Close()
		t.Fatalf("open queue: %v", err)
	}
	reg := registry.New(st)
	prep := newPrepareManager(t, st, queueStore)
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   reg,
		Prepare:    prep,
	})
	server := httptest.NewServer(handler)
	cleanup := func() {
		server.Close()
		_ = queueStore.Close()
		_ = st.Close()
	}
	return server, cleanup, queueStore
}

func mustMarshalPrepareRequest(t *testing.T, req prepare.Request) string {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(data)
}
