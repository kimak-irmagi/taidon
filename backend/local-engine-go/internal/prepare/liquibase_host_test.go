package prepare

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestHostLiquibaseRunnerDefaultExec(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var gotName string
	var gotArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo ok")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo ok")
	}
	runner := hostLiquibaseRunner{}
	if _, err := runner.Run(context.Background(), LiquibaseRunRequest{Args: []string{"update"}}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotName != "liquibase" {
		t.Fatalf("expected default exec name, got %q", gotName)
	}
	if len(gotArgs) == 0 {
		t.Fatalf("expected args to be passed")
	}
}

func TestHostLiquibaseRunnerLogSink(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo line1 & echo line2 1>&2")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo line1 && echo line2 1>&2")
	}
	var lines []string
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) {
		lines = append(lines, line)
	})
	runner := hostLiquibaseRunner{}
	if _, err := runner.Run(ctx, LiquibaseRunRequest{ExecPath: "liquibase", Args: []string{"update"}}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	line1 := -1
	line2 := -1
	for i, line := range lines {
		switch line {
		case "line1":
			if line1 == -1 {
				line1 = i
			}
		case "line2":
			if line2 == -1 {
				line2 = i
			}
		}
	}
	if line1 == -1 || line2 == -1 || line1 >= line2 {
		t.Fatalf("expected streamed lines, got %+v", lines)
	}
}

func TestFormatEnv(t *testing.T) {
	out := formatEnv(map[string]string{"FOO": "bar", "": "skip"})
	if len(out) != 1 || !strings.Contains(out[0], "FOO=bar") {
		t.Fatalf("unexpected env output: %+v", out)
	}
}

func TestHostLiquibaseRunnerWindowsBatMode(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var gotName string
	var gotArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo ok")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo ok")
	}
	runner := hostLiquibaseRunner{}
	req := LiquibaseRunRequest{
		ExecPath: "C:\\Program Files\\Liquibase\\liquibase.bat",
		ExecMode: "windows-bat",
		Args:     []string{"update", "--changelog-file", "C:\\Work\\changelog.xml"},
	}
	if _, err := runner.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotName != "cmd.exe" {
		t.Fatalf("expected cmd.exe, got %q", gotName)
	}
	if len(gotArgs) < 4 || gotArgs[0] != "/c" || gotArgs[1] != "call" {
		t.Fatalf("expected /c call args, got %+v", gotArgs)
	}
	if gotArgs[2] != req.ExecPath {
		t.Fatalf("expected exec path %q, got %q", req.ExecPath, gotArgs[2])
	}
}
