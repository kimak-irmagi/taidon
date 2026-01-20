package queue

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSQLiteStoreJobTaskEventRoundTrip(t *testing.T) {
	store := newQueueStore(t)

	job := JobRecord{
		JobID:       "job-1",
		Status:      "queued",
		PrepareKind: "psql",
		ImageID:     "image-1",
		PlanOnly:    true,
		CreatedAt:   "2026-01-19T00:00:00Z",
	}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	args := "args"
	if err := store.UpdateJob(context.Background(), "job-1", JobUpdate{
		Status:                stringPtr("running"),
		PrepareArgsNormalized: &args,
		StartedAt:             stringPtr("2026-01-19T00:01:00Z"),
	}); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	record, ok, err := store.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if !ok || record.Status != "running" || record.PrepareArgsNormalized == nil {
		t.Fatalf("unexpected job record: %+v", record)
	}

	tasks := []TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "plan",
			Position: 0,
			Type:     "plan",
			Status:   "queued",
		},
		{
			JobID:    "job-1",
			TaskID:   "execute-0",
			Position: 1,
			Type:     "state_execute",
			Status:   "queued",
			Cached:   boolPtrFromValue(true),
		},
	}
	if err := store.ReplaceTasks(context.Background(), "job-1", tasks); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	if err := store.UpdateTask(context.Background(), "job-1", "execute-0", TaskUpdate{
		Status:    stringPtr("running"),
		StartedAt: stringPtr("2026-01-19T00:02:00Z"),
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	taskRows, err := store.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(taskRows) != 2 || taskRows[1].Status != "running" {
		t.Fatalf("unexpected tasks: %+v", taskRows)
	}

	if _, err := store.AppendEvent(context.Background(), EventRecord{
		JobID:  "job-1",
		Type:   "status",
		Ts:     "2026-01-19T00:02:30Z",
		Status: stringPtr("running"),
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, err := store.ListEventsSince(context.Background(), "job-1", 0)
	if err != nil {
		t.Fatalf("ListEventsSince: %v", err)
	}
	if len(events) != 1 || events[0].Status == nil {
		t.Fatalf("unexpected events: %+v", events)
	}

	count, err := store.CountEvents(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("CountEvents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 event, got %d", count)
	}
}

func TestSQLiteStoreListJobsByStatus(t *testing.T) {
	store := newQueueStore(t)

	for _, jobID := range []string{"job-1", "job-2"} {
		if err := store.CreateJob(context.Background(), JobRecord{
			JobID:       jobID,
			Status:      "queued",
			PrepareKind: "psql",
			ImageID:     "image-1",
			CreatedAt:   "2026-01-19T00:00:00Z",
		}); err != nil {
			t.Fatalf("CreateJob %s: %v", jobID, err)
		}
	}
	if err := store.UpdateJob(context.Background(), "job-2", JobUpdate{
		Status: stringPtr("running"),
	}); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	jobs, err := store.ListJobsByStatus(context.Background(), []string{"running"})
	if err != nil {
		t.Fatalf("ListJobsByStatus: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "job-2" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}

func TestSQLiteStoreDeleteJobCascades(t *testing.T) {
	store := newQueueStore(t)

	if err := store.CreateJob(context.Background(), JobRecord{
		JobID:       "job-1",
		Status:      "queued",
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   "2026-01-19T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := store.ReplaceTasks(context.Background(), "job-1", []TaskRecord{
		{JobID: "job-1", TaskID: "plan", Position: 0, Type: "plan", Status: "queued"},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	if _, err := store.AppendEvent(context.Background(), EventRecord{
		JobID: "job-1",
		Type:  "status",
		Ts:    "2026-01-19T00:01:00Z",
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	if err := store.DeleteJob(context.Background(), "job-1"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	tasks, err := store.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected no tasks, got %+v", tasks)
	}

	events, err := store.ListEventsSince(context.Background(), "job-1", 0)
	if err != nil {
		t.Fatalf("ListEventsSince: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %+v", events)
	}
}

func TestSQLiteStoreClosedDatabase(t *testing.T) {
	store := newQueueStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := store.ListJobs(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenRequiresPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestNewRequiresDB(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatalf("expected error for nil db")
	}
}

func TestUpdateJobRequiresID(t *testing.T) {
	store := newQueueStore(t)
	if err := store.UpdateJob(context.Background(), "", JobUpdate{Status: stringPtr("running")}); err == nil {
		t.Fatalf("expected error for empty job id")
	}
}

func TestUpdateJobNoFields(t *testing.T) {
	store := newQueueStore(t)
	if err := store.UpdateJob(context.Background(), "job-1", JobUpdate{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestListJobsByStatusEmpty(t *testing.T) {
	store := newQueueStore(t)
	jobs, err := store.ListJobsByStatus(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListJobsByStatus: %v", err)
	}
	if jobs != nil {
		t.Fatalf("expected nil jobs, got %+v", jobs)
	}
}

func TestUpdateTaskRequiresIDs(t *testing.T) {
	store := newQueueStore(t)
	if err := store.UpdateTask(context.Background(), "", "", TaskUpdate{Status: stringPtr("running")}); err == nil {
		t.Fatalf("expected error for empty ids")
	}
}

func TestUpdateTaskNoFields(t *testing.T) {
	store := newQueueStore(t)
	if err := store.UpdateTask(context.Background(), "job-1", "task-1", TaskUpdate{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestJobSnapshotModeDefault(t *testing.T) {
	if mode := jobSnapshotMode(""); mode != "always" {
		t.Fatalf("unexpected snapshot mode: %s", mode)
	}
}

func TestNullBoolAndBoolPtr(t *testing.T) {
	if out := nullBool(nil); out.Valid {
		t.Fatalf("expected null for nil bool")
	}
	if out := nullBool(boolPtrFromValue(true)); !out.Valid || out.Int64 != 1 {
		t.Fatalf("unexpected nullBool result: %+v", out)
	}
	if out := boolPtr(sql.NullInt64{}); out != nil {
		t.Fatalf("expected nil bool")
	}
}

type errorScanner struct{}

func (errorScanner) Scan(dest ...any) error {
	return context.Canceled
}

func TestScanHelpersReturnErrors(t *testing.T) {
	if _, err := scanJob(errorScanner{}); err == nil {
		t.Fatalf("expected scanJob error")
	}
	if _, err := scanTask(errorScanner{}); err == nil {
		t.Fatalf("expected scanTask error")
	}
	if _, err := scanEvent(errorScanner{}); err == nil {
		t.Fatalf("expected scanEvent error")
	}
}

func newQueueStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func stringPtr(value string) *string {
	return &value
}

func boolPtrFromValue(value bool) *bool {
	return &value
}
