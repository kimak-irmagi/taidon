package prepare

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare/queue"
)

func TestTaskArgsSummaryBranches(t *testing.T) {
	t.Run("non state_execute returns empty", func(t *testing.T) {
		got := taskArgsSummary(queue.TaskRecord{Type: "plan"}, &Request{PrepareKind: "psql"})
		if got != "" {
			t.Fatalf("expected empty summary, got %q", got)
		}
	})

	t.Run("liquibase summary has priority", func(t *testing.T) {
		got := taskArgsSummary(queue.TaskRecord{
			Type:            "state_execute",
			ChangesetID:     strPtr("001"),
			ChangesetAuthor: strPtr("alice"),
			ChangesetPath:   strPtr("changelog.xml"),
		}, &Request{PrepareKind: "psql"})
		if got != "001::alice::changelog.xml" {
			t.Fatalf("unexpected liquibase summary: %q", got)
		}
	})

	t.Run("missing liquibase metadata falls back to psql summary", func(t *testing.T) {
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-c", "select 1"}}
		got := taskArgsSummary(queue.TaskRecord{
			TaskID:      "execute-0",
			Type:        "state_execute",
			ChangesetID: strPtr("001"),
		}, req)
		if got != "-c select 1" {
			t.Fatalf("expected psql fallback summary, got %q", got)
		}
	})
}

func TestPsqlTaskArgsSummaryBranches(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		if got := psqlTaskArgsSummary("execute-0", nil); got != "" {
			t.Fatalf("expected empty summary, got %q", got)
		}
	})

	t.Run("non psql request", func(t *testing.T) {
		req := &Request{PrepareKind: "lb", PsqlArgs: []string{"-c", "select 1"}}
		if got := psqlTaskArgsSummary("execute-0", req); got != "" {
			t.Fatalf("expected empty summary, got %q", got)
		}
	})

	t.Run("file input", func(t *testing.T) {
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-f", "schema.sql"}}
		if got := psqlTaskArgsSummary("execute-0", req); got != "-f schema.sql" {
			t.Fatalf("unexpected summary: %q", got)
		}
	})

	t.Run("stdin input", func(t *testing.T) {
		stdin := "select 1"
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-f", "-"}, Stdin: &stdin}
		if got := psqlTaskArgsSummary("execute-0", req); got != "-f -" {
			t.Fatalf("unexpected summary: %q", got)
		}
	})

	t.Run("command input", func(t *testing.T) {
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-c", "select 1"}}
		if got := psqlTaskArgsSummary("execute-0", req); got != "-c select 1" {
			t.Fatalf("unexpected summary: %q", got)
		}
	})

	t.Run("invalid args", func(t *testing.T) {
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-f"}}
		if got := psqlTaskArgsSummary("execute-0", req); got != "" {
			t.Fatalf("expected empty summary for invalid args, got %q", got)
		}
	})

	t.Run("task id out of range", func(t *testing.T) {
		req := &Request{PrepareKind: "psql", PsqlArgs: []string{"-c", "select 1"}}
		if got := psqlTaskArgsSummary("execute-1", req); got != "" {
			t.Fatalf("expected empty summary for out-of-range task id, got %q", got)
		}
	})
}

func TestReplanTasksOnDriftGetJobFailures(t *testing.T) {
	t.Run("get job returns error", func(t *testing.T) {
		queueStore := newQueueStore(t)
		faulty := &faultQueueStore{
			Store: queueStore,
			getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
				return queue.JobRecord{}, false, errors.New("get boom")
			},
		}
		mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
		coordinator := jobCoordinator{m: mgr}

		replanned, tasks, stateID, errResp := coordinator.replanTasksOnDrift(
			context.Background(),
			"job-1",
			preparedRequest{},
			[]queue.TaskRecord{{TaskID: "plan", Type: "plan"}},
		)
		if replanned || tasks != nil || stateID != "" {
			t.Fatalf("unexpected return values: replanned=%v tasks=%v stateID=%q", replanned, tasks, stateID)
		}
		if errResp == nil || !strings.Contains(errResp.Message, "cannot load job") || !strings.Contains(errResp.Details, "get boom") {
			t.Fatalf("expected get-job error response, got %+v", errResp)
		}
	})

	t.Run("get job not found", func(t *testing.T) {
		queueStore := newQueueStore(t)
		faulty := &faultQueueStore{
			Store: queueStore,
			getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
				return queue.JobRecord{}, false, nil
			},
		}
		mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
		coordinator := jobCoordinator{m: mgr}

		replanned, tasks, stateID, errResp := coordinator.replanTasksOnDrift(
			context.Background(),
			"job-404",
			preparedRequest{},
			[]queue.TaskRecord{{TaskID: "plan", Type: "plan"}},
		)
		if replanned || tasks != nil || stateID != "" {
			t.Fatalf("unexpected return values: replanned=%v tasks=%v stateID=%q", replanned, tasks, stateID)
		}
		if errResp == nil || !strings.Contains(errResp.Message, "cannot load job") || !strings.Contains(errResp.Details, "job-404") {
			t.Fatalf("expected get-job not-found response, got %+v", errResp)
		}
	})
}
