package runtime

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls     []runCall
	responses []runResponse
}

type runCall struct {
	name  string
	args  []string
	stdin *string
}

type runResponse struct {
	output string
	err    error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string, stdin *string) (string, error) {
	var captured *string
	if stdin != nil {
		value := *stdin
		captured = &value
	}
	f.calls = append(f.calls, runCall{name: name, args: append([]string{}, args...), stdin: captured})
	if len(f.responses) == 0 {
		return "", nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp.output, resp.err
}

func TestDockerRuntimeStart(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "container-1\n"},
			{output: ""},
			{output: "accepting connections\n"},
			{output: "0.0.0.0:54321\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	instance, err := rt.Start(context.Background(), StartRequest{
		ImageID: "postgres:17",
		DataDir: "/data",
		Name:    "sqlrs-test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if instance.ID != "container-1" || instance.Port != 54321 || instance.Host != "127.0.0.1" {
		t.Fatalf("unexpected instance: %+v", instance)
	}
	if len(runner.calls) == 0 {
		t.Fatalf("expected docker run to be called")
	}
	if len(runner.calls) < 4 {
		t.Fatalf("expected docker run calls, got %+v", runner.calls)
	}
	if !containsArg(runner.calls[3].args, "--name", "sqlrs-test") {
		t.Fatalf("expected container name in args: %v", runner.calls[3].args)
	}
	foundMkdir := false
	foundChown := false
	foundChmod := false
	for _, call := range runner.calls[:3] {
		if containsArg(call.args, "mkdir", "-p") {
			foundMkdir = true
		}
		if containsArg(call.args, "chown", "-R") {
			foundChown = true
		}
		if containsArg(call.args, "chmod", "0700") {
			foundChmod = true
		}
	}
	if !foundMkdir || !foundChown || !foundChmod {
		t.Fatalf("expected mkdir/chown/chmod calls, got %+v", runner.calls[:3])
	}
}

func TestDockerRuntimeInitBaseRejectsEmpty(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if err := rt.InitBase(context.Background(), "", "/data"); err == nil {
		t.Fatalf("expected error for empty image id")
	}
	if err := rt.InitBase(context.Background(), "image", ""); err == nil {
		t.Fatalf("expected error for empty data dir")
	}
}

func TestDockerRuntimeInitBaseSuccess(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: ""}, {output: ""}, {output: ""}, {output: ""}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	if len(runner.calls) < 4 || runner.calls[0].args[0] != "run" || runner.calls[1].args[0] != "run" || runner.calls[2].args[0] != "run" || runner.calls[3].args[0] != "run" {
		t.Fatalf("expected docker run calls, got %+v", runner.calls)
	}
	if !containsArg(runner.calls[0].args, "mkdir", "-p") {
		found := false
		for _, arg := range runner.calls[0].args {
			if arg == "mkdir" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected mkdir call, got %+v", runner.calls[0].args)
		}
	}
	if !containsArg(runner.calls[1].args, "chown", "-R") {
		t.Fatalf("expected chown call, got %+v", runner.calls[1].args)
	}
	if !containsArg(runner.calls[2].args, "chmod", "0700") {
		t.Fatalf("expected chmod call, got %+v", runner.calls[2].args)
	}
	if !containsArg(runner.calls[3].args, "initdb", "--username=sqlrs") {
		found := false
		for _, arg := range runner.calls[3].args {
			if arg == "initdb" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected initdb call, got %+v", runner.calls[3].args)
		}
	}
}

func TestDockerRuntimeInitBaseDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{
		{output: "failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine; check if the daemon is running", err: errors.New("fail")},
	}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeInitBasePermissionError(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{
		{output: "chown: changing ownership of '/var/lib/postgresql/data/pgdata': Operation not permitted", err: errors.New("fail")},
	}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "permissions are not supported") {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestDockerRuntimeStartRejectsEmptyInputs(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "", DataDir: "/data"}); err == nil {
		t.Fatalf("expected error for empty image id")
	}
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: ""}); err == nil {
		t.Fatalf("expected error for empty data dir")
	}
}

