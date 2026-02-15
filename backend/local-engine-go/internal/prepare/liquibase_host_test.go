package prepare

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestQuoteCmdArg(t *testing.T) {
	if out := quoteCmdArg(""); out != "\"\"" {
		t.Fatalf("expected empty quoted, got %q", out)
	}
	if out := quoteCmdArg("plain"); out != "plain" {
		t.Fatalf("expected unchanged, got %q", out)
	}
	if out := quoteCmdArg(`has space`); out != `"has space"` {
		t.Fatalf("expected quoted, got %q", out)
	}
	if out := quoteCmdArg(`has"quote`); out != `"has""quote"` {
		t.Fatalf("expected escaped quote, got %q", out)
	}
}

func TestBuildCmdLine(t *testing.T) {
	out := buildCmdLine("liquibase", []string{"--defaults-file", "C:\\path with space\\lb.properties"})
	if !strings.Contains(out, `"C:\path with space\lb.properties"`) {
		t.Fatalf("expected quoted path, got %q", out)
	}
	if !strings.HasPrefix(out, "liquibase") {
		t.Fatalf("expected exec prefix, got %q", out)
	}
}

func TestNormalizeExecMode(t *testing.T) {
	if normalizeExecMode("") != "auto" {
		t.Fatalf("expected auto for empty")
	}
	if normalizeExecMode("Native") != "native" {
		t.Fatalf("expected normalized native")
	}
	if normalizeExecMode("invalid") != "auto" {
		t.Fatalf("expected fallback auto")
	}
}

func TestShouldUseWindowsBat(t *testing.T) {
	if !shouldUseWindowsBat("liquibase", "windows-bat") {
		t.Fatalf("expected windows-bat mode to force true")
	}
	if shouldUseWindowsBat("liquibase", "native") {
		t.Fatalf("expected native mode to force false")
	}
	if !shouldUseWindowsBat("C:\\Tools\\liquibase.BAT", "auto") {
		t.Fatalf("expected .bat to use windows mode")
	}
	if shouldUseWindowsBat("liquibase", "auto") {
		t.Fatalf("expected non-bat to use native mode")
	}
}

func TestRunCommandWithSink(t *testing.T) {
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) {})
	cmd := helperCommand(ctx, "line1\nline2\n", "err1\n", 0)
	var lines []string
	ctx = engineRuntime.WithLogSink(ctx, func(line string) { lines = append(lines, line) })
	out, err := runCommandWithSink(ctx, cmd)
	if err != nil {
		t.Fatalf("runCommandWithSink: %v", err)
	}
	if !strings.Contains(out, "line1") || !strings.Contains(out, "err1") {
		t.Fatalf("expected output to include lines, got %q", out)
	}
	if len(lines) == 0 {
		t.Fatalf("expected sink lines")
	}
}

func TestRunCommandWithSinkError(t *testing.T) {
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) {})
	cmd := helperCommand(ctx, "ok\n", "", 2)
	out, err := runCommandWithSink(ctx, cmd)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected output, got %q", out)
	}
}

func TestRunCommandWithSinkNoSink(t *testing.T) {
	cmd := helperCommand(context.Background(), "plain\n", "", 0)
	out, err := runCommandWithSink(context.Background(), cmd)
	if err != nil {
		t.Fatalf("runCommandWithSink: %v", err)
	}
	if strings.TrimSpace(out) != "plain" {
		t.Fatalf("expected output, got %q", out)
	}
}

func TestFormatEnv(t *testing.T) {
	out := formatEnv(map[string]string{" FOO ": "bar", "": "skip"})
	if len(out) != 1 || out[0] != "FOO=bar" {
		t.Fatalf("unexpected env: %v", out)
	}
}

