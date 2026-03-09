package prepare

import (
	"strings"
	"testing"
)

func TestBuildPsqlStepsSplitsCommandsAndFiles(t *testing.T) {
	stdin := "select 6;"
	steps, err := buildPsqlSteps([]string{
		"-v", "ON_ERROR_STOP=1",
		"-c", "select 1",
		"--command=select 2",
		"-cselect 3",
		"-f", "schema.sql",
		"--file=seed.sql",
		"-f-",
	}, &stdin)
	if err != nil {
		t.Fatalf("buildPsqlSteps: %v", err)
	}
	if len(steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(steps))
	}
	for i, step := range steps {
		if len(step.args) < 2 || step.args[0] != "-v" || step.args[1] != "ON_ERROR_STOP=1" {
			t.Fatalf("step %d missing shared args: %+v", i, step.args)
		}
	}
	if steps[0].inputs[0] != (psqlInput{kind: "command", value: "select 1"}) {
		t.Fatalf("unexpected command step: %+v", steps[0])
	}
	if steps[1].inputs[0] != (psqlInput{kind: "command", value: "select 2"}) {
		t.Fatalf("unexpected command= step: %+v", steps[1])
	}
	if steps[2].inputs[0] != (psqlInput{kind: "command", value: "select 3"}) {
		t.Fatalf("unexpected compact command step: %+v", steps[2])
	}
	if steps[3].inputs[0] != (psqlInput{kind: "file", value: "schema.sql"}) {
		t.Fatalf("unexpected file step: %+v", steps[3])
	}
	if steps[4].inputs[0] != (psqlInput{kind: "file", value: "seed.sql"}) {
		t.Fatalf("unexpected file= step: %+v", steps[4])
	}
	if steps[5].inputs[0] != (psqlInput{kind: "stdin", value: stdin}) || steps[5].stdin == nil || *steps[5].stdin != stdin {
		t.Fatalf("unexpected stdin step: %+v", steps[5])
	}
}

func TestBuildPsqlStepsReturnsSharedArgsWhenNoSteps(t *testing.T) {
	steps, err := buildPsqlSteps([]string{"-v", "ON_ERROR_STOP=1", "postgres://db"}, nil)
	if err != nil {
		t.Fatalf("buildPsqlSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one default step, got %d", len(steps))
	}
	if strings.Join(steps[0].args, " ") != "-v ON_ERROR_STOP=1 postgres://db" {
		t.Fatalf("unexpected args: %+v", steps[0].args)
	}
	if len(steps[0].inputs) != 0 || steps[0].stdin != nil {
		t.Fatalf("expected no step inputs, got %+v", steps[0])
	}
}

func TestBuildPsqlStepsAndFileStepErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing short command", args: []string{"-c"}, want: "missing value for command flag"},
		{name: "missing long command", args: []string{"--command"}, want: "missing value for command flag"},
		{name: "missing short file", args: []string{"-f"}, want: "missing value for file flag"},
		{name: "missing long file equals", args: []string{"--file="}, want: "missing value for file flag"},
		{name: "blank file path", args: []string{"-f", "   "}, want: "file path is empty"},
		{name: "stdin required", args: []string{"-f-"}, want: "stdin is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildPsqlSteps(tc.args, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPsqlStepSelectionAndTaskIndex(t *testing.T) {
	steps := []psqlStep{{args: []string{"-c", "select 1"}}, {args: []string{"-c", "select 2"}}}

	step, err := psqlStepForTask(steps, "")
	if err != nil {
		t.Fatalf("psqlStepForTask blank id: %v", err)
	}
	if step.args[1] != "select 1" {
		t.Fatalf("expected first step, got %+v", step)
	}

	step, err = psqlStepForTask(steps, "execute-1")
	if err != nil {
		t.Fatalf("psqlStepForTask indexed: %v", err)
	}
	if step.args[1] != "select 2" {
		t.Fatalf("expected second step, got %+v", step)
	}

	if _, err := psqlStepForTask(nil, ""); err == nil || !strings.Contains(err.Error(), "psql steps are required") {
		t.Fatalf("expected empty steps error, got %v", err)
	}
	if _, err := psqlStepForTask(steps, "execute-3"); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected range error, got %v", err)
	}
	for _, taskID := range []string{"task-1", "execute-", "execute--1", "execute-nope"} {
		if _, err := executeTaskIndex(taskID); err == nil || !strings.Contains(err.Error(), "invalid execute task id") {
			t.Fatalf("expected invalid task id error for %q, got %v", taskID, err)
		}
	}
}

func TestPsqlStepForPreparedTaskUsesPreparedFallbackAndDelegation(t *testing.T) {
	stdin := "select stdin;"
	prepared := preparedRequest{
		request:        Request{Stdin: &stdin},
		normalizedArgs: []string{"-v", "ON_ERROR_STOP=1"},
		psqlInputs:     []psqlInput{{kind: "command", value: "select 1"}},
	}

	step, err := psqlStepForPreparedTask(prepared, "")
	if err != nil {
		t.Fatalf("psqlStepForPreparedTask fallback: %v", err)
	}
	if strings.Join(step.args, " ") != "-v ON_ERROR_STOP=1" {
		t.Fatalf("unexpected fallback args: %+v", step.args)
	}
	if len(step.inputs) != 1 || step.inputs[0] != (psqlInput{kind: "command", value: "select 1"}) || step.stdin != &stdin {
		t.Fatalf("unexpected fallback step: %+v", step)
	}

	prepared.psqlSteps = []psqlStep{{args: []string{"-c", "select 1"}}, {args: []string{"-c", "select 2"}}}
	step, err = psqlStepForPreparedTask(prepared, "execute-1")
	if err != nil {
		t.Fatalf("psqlStepForPreparedTask delegated: %v", err)
	}
	if step.args[1] != "select 2" {
		t.Fatalf("unexpected delegated step: %+v", step)
	}
}