func TestDockerRuntimeStartRunError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "boom\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeStartDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{output: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?", err: errors.New("fail")}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data"}); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeStartEmptyContainerID(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: ""},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeStartExecError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "container-1\n"},
			{output: "boom\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeStartPortParseError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "container-1\n"},
			{output: ""},
			{output: "accepting connections\n"},
			{output: "invalid\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeExecRejectsEmptyID(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if _, err := rt.Exec(context.Background(), "", ExecRequest{}); err == nil {
		t.Fatalf("expected error for empty container id")
	}
}

func TestDockerRuntimeExecReportsRunnerError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{output: "boom\n", err: errors.New("fail")}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Exec(context.Background(), "container-1", ExecRequest{Args: []string{"pg_isready"}}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error with output, got %v", err)
	}
}

func TestDockerRuntimeExecIncludesEnvDirStdin(t *testing.T) {
	input := "select 1;"
	runner := &fakeRunner{
		responses: []runResponse{{output: "ok\n"}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.Exec(context.Background(), "container-1", ExecRequest{
		User:  "postgres",
		Args:  []string{"psql", "-c", "select 1"},
		Env:   map[string]string{"FOO": "bar"},
		Dir:   "/work",
		Stdin: &input,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected single exec call, got %+v", runner.calls)
	}
	args := runner.calls[0].args
	if !containsArg(args, "-w", "/work") {
		t.Fatalf("expected workdir flag, got %+v", args)
	}
	if !containsArg(args, "-e", "FOO=bar") {
		t.Fatalf("expected env flag, got %+v", args)
	}
	if !containsFlag(args, "-i") {
		t.Fatalf("expected stdin flag, got %+v", args)
	}
	if runner.calls[0].stdin == nil || *runner.calls[0].stdin != input {
		t.Fatalf("expected stdin to be forwarded, got %+v", runner.calls[0].stdin)
	}
}

func TestDockerRuntimeStopEmptyID(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if err := rt.Stop(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerRuntimeStopIgnoresMissingContainer(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{
			output: "Error response from daemon: No such container: container-1\n",
			err:    errors.New("exit 1"),
		}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.Stop(context.Background(), "container-1"); err != nil {
		t.Fatalf("expected missing container to be ignored, got %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0].args[0] != "stop" {
		t.Fatalf("expected docker stop call, got %+v", runner.calls)
	}
}

func TestDockerRuntimeWaitForReadyTimeout(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "nope\n", err: errors.New("fail")},
			{output: "nope\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.WaitForReady(context.Background(), "container-1", time.Millisecond); err == nil || !strings.Contains(err.Error(), "postgres readiness failed") {
		t.Fatalf("expected readiness failure, got %v", err)
	}
}

func TestDockerRuntimeWaitForReadySuccess(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "accepting connections\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.WaitForReady(context.Background(), "container-1", time.Second); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
}

func TestDockerRuntimeRunPermissionCommandDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{err: DockerUnavailableError{Message: "daemon unavailable"}},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.runPermissionCommand(context.Background(), []string{"run"}); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeRunPermissionCommandPermissionError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "chmod: operation not permitted", err: errors.New("exit 1")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.runPermissionCommand(context.Background(), []string{"run"}); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestDockerRuntimeResolveImageWithDigest(t *testing.T) {
	runner := &fakeRunner{}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	imageID := "image-1@sha256:abc"
	resolved, err := rt.ResolveImage(context.Background(), imageID)
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if resolved != imageID {
		t.Fatalf("unexpected resolved image: %s", resolved)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no runner calls, got %+v", runner.calls)
	}
}

func TestDockerRuntimeResolveImageInspectSuccess(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{output: "repo@sha256:abc\n"}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	resolved, err := rt.ResolveImage(context.Background(), "image-1")
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if resolved != "repo@sha256:abc" {
		t.Fatalf("unexpected resolved image: %s", resolved)
	}
	if len(runner.calls) != 1 || runner.calls[0].args[0] != "image" {
		t.Fatalf("expected image inspect call, got %+v", runner.calls)
	}
}

func TestDockerRuntimeResolveImagePullOnInspectError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "no such image\n", err: errors.New("fail")},
			{output: "pulled\n"},
			{output: "repo@sha256:resolved\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	resolved, err := rt.ResolveImage(context.Background(), "image-1")
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if resolved != "repo@sha256:resolved" {
		t.Fatalf("unexpected resolved image: %s", resolved)
	}
	if len(runner.calls) != 3 || runner.calls[1].args[0] != "pull" {
		t.Fatalf("expected inspect, pull, inspect calls, got %+v", runner.calls)
	}
}

func TestDockerRuntimeResolveImageDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "Cannot connect to the Docker daemon", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.ResolveImage(context.Background(), "image-1"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestParseHostPort(t *testing.T) {
	port, err := parseHostPort("0.0.0.0:5432\n")
	if err != nil || port != 5432 {
		t.Fatalf("unexpected parse result: port=%d err=%v", port, err)
	}
	if _, err := parseHostPort(""); err == nil {
		t.Fatalf("expected error for empty output")
	}
	if _, err := parseHostPort("not-a-port"); err == nil {
		t.Fatalf("expected error for invalid output")
	}
}

func TestIsDockerUnavailableOutput(t *testing.T) {
	if !isDockerUnavailableOutput("Cannot connect to the Docker daemon", errors.New("fail")) {
		t.Fatalf("expected unavailable for docker daemon string")
	}
	if isDockerUnavailableOutput("boom", errors.New("fail")) {
		t.Fatalf("expected unavailable false for unrelated error")
	}
	if isDockerUnavailableOutput("", nil) {
		t.Fatalf("expected unavailable false for empty output")
	}
	if !isDockerUnavailableOutput("failed to connect to the docker api", errors.New("fail")) {
		t.Fatalf("expected unavailable for docker api string")
	}
}

func TestIsDockerNotFoundOutput(t *testing.T) {
	if !isDockerNotFoundOutput("Error response from daemon: No such container: abc", errors.New("exit 1")) {
		t.Fatalf("expected no such container to be detected")
	}
	if !isDockerNotFoundOutput("container abc is not running", errors.New("exit 1")) {
		t.Fatalf("expected is not running container to be detected")
	}
	if isDockerNotFoundOutput("boom", errors.New("fail")) {
		t.Fatalf("expected unrelated error to be false")
	}
	if isDockerNotFoundOutput("", nil) {
		t.Fatalf("expected empty output to be false")
	}
}

func TestExecRunnerRun(t *testing.T) {
	runner := execRunner{}
	cmd, args := shellCommand("echo ok")
	output, err := runner.Run(context.Background(), cmd, args, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(output, "ok") {
		t.Fatalf("expected output to include ok, got %q", output)
	}
}

func TestExecRunnerRunError(t *testing.T) {
	runner := execRunner{}
	cmd, args := shellCommand("echo err 1>&2 && exit 1")
	output, err := runner.Run(context.Background(), cmd, args, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(output), "err") {
		t.Fatalf("expected stderr in output, got %q", output)
	}
}

func TestExecRunnerRunStreaming(t *testing.T) {
	runner := execRunner{}
	cmd, args := shellCommand("echo out && echo err 1>&2")
	var lines []string
	output, err := runner.RunStreaming(context.Background(), cmd, args, nil, func(line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("RunStreaming: %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("expected streamed lines, got %+v", lines)
	}
	if !strings.Contains(output, "out") || !strings.Contains(output, "err") {
		t.Fatalf("expected output to include stdout/stderr, got %q", output)
	}
}

func TestExecRunnerRunStreamingWithStdin(t *testing.T) {
	runner := execRunner{}
	cmd := "sh"
	args := []string{"-c", "read x; echo $x"}
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/V:ON", "/C", "set /p x= & echo !x!"}
	}
	input := "hello"
	var lines []string
	output, err := runner.RunStreaming(context.Background(), cmd, args, &input, func(line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("RunStreaming: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output to include stdin, got %q", output)
	}
	if len(lines) == 0 || lines[0] != "hello" {
		t.Fatalf("expected stdin line in sink, got %+v", lines)
	}
}

func TestExecRunnerRunStreamingNilSink(t *testing.T) {
	runner := execRunner{}
	cmd, args := shellCommand("echo ok")
	if _, err := runner.RunStreaming(context.Background(), cmd, args, nil, nil); err != nil {
		t.Fatalf("RunStreaming nil sink: %v", err)
	}
}

func TestExecRunnerRunStreamingStdoutPipeError(t *testing.T) {
	prevStdout := cmdStdoutPipe
	defer func() { cmdStdoutPipe = prevStdout }()
	cmdStdoutPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		return nil, errors.New("stdout boom")
	}

	runner := execRunner{}
	_, err := runner.RunStreaming(context.Background(), "cmd", []string{"/c", "echo ok"}, nil, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "stdout boom") {
		t.Fatalf("expected stdout pipe error, got %v", err)
	}
}

func TestExecRunnerRunStreamingStderrPipeError(t *testing.T) {
	prevStdout := cmdStdoutPipe
	prevStderr := cmdStderrPipe
	defer func() {
		cmdStdoutPipe = prevStdout
		cmdStderrPipe = prevStderr
	}()
	cmdStdoutPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	cmdStderrPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		return nil, errors.New("stderr boom")
	}

	runner := execRunner{}
	_, err := runner.RunStreaming(context.Background(), "cmd", []string{"/c", "echo ok"}, nil, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "stderr boom") {
		t.Fatalf("expected stderr pipe error, got %v", err)
	}
}

func TestExecRunnerRunStreamingStartError(t *testing.T) {
	prevStdout := cmdStdoutPipe
	prevStderr := cmdStderrPipe
	prevStart := cmdStart
	defer func() {
		cmdStdoutPipe = prevStdout
		cmdStderrPipe = prevStderr
		cmdStart = prevStart
	}()
	cmdStdoutPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	cmdStderrPipe = func(cmd *exec.Cmd) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	cmdStart = func(cmd *exec.Cmd) error {
		return errors.New("start boom")
	}

	runner := execRunner{}
	_, err := runner.RunStreaming(context.Background(), "cmd", []string{"/c", "echo ok"}, nil, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "start boom") {
		t.Fatalf("expected start error, got %v", err)
	}
}

func TestDockerRuntimeLogSink(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{output: "line-1\nline-2\n"}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	var lines []string
	ctx := WithLogSink(context.Background(), func(line string) {
		lines = append(lines, line)
	})
	if _, err := rt.Exec(ctx, "container-1", ExecRequest{Args: []string{"echo", "ok"}}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
	if lines[0] != "line-1" || lines[1] != "line-2" {
		t.Fatalf("unexpected log lines: %+v", lines)
	}
}

func TestIsInitdbPermissionOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "empty", output: "", want: false},
		{name: "initdb-permissions", output: "initdb: error: could not change permissions of directory", want: true},
		{name: "chown-not-permitted", output: "chown: changing ownership of '/var/lib/postgresql/data/pgdata': Operation not permitted", want: true},
		{name: "chmod-not-permitted", output: "chmod: changing permissions of '/var/lib/postgresql/data/pgdata': Operation not permitted", want: true},
		{name: "generic-permissions", output: "Operation not permitted: permissions data", want: true},
		{name: "pgdata-path", output: "operation not permitted " + PostgresDataDir, want: true},
		{name: "unrelated", output: "permission denied for user", want: false},
	}
	for _, testCase := range cases {
		if got := isInitdbPermissionOutput(testCase.output); got != testCase.want {
			t.Fatalf("%s: expected %v, got %v", testCase.name, testCase.want, got)
		}
	}
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", command}
	}
	return "sh", []string{"-c", command}
}

func containsArg(args []string, key string, value string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == key && i+1 < len(args) && args[i+1] == value {
			return true
		}
		if strings.HasPrefix(args[i], key+"=") && strings.TrimPrefix(args[i], key+"=") == value {
			return true
		}
	}
	return false
}

func containsFlag(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
