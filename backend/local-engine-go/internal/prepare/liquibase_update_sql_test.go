package prepare

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestRunLiquibaseUpdateSQLMissingRunner(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	mgr.liquibase = nil
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}, normalizedArgs: []string{"update"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "liquibase runner is required") {
		t.Fatalf("expected runner error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLMissingRuntime(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}, normalizedArgs: []string{"update"}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, nil)
	if errResp == nil || !strings.Contains(errResp.Message, "runtime instance is required") {
		t.Fatalf("expected runtime error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLMissingConnectionInfo(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}, normalizedArgs: []string{"update"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "missing connection info") {
		t.Fatalf("expected connection info error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLExecPathError(t *testing.T) {
	setWSLForTest(t, true)
	withExecCommandStub(t, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 1)
	})
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", LiquibaseExec: "C:\\Tools\\liquibase.exe"},
		normalizedArgs: []string{"update"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot resolve liquibase executable") {
		t.Fatalf("expected exec path error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLMapArgsError(t *testing.T) {
	setWSLForTest(t, true)
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", LiquibaseExec: "C:\\Tools\\liquibase.bat", LiquibaseExecMode: "windows-bat"},
		normalizedArgs: []string{"update", "--changelog-file"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot map liquibase arguments") {
		t.Fatalf("expected map args error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLWorkDirMapError(t *testing.T) {
	setWSLForTest(t, true)
	withExecCommandStub(t, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 1)
	})
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", LiquibaseExec: "C:\\Tools\\liquibase.bat", LiquibaseExecMode: "windows-bat", WorkDir: "/mnt/c/work"},
		normalizedArgs: []string{"update"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot map liquibase workdir") {
		t.Fatalf("expected workdir map error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLEnvMapError(t *testing.T) {
	setWSLForTest(t, true)
	withExecCommandStub(t, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 1)
	})
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb", LiquibaseEnv: map[string]string{"JAVA_HOME": "C:\\Java"}},
		normalizedArgs: []string{"update"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot map liquibase env") {
		t.Fatalf("expected env map error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLParseError(t *testing.T) {
	liquibase := &fakeLiquibaseRunner{output: "not a changeset"}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}, normalizedArgs: []string{"update"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot parse liquibase changesets") {
		t.Fatalf("expected parse error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLSuccess(t *testing.T) {
	output := strings.Join([]string{
		"-- Changeset db/changelog.xml::1::dev",
		"INSERT INTO databasechangelog (MD5SUM) VALUES ('abc');",
	}, "\n")
	liquibase := &fakeLiquibaseRunner{output: output}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: liquibase})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}, normalizedArgs: []string{"update"}}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	changesets, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp != nil {
		t.Fatalf("expected success, got %+v", errResp)
	}
	if len(changesets) != 1 || changesets[0].ID != "1" || changesets[0].Author != "dev" {
		t.Fatalf("unexpected changesets: %+v", changesets)
	}
	if len(liquibase.runs) != 1 || !containsArg(liquibase.runs[0].Args, "updateSQL") {
		t.Fatalf("expected updateSQL in args, got %+v", liquibase.runs)
	}
}

func withExecCommandStub(t *testing.T, fn func(context.Context, string, ...string) *exec.Cmd) {
	t.Helper()
	prev := execCommand
	execCommand = fn
	t.Cleanup(func() { execCommand = prev })
}

func commandExit(ctx context.Context, code int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", "exit "+strconv.Itoa(code))
	}
	return exec.CommandContext(ctx, "sh", "-c", "exit "+strconv.Itoa(code))
}
