package prepare

import (
	"context"
	"reflect"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestLiquibasePlanningAndExecutionRequestConsistency(t *testing.T) {
	output := "-- Changeset db/changelog.xml::1::dev\nSELECT 1;"
	liquibase := &fakeLiquibaseRunner{output: output}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})
	prepared := preparedRequest{
		request: Request{
			PrepareKind:   "lb",
			WorkDir:       "/work",
			LiquibaseEnv:  map[string]string{"LIQUIBASE_HOME": "/opt/liquibase"},
			LiquibaseExec: "liquibase",
		},
		normalizedArgs: []string{"update", "--changelog-file", "db/changelog.xml"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	changesets, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp != nil {
		t.Fatalf("runLiquibaseUpdateSQL: %+v", errResp)
	}
	if len(changesets) != 1 {
		t.Fatalf("expected parsed changeset, got %+v", changesets)
	}
	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{}); errResp != nil {
		t.Fatalf("executeLiquibaseStep: %+v", errResp)
	}
	if len(liquibase.runs) != 2 {
		t.Fatalf("expected two liquibase runs, got %d", len(liquibase.runs))
	}

	planReq := liquibase.runs[0]
	execReq := liquibase.runs[1]
	if planReq.ExecPath != execReq.ExecPath {
		t.Fatalf("expected same exec path, got plan=%q exec=%q", planReq.ExecPath, execReq.ExecPath)
	}
	if planReq.ExecMode != execReq.ExecMode {
		t.Fatalf("expected same exec mode, got plan=%q exec=%q", planReq.ExecMode, execReq.ExecMode)
	}
	if planReq.WorkDir != execReq.WorkDir {
		t.Fatalf("expected same workdir, got plan=%q exec=%q", planReq.WorkDir, execReq.WorkDir)
	}
	if !reflect.DeepEqual(planReq.Env, execReq.Env) {
		t.Fatalf("expected same env map, plan=%+v exec=%+v", planReq.Env, execReq.Env)
	}
	if !containsArg(planReq.Args, "--changelog-file") || !containsArg(execReq.Args, "--changelog-file") {
		t.Fatalf("expected changelog arg in both requests: plan=%+v exec=%+v", planReq.Args, execReq.Args)
	}
	if !containsArg(planReq.Args, "updateSQL") {
		t.Fatalf("expected updateSQL command in planning request: %+v", planReq.Args)
	}
	if !containsArg(execReq.Args, "update") && !containsArg(execReq.Args, "update-count") {
		t.Fatalf("expected execution command in request: %+v", execReq.Args)
	}
	if len(planReq.Args) < 2 || len(execReq.Args) < 2 {
		t.Fatalf("expected connection prefix args, plan=%+v exec=%+v", planReq.Args, execReq.Args)
	}
	if planReq.Args[1] != "--username=sqlrs" || execReq.Args[1] != "--username=sqlrs" {
		t.Fatalf("expected username arg in both requests, plan=%+v exec=%+v", planReq.Args, execReq.Args)
	}
}
