package cli

import (
	"bytes"
	"strings"
	"testing"

	"sqlrs/cli/internal/client"
)

func TestPrintPlan(t *testing.T) {
	result := PlanResult{
		Tasks: []client.PlanTask{
			{Type: "plan", PlannerKind: "psql"},
			{
				Type:          "state_execute",
				Input:         &client.TaskInput{Kind: "image", ID: "img"},
				TaskHash:      "hash-1",
				OutputStateID: "state-1",
				Cached:        boolPtr(true),
			},
			{
				Type:         "prepare_instance",
				Input:        &client.TaskInput{Kind: "state", ID: "state-2"},
				InstanceMode: "ephemeral",
			},
		},
	}

	var out bytes.Buffer
	if err := PrintPlan(&out, result); err != nil {
		t.Fatalf("PrintPlan: %v", err)
	}

	expected := strings.Join([]string{
		"Final state: state-2",
		"Tasks:",
		"  1. plan (planner: psql)",
		"  2. state_execute input=image:img hash=hash-1 output=state-1 cached=yes",
		"  3. prepare_instance input=state:state-2 mode=ephemeral",
		"",
	}, "\n")
	if out.String() != expected {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestPrintPlanMissingFinalState(t *testing.T) {
	var out bytes.Buffer
	err := PrintPlan(&out, PlanResult{Tasks: []client.PlanTask{{Type: "plan"}}})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFinalStateIDPrefersPrepareInstance(t *testing.T) {
	state, err := finalStateID([]client.PlanTask{
		{Type: "plan"},
		{Type: "state_execute", OutputStateID: "state-1"},
		{Type: "prepare_instance", Input: &client.TaskInput{Kind: "state", ID: "state-2"}},
	})
	if err != nil {
		t.Fatalf("finalStateID: %v", err)
	}
	if state != "state-2" {
		t.Fatalf("expected state-2, got %q", state)
	}
}

func TestFinalStateIDFallsBackToExecute(t *testing.T) {
	state, err := finalStateID([]client.PlanTask{
		{Type: "plan"},
		{Type: "state_execute", OutputStateID: "state-3"},
		{Type: "prepare_instance", Input: &client.TaskInput{Kind: "image", ID: "img"}},
	})
	if err != nil {
		t.Fatalf("finalStateID: %v", err)
	}
	if state != "state-3" {
		t.Fatalf("expected state-3, got %q", state)
	}
}

func TestFinalStateIDMissing(t *testing.T) {
	if _, err := finalStateID([]client.PlanTask{{Type: "plan"}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFormatPlanTask(t *testing.T) {
	cached := true
	cases := []struct {
		task     client.PlanTask
		expected string
	}{
		{task: client.PlanTask{Type: "plan", PlannerKind: "psql"}, expected: "plan (planner: psql)"},
		{task: client.PlanTask{Type: "plan"}, expected: "plan"},
		{
			task: client.PlanTask{
				Type:          "state_execute",
				Input:         &client.TaskInput{Kind: "image", ID: "img"},
				TaskHash:      "hash",
				OutputStateID: "state-1",
				Cached:        &cached,
			},
			expected: "state_execute input=image:img hash=hash output=state-1 cached=yes",
		},
		{
			task: client.PlanTask{
				Type:         "prepare_instance",
				Input:        &client.TaskInput{Kind: "state", ID: "state-2"},
				InstanceMode: "",
			},
			expected: "prepare_instance input=state:state-2 mode=unknown",
		},
		{task: client.PlanTask{Type: "custom"}, expected: "custom"},
	}

	for _, tc := range cases {
		if got := formatPlanTask(tc.task); got != tc.expected {
			t.Fatalf("unexpected formatPlanTask output: %q (expected %q)", got, tc.expected)
		}
	}
}

func TestFormatTaskInput(t *testing.T) {
	if got := formatTaskInput(nil); got != "unknown" {
		t.Fatalf("expected unknown, got %q", got)
	}
	if got := formatTaskInput(&client.TaskInput{ID: "id"}); got != "id" {
		t.Fatalf("expected id, got %q", got)
	}
	if got := formatTaskInput(&client.TaskInput{Kind: "state", ID: "s1"}); got != "state:s1" {
		t.Fatalf("expected state:s1, got %q", got)
	}
}

func TestFormatCached(t *testing.T) {
	if got := formatCached(nil); got != "n/a" {
		t.Fatalf("expected n/a, got %q", got)
	}
	if got := formatCached(boolPtr(true)); got != "yes" {
		t.Fatalf("expected yes, got %q", got)
	}
	if got := formatCached(boolPtr(false)); got != "no" {
		t.Fatalf("expected no, got %q", got)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
