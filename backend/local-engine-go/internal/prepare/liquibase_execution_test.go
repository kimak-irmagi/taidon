package prepare

import (
	"context"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestExecuteLiquibaseStepSuccess(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "ok"}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})

	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", WorkDir: "/mnt/c/work"},
		normalizedArgs: []string{"update", "--changelog-file", "/sqlrs/mnt/path1"},
		liquibaseMounts: []engineRuntime.Mount{
			{HostPath: "/host/changelog.xml", ContainerPath: "/sqlrs/mnt/path1", ReadOnly: true},
		},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}

	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{}); errResp != nil {
		t.Fatalf("executeLiquibaseStep: %+v", errResp)
	}
	if len(liquibase.runs) != 1 {
		t.Fatalf("expected liquibase run call, got %+v", liquibase.runs)
	}
	req := liquibase.runs[0]
	if req.ExecPath != "" && req.ExecPath != "liquibase" {
		t.Fatalf("expected default exec path, got %q", req.ExecPath)
	}
	if req.Env != nil {
		if value, ok := req.Env["JAVA_HOME"]; ok && value == "" {
			t.Fatalf("expected JAVA_HOME to be omitted when empty")
		}
	}
	if len(req.Args) < 3 || !strings.HasPrefix(req.Args[0], "--url=") || req.Args[1] != "--username=sqlrs" {
		t.Fatalf("expected connection args prefix, got %+v", req.Args)
	}
}

func TestExecuteLiquibaseStepUsesUpdateCountForChangeset(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "ok"}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})

	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb"},
		normalizedArgs: []string{"update", "--changelog-file", "/sqlrs/mnt/path1"},
	}
	task := taskState{PlanTask: PlanTask{
		ChangesetID:     "1",
		ChangesetAuthor: "dev",
		ChangesetPath:   "changelog.xml",
	}}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}

	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, task); errResp != nil {
		t.Fatalf("executeLiquibaseStep: %+v", errResp)
	}
	if len(liquibase.runs) != 1 {
		t.Fatalf("expected liquibase run call, got %+v", liquibase.runs)
	}
	args := liquibase.runs[0].Args
	found := false
	for _, arg := range args {
		if arg == "update-count" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected update-count in args, got %+v", args)
	}
	if !containsArg(args, "--count") || !containsArg(args, "1") {
		t.Fatalf("expected count args, got %+v", args)
	}
}

func TestExecuteLiquibaseStepWindowsModeSkipsWorkDir(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "ok"}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})

	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", WorkDir: "C:\\work", LiquibaseExec: "C:\\Tools\\liquibase.bat", LiquibaseExecMode: "windows-bat"},
		normalizedArgs: []string{"update"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}

	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{}); errResp != nil {
		t.Fatalf("executeLiquibaseStep: %+v", errResp)
	}
	if len(liquibase.runs) != 1 {
		t.Fatalf("expected liquibase run call, got %+v", liquibase.runs)
	}
	if liquibase.runs[0].WorkDir != "C:\\work" {
		t.Fatalf("expected workdir to be preserved, got %q", liquibase.runs[0].WorkDir)
	}
}

func TestExecuteLiquibaseStepDerivesWorkDirFromChangelog(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "ok"}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})

	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", LiquibaseExec: "C:\\Tools\\liquibase.bat", LiquibaseExecMode: "windows-bat"},
		normalizedArgs: []string{"update", "--changelog-file", "C:\\work\\changelog.xml"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}

	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{}); errResp != nil {
		t.Fatalf("executeLiquibaseStep: %+v", errResp)
	}
	if len(liquibase.runs) != 1 {
		t.Fatalf("expected liquibase run call, got %+v", liquibase.runs)
	}
	if liquibase.runs[0].WorkDir != "C:\\work" {
		t.Fatalf("expected derived workdir, got %q", liquibase.runs[0].WorkDir)
	}
}

func TestExecuteLiquibaseStepMissingRunner(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: nil})
	mgr.liquibase = nil
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1"}}

	errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{})
	if errResp == nil || !strings.Contains(errResp.Message, "liquibase runner is required") {
		t.Fatalf("expected liquibase runner error, got %+v", errResp)
	}
}

func TestExecuteLiquibaseStepMissingContainerID(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{}}

	errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{})
	if errResp == nil || !strings.Contains(errResp.Message, "runtime instance is missing connection info") {
		t.Fatalf("expected container id error, got %+v", errResp)
	}
}

func TestExecuteLiquibaseStepRunError(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "boom", err: context.Canceled}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})
	prepared := preparedRequest{request: Request{PrepareKind: "lb", LiquibaseEnv: map[string]string{"JAVA_HOME": "C:\\Java"}}}
	rt := &jobRuntime{instance: engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}}

	errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", prepared, rt, taskState{})
	if errResp == nil || !strings.Contains(errResp.Message, "liquibase execution failed") {
		t.Fatalf("expected execution error, got %+v", errResp)
	}
	if len(liquibase.runs) != 1 || liquibase.runs[0].Env["JAVA_HOME"] != "C:\\Java" {
		t.Fatalf("expected JAVA_HOME to be forwarded, got %+v", liquibase.runs)
	}
}

func TestPrependLiquibaseConnectionArgs(t *testing.T) {
	out := prependLiquibaseConnectionArgs([]string{"update"}, engineRuntime.Instance{Host: "host", Port: 5432})
	if len(out) < 3 {
		t.Fatalf("expected args, got %+v", out)
	}
	if !strings.HasPrefix(out[0], "--url=") || out[1] != "--username=sqlrs" {
		t.Fatalf("unexpected connection args: %+v", out[:2])
	}
	if out[2] != "update" {
		t.Fatalf("expected update arg, got %+v", out)
	}
	empty := prependLiquibaseConnectionArgs(nil, engineRuntime.Instance{})
	if len(empty) != 2 || !strings.HasPrefix(empty[0], "--url=") {
		t.Fatalf("expected connection args only, got %+v", empty)
	}
}
