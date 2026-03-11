package prepare

import (
	"encoding/json"
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare/queue"
)

func TestDecodeJobRequestBranches(t *testing.T) {
	if got := decodeJobRequest(queue.JobRecord{}); got != nil {
		t.Fatalf("expected nil for missing request json, got %+v", got)
	}

	blank := "   "
	if got := decodeJobRequest(queue.JobRecord{RequestJSON: &blank}); got != nil {
		t.Fatalf("expected nil for blank request json, got %+v", got)
	}

	invalid := "{not-json"
	if got := decodeJobRequest(queue.JobRecord{RequestJSON: &invalid}); got != nil {
		t.Fatalf("expected nil for invalid request json, got %+v", got)
	}

	raw, err := json.Marshal(Request{
		PrepareKind: "psql",
		ImageID:     "postgres:17",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	payload := string(raw)
	got := decodeJobRequest(queue.JobRecord{RequestJSON: &payload})
	if got == nil {
		t.Fatalf("expected decoded request")
	}
	if got.PrepareKind != "psql" || got.ImageID != "postgres:17" || len(got.PsqlArgs) != 2 {
		t.Fatalf("unexpected decoded request: %+v", got)
	}
}

func TestTaskEntryFromRecordBranches(t *testing.T) {
	t.Run("without input pointers", func(t *testing.T) {
		task := queue.TaskRecord{
			TaskID: "plan",
			JobID:  "job-1",
			Type:   "plan",
			Status: StatusSucceeded,
		}
		entry := taskEntryFromRecord(task, nil)
		if entry.Input != nil {
			t.Fatalf("expected nil input, got %+v", entry.Input)
		}
		if entry.ArgsSummary != "" {
			t.Fatalf("expected empty args summary, got %q", entry.ArgsSummary)
		}
	})

	t.Run("liquibase summary", func(t *testing.T) {
		task := queue.TaskRecord{
			TaskID:          "execute-0",
			JobID:           "job-1",
			Type:            "state_execute",
			Status:          StatusSucceeded,
			ChangesetID:     strPtr("001"),
			ChangesetAuthor: strPtr("alice"),
			ChangesetPath:   strPtr("changelog.xml"),
		}
		entry := taskEntryFromRecord(task, nil)
		if entry.ArgsSummary != "001::alice::changelog.xml" {
			t.Fatalf("unexpected liquibase summary: %q", entry.ArgsSummary)
		}
	})

	t.Run("psql summary", func(t *testing.T) {
		task := queue.TaskRecord{
			TaskID:    "execute-0",
			JobID:     "job-1",
			Type:      "state_execute",
			Status:    StatusSucceeded,
			InputKind: strPtr("state"),
			InputID:   strPtr("state-1"),
		}
		req := &Request{
			PrepareKind: "psql",
			PsqlArgs:    []string{"-c", "select 1"},
		}
		entry := taskEntryFromRecord(task, req)
		if entry.Input == nil || entry.Input.Kind != "state" || entry.Input.ID != "state-1" {
			t.Fatalf("unexpected input projection: %+v", entry.Input)
		}
		if entry.ArgsSummary != "-c select 1" {
			t.Fatalf("unexpected psql summary: %q", entry.ArgsSummary)
		}
	})
}

func TestEventFromRecordBranches(t *testing.T) {
	t.Run("invalid json payloads are ignored", func(t *testing.T) {
		bad := "{bad-json"
		event := eventFromRecord(queue.EventRecord{
			Type:       "task",
			Ts:         "2026-03-10T10:00:00Z",
			Status:     strPtr(StatusFailed),
			TaskID:     strPtr("execute-0"),
			Message:    strPtr("failed"),
			ResultJSON: &bad,
			ErrorJSON:  &bad,
		})
		if event.Result != nil || event.Error != nil {
			t.Fatalf("expected invalid payloads to be ignored, got %+v", event)
		}
	})

	t.Run("valid json payloads are decoded", func(t *testing.T) {
		resultRaw, err := json.Marshal(Result{StateID: "state-1", ImageID: "postgres:17"})
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		errorRaw, err := json.Marshal(ErrorResponse{Code: "internal_error", Message: "boom"})
		if err != nil {
			t.Fatalf("marshal error response: %v", err)
		}
		resultStr := string(resultRaw)
		errorStr := string(errorRaw)

		event := eventFromRecord(queue.EventRecord{
			Type:       "task",
			Ts:         "2026-03-10T10:00:00Z",
			Status:     strPtr(StatusFailed),
			TaskID:     strPtr("execute-0"),
			Message:    strPtr("failed"),
			ResultJSON: &resultStr,
			ErrorJSON:  &errorStr,
		})
		if event.Result == nil || event.Result.StateID != "state-1" {
			t.Fatalf("expected decoded result, got %+v", event.Result)
		}
		if event.Error == nil || event.Error.Code != "internal_error" || event.Error.Message != "boom" {
			t.Fatalf("expected decoded error, got %+v", event.Error)
		}
	})
}
