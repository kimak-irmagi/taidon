package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPlanPsqlCreatesStateChainPerInput(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "01-schema.sql")
	secondPath := filepath.Join(dir, "02-data.sql")
	thirdPath := filepath.Join(dir, "03-constraints.sql")
	if err := os.WriteFile(firstPath, []byte("create table test(id int);\n"), 0o600); err != nil {
		t.Fatalf("write first script: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("insert into test values (1);\n"), 0o600); err != nil {
		t.Fatalf("write second script: %v", err)
	}
	if err := os.WriteFile(thirdPath, []byte("alter table test add primary key (id);\n"), 0o600); err != nil {
		t.Fatalf("write third script: %v", err)
	}

	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1@sha256:resolved",
		PsqlArgs:    []string{"-f", firstPath, "-f", secondPath, "-f", thirdPath},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if errResp := mgr.ensureResolvedImageID(context.Background(), "job-1", &prepared, nil); errResp != nil {
		t.Fatalf("ensureResolvedImageID: %+v", errResp)
	}

	tasks, stateID, errResp := mgr.buildPlan(context.Background(), "job-1", prepared)
	if errResp != nil {
		t.Fatalf("buildPlan: %+v", errResp)
	}
	if len(tasks) != 5 {
		t.Fatalf("expected 5 tasks (plan + 3 execute + prepare-instance), got %d", len(tasks))
	}
	firstExec := tasks[1]
	secondExec := tasks[2]
	thirdExec := tasks[3]
	if firstExec.Type != "state_execute" || secondExec.Type != "state_execute" || thirdExec.Type != "state_execute" {
		t.Fatalf("expected 3 state_execute tasks, got %+v", tasks)
	}
	if firstExec.Input == nil || firstExec.Input.Kind != "image" || firstExec.Input.ID != prepared.effectiveImageID() {
		t.Fatalf("unexpected first input: %+v", firstExec.Input)
	}
	if secondExec.Input == nil || secondExec.Input.Kind != "state" || secondExec.Input.ID != firstExec.OutputStateID {
		t.Fatalf("unexpected second input: %+v", secondExec.Input)
	}
	if thirdExec.Input == nil || thirdExec.Input.Kind != "state" || thirdExec.Input.ID != secondExec.OutputStateID {
		t.Fatalf("unexpected third input: %+v", thirdExec.Input)
	}
	if stateID != thirdExec.OutputStateID {
		t.Fatalf("expected final state id %s, got %s", thirdExec.OutputStateID, stateID)
	}
	if tasks[4].Type != "prepare_instance" || tasks[4].Input == nil || tasks[4].Input.ID != thirdExec.OutputStateID {
		t.Fatalf("unexpected prepare-instance task: %+v", tasks[4])
	}
}

func TestSubmitPsqlExecutesEachInputAsSeparateStep(t *testing.T) {
	store := &fakeStore{}
	psql := &fakePsqlRunner{}
	mgr := newManagerWithDeps(t, store, newQueueStore(t), &testDeps{psql: psql})

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1@sha256:resolved",
		PsqlArgs:    []string{"-c", "select 1", "-c", "select 2"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if len(psql.runs) != 2 {
		t.Fatalf("expected 2 psql runs, got %d", len(psql.runs))
	}
	if !containsArg(psql.runs[0].Args, "select 1") || containsArg(psql.runs[0].Args, "select 2") {
		t.Fatalf("expected first run to execute only first command, got %+v", psql.runs[0].Args)
	}
	if !containsArg(psql.runs[1].Args, "select 2") || containsArg(psql.runs[1].Args, "select 1") {
		t.Fatalf("expected second run to execute only second command, got %+v", psql.runs[1].Args)
	}
	if len(store.states) != 2 {
		t.Fatalf("expected 2 persisted states, got %d", len(store.states))
	}
	if store.states[0].ParentStateID != nil {
		t.Fatalf("expected first state to be rooted at image, got parent %+v", store.states[0].ParentStateID)
	}
	if store.states[1].ParentStateID == nil || *store.states[1].ParentStateID != store.states[0].StateID {
		t.Fatalf("expected second state parent to be first state, got %+v", store.states[1].ParentStateID)
	}
}