func TestHostLiquibaseRunnerWindowsMode(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var capturedName string
	var capturedArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = append([]string{}, args...)
		return helperCommand(ctx, "", "", 0)
	}

	runner := hostLiquibaseRunner{}
	_, err := runner.Run(context.Background(), LiquibaseRunRequest{
		ExecPath: "C:\\Tools\\liquibase.bat",
		ExecMode: "windows-bat",
		Args:     []string{"update"},
		WorkDir:  "C:\\work",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if capturedName != "cmd.exe" {
		t.Fatalf("expected cmd.exe, got %q", capturedName)
	}
	if len(capturedArgs) < 6 || capturedArgs[0] != "/c" || capturedArgs[1] != "cd" || capturedArgs[2] != "/d" {
		t.Fatalf("unexpected args: %v", capturedArgs)
	}
}

func TestHostLiquibaseRunnerNativeModeUsesWorkDirEnv(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var captured *exec.Cmd
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := helperCommand(ctx, "", "", 0)
		captured = cmd
		return cmd
	}

	workDir := t.TempDir()
	runner := hostLiquibaseRunner{}
	_, err := runner.Run(context.Background(), LiquibaseRunRequest{
		ExecMode: "native",
		Args:     []string{"update"},
		Env:      map[string]string{"FOO": "bar"},
		WorkDir:  workDir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if captured == nil || captured.Dir != workDir {
		t.Fatalf("expected work dir, got %q", captured.Dir)
	}
	found := false
	for _, entry := range captured.Env {
		if entry == "FOO=bar" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected env to include FOO=bar")
	}
}

func TestHostLiquibaseRunnerWindowsModeWithoutWorkDir(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var capturedName string
	var capturedArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = append([]string{}, args...)
		return helperCommand(ctx, "", "", 0)
	}

	runner := hostLiquibaseRunner{}
	_, err := runner.Run(context.Background(), LiquibaseRunRequest{
		ExecPath: "C:\\Tools\\liquibase.bat",
		ExecMode: "windows-bat",
		Args:     []string{"update"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if capturedName != "cmd.exe" {
		t.Fatalf("expected cmd.exe, got %q", capturedName)
	}
	if len(capturedArgs) < 4 || capturedArgs[0] != "/c" || capturedArgs[1] != "call" {
		t.Fatalf("unexpected args: %v", capturedArgs)
	}
}

func TestHostLiquibaseRunnerWindowsModeWithoutWorkDirLogsPreview(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return helperCommand(ctx, "", "", 0)
	}

	var logs []string
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) { logs = append(logs, line) })
	runner := hostLiquibaseRunner{}
	_, err := runner.Run(ctx, LiquibaseRunRequest{
		ExecPath: "C:\\Tools\\liquibase.bat",
		ExecMode: "windows-bat",
		Args:     []string{"update"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !containsLine(logs, "exec: cmd.exe /c call") {
		t.Fatalf("expected no-workdir windows preview log, got %v", logs)
	}
}

func TestHostLiquibaseRunnerDefaultsExecPathAndLogsEmptyArgs(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	var capturedName string
	var capturedArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = append([]string{}, args...)
		return helperCommand(ctx, "", "", 0)
	}

	var logs []string
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) { logs = append(logs, line) })

	runner := hostLiquibaseRunner{}
	_, err := runner.Run(ctx, LiquibaseRunRequest{
		ExecPath: "",
		ExecMode: "native",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if capturedName != "liquibase" {
		t.Fatalf("expected default liquibase exec path, got %q", capturedName)
	}
	if len(capturedArgs) != 0 {
		t.Fatalf("expected no args, got %v", capturedArgs)
	}
	found := false
	for _, line := range logs {
		if line == "exec: raw args=<empty>" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected empty-args log line, got %v", logs)
	}
}

func TestHostLiquibaseRunnerWindowsModeLogsWithArgs(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return helperCommand(ctx, "", "", 0)
	}

	var logs []string
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) { logs = append(logs, line) })
	runner := hostLiquibaseRunner{}
	_, err := runner.Run(ctx, LiquibaseRunRequest{
		ExecPath: "C:\\Tools\\liquibase.bat",
		ExecMode: "windows-bat",
		Args:     []string{"update"},
		WorkDir:  "C:\\work",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected log lines")
	}
	if !containsLine(logs, "exec: raw args=") {
		t.Fatalf("expected raw args log line, got %v", logs)
	}
	if !containsLine(logs, "exec: cmd.exe /c cd /d") {
		t.Fatalf("expected windows command preview log line, got %v", logs)
	}
}

func TestRunCommandWithSinkStartError(t *testing.T) {
	ctx := engineRuntime.WithLogSink(context.Background(), func(line string) {})
	cmd := exec.CommandContext(ctx, "definitely-missing-command-for-test")
	out, err := runCommandWithSink(ctx, cmd)
	if err == nil {
		t.Fatalf("expected start error")
	}
	if out != "" {
		t.Fatalf("expected empty output on start error, got %q", out)
	}
}

func helperCommand(ctx context.Context, stdout string, stderr string, exitCode int) *exec.Cmd {
	args := []string{"-test.run=TestHelperProcess", "--", stdout, stderr, strconv.Itoa(exitCode)}
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	index := -1
	for i, arg := range args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index == -1 || len(args) < index+4 {
		fmt.Fprint(os.Stderr, "missing args")
		os.Exit(2)
	}
	stdout := args[index+1]
	stderr := args[index+2]
	code, _ := strconv.Atoi(args[index+3])
	if stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	os.Exit(code)
}

func containsLine(lines []string, prefix string) bool {
	for _, line := range lines {
		if strings.Contains(line, prefix) {
			return true
		}
	}
	return false
}
