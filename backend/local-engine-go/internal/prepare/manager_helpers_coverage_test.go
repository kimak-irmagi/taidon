package prepare

import (
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare/queue"
)

func TestFindOutputStateIDSelectsLastExecuteOutput(t *testing.T) {
	tasks := []taskState{
		{PlanTask: PlanTask{Type: "plan", OutputStateID: "ignored"}},
		{PlanTask: PlanTask{Type: "state_execute", OutputStateID: ""}},
		{PlanTask: PlanTask{Type: "state_execute", OutputStateID: "state-1"}},
		{PlanTask: PlanTask{Type: "state_execute", OutputStateID: "state-2"}},
	}

	if got := findOutputStateID(tasks); got != "state-2" {
		t.Fatalf("expected last execute output state id, got %q", got)
	}
	if got := findOutputStateID(nil); got != "" {
		t.Fatalf("expected empty output state id for empty tasks, got %q", got)
	}
}

func TestTaskInputAndValueHelpers(t *testing.T) {
	if got := taskInputKind(nil); got != "" {
		t.Fatalf("expected empty kind for nil input, got %q", got)
	}
	if got := taskInputID(nil); got != "" {
		t.Fatalf("expected empty id for nil input, got %q", got)
	}
	if got := taskInputKind(&TaskInput{Kind: "image", ID: "img-1"}); got != "image" {
		t.Fatalf("expected kind=image, got %q", got)
	}
	if got := taskInputID(&TaskInput{Kind: "image", ID: "img-1"}); got != "img-1" {
		t.Fatalf("expected id=img-1, got %q", got)
	}

	if got := valueOrEmpty(nil); got != "" {
		t.Fatalf("expected empty string for nil value, got %q", got)
	}
	nonEmpty := "value"
	if got := valueOrEmpty(&nonEmpty); got != "value" {
		t.Fatalf("expected value from pointer, got %q", got)
	}

	if got := nullableString("   "); got != nil {
		t.Fatalf("expected nil for blank value, got %q", *got)
	}
	ptr := nullableString(" value ")
	if ptr == nil || *ptr != " value " {
		t.Fatalf("expected non-empty pointer to original value, got %+v", ptr)
	}
}

func TestHasRunningTasks(t *testing.T) {
	if hasRunningTasks(nil) {
		t.Fatalf("expected false for empty tasks")
	}
	if hasRunningTasks([]queue.TaskRecord{{TaskID: "t1", Status: StatusSucceeded}}) {
		t.Fatalf("expected false for non-running tasks")
	}
	if !hasRunningTasks([]queue.TaskRecord{
		{TaskID: "t1", Status: StatusSucceeded},
		{TaskID: "t2", Status: StatusRunning},
	}) {
		t.Fatalf("expected true when at least one task is running")
	}
}

func TestNeedsImageResolve(t *testing.T) {
	if !needsImageResolve("postgres:17") {
		t.Fatalf("expected tag-only image to require resolve")
	}
	if needsImageResolve("postgres@sha256:abc") {
		t.Fatalf("expected digest image to skip resolve")
	}
	if !needsImageResolve("   ") {
		t.Fatalf("expected blank image id to require resolve")
	}
}
