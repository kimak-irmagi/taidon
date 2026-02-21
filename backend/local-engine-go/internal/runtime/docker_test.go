package runtime

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

type streamingFakeRunner struct {
	runCalls     int
	streamCalls  int
	output       string
	err          error
	streamedLine string
}

func (s *streamingFakeRunner) Run(ctx context.Context, name string, args []string, stdin *string) (string, error) {
	s.runCalls++
	return s.output, s.err
}

func (s *streamingFakeRunner) RunStreaming(ctx context.Context, name string, args []string, stdin *string, sink LogSink) (string, error) {
	s.streamCalls++
	if sink != nil && s.streamedLine != "" {
		sink(s.streamedLine)
	}
	return s.output, s.err
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
			{output: ""},
			{output: "accepting connections\n"},
			{output: "0.0.0.0:54321\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	instance, err := rt.Start(context.Background(), StartRequest{
		ImageID:     "postgres:17",
		DataDir:     "/data",
		Name:        "sqlrs-test",
		AllowInitdb: false,
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
	if len(runner.calls) < 7 {
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
		if containsFlag(call.args, "chmod") && containsFlag(call.args, "0700") {
			foundChmod = true
		}
	}
	if !foundMkdir || !foundChown || !foundChmod {
		t.Fatalf("expected mkdir/chown/chmod calls, got %+v", runner.calls[:5])
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
	runner := &fakeRunner{responses: []runResponse{
		{output: ""}, {output: ""}, {output: ""},
		{output: "missing\n", err: errors.New("exit 1")},
		{output: ""},
		{output: ""},
	}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	if len(runner.calls) < 6 || runner.calls[0].args[0] != "run" || runner.calls[1].args[0] != "run" || runner.calls[2].args[0] != "run" || runner.calls[3].args[0] != "run" || runner.calls[4].args[0] != "run" || runner.calls[5].args[0] != "run" {
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
	if !containsFlag(runner.calls[1].args, PostgresDataDir) {
		t.Fatalf("expected chown target %q, got %+v", PostgresDataDir, runner.calls[1].args)
	}
	if !(containsFlag(runner.calls[2].args, "chmod") && containsFlag(runner.calls[2].args, "0700")) {
		t.Fatalf("expected chmod call, got %+v", runner.calls[2].args)
	}
	if !containsFlag(runner.calls[2].args, PostgresDataDir) {
		t.Fatalf("expected chmod target %q, got %+v", PostgresDataDir, runner.calls[2].args)
	}
	if !containsArg(runner.calls[4].args, "initdb", "--username=sqlrs") {
		found := false
		for _, arg := range runner.calls[4].args {
			if arg == "initdb" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected initdb call, got %+v", runner.calls[4].args)
		}
	}
}

func TestDockerRuntimeInitBaseSkipsWhenPGVersionExists(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pgdata, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write pg_version: %v", err)
	}
	runner := &fakeRunner{responses: []runResponse{{output: ""}, {output: ""}, {output: ""}, {output: ""}, {output: ""}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", dir); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	expectedCalls := 3
	if runtime.GOOS == "linux" {
		expectedCalls = 4
	}
	if len(runner.calls) != expectedCalls {
		t.Fatalf("expected only permission calls, got %+v", runner.calls)
	}
	for _, call := range runner.calls {
		if containsArg(call.args, "initdb", "--username=sqlrs") {
			t.Fatalf("expected initdb to be skipped, got %+v", call.args)
		}
	}
}

func TestEnsureHostAuthAddsEntries(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("local all all trust\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba: %v", err)
	}
	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("ensureHostAuth: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "0.0.0.0/0") || !strings.Contains(text, "::/0") {
		t.Fatalf("expected host entries, got %q", text)
	}
}

func TestEnsureHostAuthAddsNewlineBeforeAppendedEntries(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("local all all trust"), 0o600); err != nil {
		t.Fatalf("write pg_hba: %v", err)
	}
	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("ensureHostAuth: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "trust\nhost all all 0.0.0.0/0 trust") {
		t.Fatalf("expected newline before appended host entries, got %q", text)
	}
}

func TestEnsureHostAuthSkipsWhenMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("expected missing pg_hba to be ignored, got %v", err)
	}
}

func TestEnsureHostAuthReturnsReadError(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir pg_hba as dir: %v", err)
	}
	if err := ensureHostAuth(dir); err == nil {
		t.Fatalf("expected read error for directory path")
	}
}

func TestEnsureHostAuthSkipsPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows does not enforce unix permission bits")
	}

	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("local all all trust\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod pg_hba: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()

	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("expected permission denied to be ignored, got %v", err)
	}
}

func TestEnsureHostAuthRequiresDataDir(t *testing.T) {
	if err := ensureHostAuth(""); err == nil {
		t.Fatalf("expected error for empty data dir")
	}
}

func TestEnsureHostAuthIdempotent(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	initial := "host all all 0.0.0.0/0 trust\nhost all all ::/0 trust\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write pg_hba: %v", err)
	}
	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("ensureHostAuth: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba: %v", err)
	}
	if string(content) != initial {
		t.Fatalf("expected pg_hba to be unchanged, got %q", string(content))
	}
}

func TestEnsureHostAuthAddsOnlyMissingAddressFamily(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pgdata, "pg_hba.conf")
	initial := "host all all 0.0.0.0/0 trust\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write pg_hba: %v", err)
	}
	if err := ensureHostAuth(dir); err != nil {
		t.Fatalf("ensureHostAuth: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba: %v", err)
	}
	text := string(content)
	if strings.Count(text, "0.0.0.0/0") != 1 {
		t.Fatalf("expected ipv4 entry to stay single, got %q", text)
	}
	if strings.Count(text, "::/0") != 1 {
		t.Fatalf("expected exactly one appended ipv6 entry, got %q", text)
	}
}

func TestPgDataHostDir(t *testing.T) {
	dir := t.TempDir()
	expected := filepath.Join(dir, "pgdata")
	if got := pgDataHostDir(dir); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDockerRuntimeInitBaseSkipsWhenPGVersionExistsInContainer(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{responses: []runResponse{{output: ""}, {output: ""}, {output: ""}, {output: ""}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", dir); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	expectedCalls := 4
	if runtime.GOOS == "linux" {
		expectedCalls = 5
	}
	if len(runner.calls) != expectedCalls {
		t.Fatalf("expected permission calls + pgversion check, got %+v", runner.calls)
	}
	if containsArg(runner.calls[len(runner.calls)-1].args, "initdb", "--username=sqlrs") {
		t.Fatalf("expected initdb to be skipped")
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
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "", DataDir: "/data", AllowInitdb: false}); err == nil {
		t.Fatalf("expected error for empty image id")
	}
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "", AllowInitdb: false}); err == nil {
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
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeStartDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{{output: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?", err: errors.New("fail")}},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil || !strings.Contains(err.Error(), "docker is not running") {
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
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil {
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
			{output: ""},
			{output: "boom\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDockerRuntimeStartRunsInitdbWhenMissing(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "container-1\n"},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "accepting connections\n"},
			{output: "0.0.0.0:54321\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: true}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	found := false
	for _, call := range runner.calls {
		if containsArg(call.args, "initdb", "--username=sqlrs") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected initdb exec call, got %+v", runner.calls)
	}
}

func TestDockerRuntimeStartInitdbPermissionError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "container-1\n"},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: "initdb: error: could not change permissions of directory", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: true}); err == nil || !strings.Contains(err.Error(), "permissions are not supported") {
		t.Fatalf("expected permission error, got %v", err)
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
			{output: ""},
			{output: "accepting connections\n"},
			{output: "invalid\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil {
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

func TestDockerRuntimeRunContainerRejectsEmptyImage(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if _, err := rt.RunContainer(context.Background(), RunRequest{}); err == nil {
		t.Fatalf("expected error for empty image id")
	}
}

func TestDockerRuntimeRunContainerBuildsArgs(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: "ok\n"}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.RunContainer(context.Background(), RunRequest{
		ImageID: "liquibase:latest",
		Args:    []string{"update"},
		Env:     map[string]string{"FOO": "bar"},
		Dir:     "/work",
		User:    "liquibase",
		Name:    "sqlrs-liquibase",
		Network: "container:pg-1",
		Mounts: []Mount{
			{HostPath: "/host", ContainerPath: "/mnt", ReadOnly: true},
		},
	})
	if err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected single run call, got %+v", runner.calls)
	}
	args := runner.calls[0].args
	if args[0] != "run" || args[1] != "--rm" {
		t.Fatalf("expected docker run args, got %+v", args[:2])
	}
	if !containsArg(args, "--name", "sqlrs-liquibase") {
		t.Fatalf("expected name flag, got %+v", args)
	}
	if !containsArg(args, "-u", "liquibase") {
		t.Fatalf("expected user flag, got %+v", args)
	}
	if !containsArg(args, "-w", "/work") {
		t.Fatalf("expected workdir flag, got %+v", args)
	}
	if !containsArg(args, "--network", "container:pg-1") {
		t.Fatalf("expected network flag, got %+v", args)
	}
	if !containsArg(args, "-e", "FOO=bar") {
		t.Fatalf("expected env flag, got %+v", args)
	}
	if !containsArg(args, "-v", "/host:/mnt:ro") {
		t.Fatalf("expected mount flag, got %+v", args)
	}
	if !containsFlag(args, "liquibase:latest") || !containsFlag(args, "update") {
		t.Fatalf("expected image and args, got %+v", args)
	}
}

func TestWindowsDrivePathToLinux(t *testing.T) {
	got, ok := windowsDrivePathToLinux(`D:\a\temp\store`)
	if !ok {
		t.Fatalf("expected conversion to succeed")
	}
	if got != "/mnt/d/a/temp/store" {
		t.Fatalf("unexpected converted path: %s", got)
	}
	root, ok := windowsDrivePathToLinux(`C:\`)
	if !ok || root != "/mnt/c" {
		t.Fatalf("unexpected root conversion: ok=%v path=%s", ok, root)
	}
	if _, ok := windowsDrivePathToLinux(`/var/tmp/store`); ok {
		t.Fatalf("expected non-drive path to skip conversion")
	}
}

func TestWindowsWSLUNCPathToLinux(t *testing.T) {
	got, ok := windowsWSLUNCPathToLinux(`\\wsl$\Ubuntu-24.04\tmp\sqlrs\store`)
	if !ok {
		t.Fatalf("expected UNC conversion to succeed")
	}
	if got != "/tmp/sqlrs/store" {
		t.Fatalf("unexpected UNC conversion: %s", got)
	}
	got, ok = windowsWSLUNCPathToLinux(`\\wsl.localhost\Ubuntu\var\lib\sqlrs`)
	if !ok {
		t.Fatalf("expected localhost UNC conversion to succeed")
	}
	if got != "/var/lib/sqlrs" {
		t.Fatalf("unexpected localhost UNC conversion: %s", got)
	}
	if _, ok := windowsWSLUNCPathToLinux(`\\server\share\path`); ok {
		t.Fatalf("expected non-WSL UNC path to skip conversion")
	}
}

func TestDockerBindSpecUsesLinuxPathStyleOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path normalization")
	}
	t.Setenv(dockerHostPathStyleEnv, dockerHostPathLinux)
	spec := dockerBindSpec(`D:\a\temp\store`, PostgresDataDirRoot, false)
	if spec != "/mnt/d/a/temp/store:"+PostgresDataDirRoot {
		t.Fatalf("unexpected bind spec: %s", spec)
	}
	readOnly := dockerBindSpec(`C:\workspace`, "/work", true)
	if readOnly != "/mnt/c/workspace:/work:ro" {
		t.Fatalf("unexpected readonly bind spec: %s", readOnly)
	}
	unc := dockerBindSpec(`\\wsl$\Ubuntu-24.04\tmp\sqlrs\store`, PostgresDataDirRoot, false)
	if unc != "/tmp/sqlrs/store:"+PostgresDataDirRoot {
		t.Fatalf("unexpected UNC bind spec: %s", unc)
	}
}

func TestDockerRuntimeRunContainerConvertsMountsForLinuxPathStyleOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path normalization")
	}
	t.Setenv(dockerHostPathStyleEnv, dockerHostPathLinux)
	runner := &fakeRunner{responses: []runResponse{{output: "ok\n"}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.RunContainer(context.Background(), RunRequest{
		ImageID: "liquibase:latest",
		Mounts: []Mount{
			{HostPath: `D:\work\project`, ContainerPath: "/work", ReadOnly: true},
		},
	})
	if err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one docker call, got %d", len(runner.calls))
	}
	if !containsArg(runner.calls[0].args, "-v", "/mnt/d/work/project:/work:ro") {
		t.Fatalf("expected converted mount path, got %+v", runner.calls[0].args)
	}
}

func TestDockerRuntimeRunContainerDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{err: DockerUnavailableError{Message: "daemon unavailable"}}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.RunContainer(context.Background(), RunRequest{ImageID: "image"}); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeRunContainerRunError(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: "boom\n", err: errors.New("fail")}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.RunContainer(context.Background(), RunRequest{ImageID: "image"}); err == nil || !strings.Contains(err.Error(), "docker run failed") {
		t.Fatalf("expected run error, got %v", err)
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
	port, err = parseHostPort("invalid\n0.0.0.0:5433\n")
	if err != nil || port != 5433 {
		t.Fatalf("unexpected parse result: port=%d err=%v", port, err)
	}
	port, err = parseHostPort("\n \n0.0.0.0:5434\n")
	if err != nil || port != 5434 {
		t.Fatalf("unexpected parse result with empty lines: port=%d err=%v", port, err)
	}
	port, err = parseHostPort("invalid\n \n0.0.0.0:5435\n")
	if err != nil || port != 5435 {
		t.Fatalf("unexpected parse result with middle blank line: port=%d err=%v", port, err)
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
	if !isDockerUnavailableOutput("", errors.New("npipe:////./pipe/dockerDesktopLinuxEngine")) {
		t.Fatalf("expected unavailable for npipe docker desktop string")
	}
	if !isDockerUnavailableOutput("Is the docker daemon running?", errors.New("fail")) {
		t.Fatalf("expected unavailable for daemon running hint")
	}
	if !isDockerUnavailableOutput("Cannot connect to the Docker daemon", nil) {
		t.Fatalf("expected unavailable for daemon string even without wrapped error")
	}
	if !isDockerUnavailableOutput("", errors.New("npipe docker pipe")) {
		t.Fatalf("expected unavailable for npipe+docker+pipe pattern")
	}
	if isDockerUnavailableOutput("   ", emptyErr{}) {
		t.Fatalf("expected unavailable false for effectively empty combined value")
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

type emptyErr struct{}

func (emptyErr) Error() string { return "" }

func TestDockerUnavailableErrorMessage(t *testing.T) {
	if (DockerUnavailableError{}).Error() != "docker daemon unavailable" {
		t.Fatalf("expected default docker unavailable message")
	}
	if (DockerUnavailableError{Message: "  custom  "}).Error() != "custom" {
		t.Fatalf("expected trimmed message")
	}
}

func TestDockerUnavailableHint(t *testing.T) {
	if hint := dockerUnavailableHint("npipe:////./pipe/dockerDesktopLinuxEngine", nil); hint != "start Docker Desktop and retry" {
		t.Fatalf("unexpected hint: %s", hint)
	}
	if hint := dockerUnavailableHint("Cannot connect to unix:///var/run/docker.sock", nil); hint != "start the Docker daemon and retry" {
		t.Fatalf("unexpected hint: %s", hint)
	}
	if hint := dockerUnavailableHint("something else", nil); hint != "start Docker and retry" {
		t.Fatalf("unexpected hint: %s", hint)
	}
}

func TestIsDockerUnavailable(t *testing.T) {
	if !isDockerUnavailable(DockerUnavailableError{}) {
		t.Fatalf("expected docker unavailable to be detected")
	}
	if isDockerUnavailable(errors.New("boom")) {
		t.Fatalf("expected docker unavailable false for generic error")
	}
}

func TestWrapDockerError(t *testing.T) {
	err := wrapDockerError(errors.New("fail"), "boom")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected output to be included, got %v", err)
	}
	err = wrapDockerError(errors.New("fail"), "")
	if err == nil || err.Error() != "fail" {
		t.Fatalf("expected original error, got %v", err)
	}
	err = wrapDockerError(errors.New("fail"), "Cannot connect to the Docker daemon")
	if !isDockerUnavailable(err) {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeRunUsesStreamingRunner(t *testing.T) {
	runner := &streamingFakeRunner{output: "ok\n", streamedLine: "streamed"}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	ctx := WithLogSink(context.Background(), func(string) {})
	if _, err := rt.Exec(ctx, "container-1", ExecRequest{Args: []string{"echo", "ok"}}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if runner.streamCalls != 1 {
		t.Fatalf("expected streaming runner to be used")
	}
	if runner.runCalls != 0 {
		t.Fatalf("expected non-streaming runner to be unused")
	}
}

func TestDockerRuntimeRunStreamingErrorWrapsUnavailable(t *testing.T) {
	runner := &streamingFakeRunner{
		output: "Cannot connect to the Docker daemon",
		err:    errors.New("fail"),
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	ctx := WithLogSink(context.Background(), func(string) {})
	if _, err := rt.Exec(ctx, "container-1", ExecRequest{Args: []string{"echo", "ok"}}); err == nil || !isDockerUnavailable(err) {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeRunMountError(t *testing.T) {
	prev := ensureMountFn
	ensureMountFn = func() error {
		return errors.New("mount fail")
	}
	t.Cleanup(func() { ensureMountFn = prev })

	runner := &fakeRunner{}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Exec(context.Background(), "container-1", ExecRequest{Args: []string{"echo", "ok"}}); err == nil || !strings.Contains(err.Error(), "mount fail") {
		t.Fatalf("expected mount error, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no docker calls, got %+v", runner.calls)
	}
}

func TestNewDockerDefaults(t *testing.T) {
	prevLookPath := execLookPath
	t.Cleanup(func() { execLookPath = prevLookPath })
	execLookPath = func(string) (string, error) {
		return "/usr/bin/docker", nil
	}
	rt := NewDocker(Options{})
	if rt.binary != defaultDockerBinary {
		t.Fatalf("expected default binary, got %s", rt.binary)
	}
	if _, ok := rt.runner.(execRunner); !ok {
		t.Fatalf("expected execRunner by default")
	}
}

func TestDockerRuntimeResolveImageRejectsEmpty(t *testing.T) {
	rt := NewDocker(Options{Runner: &fakeRunner{}})
	if _, err := rt.ResolveImage(context.Background(), " "); err == nil {
		t.Fatalf("expected error for empty image id")
	}
}

func TestNewDockerPrefersLinuxDockerOverWindowsInterop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only behavior")
	}
	prevLookPath := execLookPath
	prevStat := osStat
	t.Cleanup(func() {
		execLookPath = prevLookPath
		osStat = prevStat
	})
	execLookPath = func(string) (string, error) {
		return "C:\\Program Files\\Docker\\docker.exe", nil
	}
	osStat = func(name string) (os.FileInfo, error) {
		if name == "/usr/bin/docker" {
			return fakeFileInfo{name: name}, nil
		}
		return nil, os.ErrNotExist
	}

	rt := NewDocker(Options{})
	if rt.binary != "/usr/bin/docker" {
		t.Fatalf("expected /usr/bin/docker, got %s", rt.binary)
	}
}

func TestDockerRuntimeResolveImagePullError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "no such image\n", err: errors.New("fail")},
			{output: "pull failed\n", err: errors.New("boom")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.ResolveImage(context.Background(), "image-1"); err == nil || !strings.Contains(err.Error(), "docker pull failed") {
		t.Fatalf("expected pull failure, got %v", err)
	}
}

func TestDockerRuntimeResolveImageInspectAfterPullDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "no such image\n", err: errors.New("fail")},
			{output: "pulled\n"},
			{output: "Cannot connect to the Docker daemon", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.ResolveImage(context.Background(), "image-1"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeResolveImageEmptyDigest(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "\n"},
			{output: "pulled\n"},
			{output: "\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.ResolveImage(context.Background(), "image-1"); err == nil || !strings.Contains(err.Error(), "image digest is empty") {
		t.Fatalf("expected empty digest error, got %v", err)
	}
}

func TestDockerRuntimeResolveImageInspectAfterPullError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "no such image\n", err: errors.New("fail")},
			{output: "pulled\n"},
			{output: "inspect boom\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.ResolveImage(context.Background(), "image-1"); err == nil || !strings.Contains(err.Error(), "docker inspect failed") {
		t.Fatalf("expected inspect error, got %v", err)
	}
}

func TestDockerRuntimeInitBaseEnsureDataDirOwnerError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "boom\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "data directory setup failed") {
		t.Fatalf("expected setup error, got %v", err)
	}
}

func TestDockerRuntimeInitBaseInitdbError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: "initdb boom\n", err: errors.New("fail")},
			{output: "missing\n", err: errors.New("exit 1")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "initdb failed") {
		t.Fatalf("expected initdb error, got %v", err)
	}
}

func TestDockerRuntimeInitBaseInitdbDockerUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: "Cannot connect to the Docker daemon", err: errors.New("fail")},
			{output: "missing\n", err: errors.New("exit 1")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeInitBaseInitdbPermissionOutput(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: "initdb: error: could not change permissions of directory", err: errors.New("fail")},
			{output: "missing\n", err: errors.New("exit 1")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.InitBase(context.Background(), "image", "/data"); err == nil || !strings.Contains(err.Error(), "permissions are not supported") {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestDockerRuntimeStartPortCommandError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "container-1\n"},
			{output: ""},
			{output: ""},
			{output: "accepting connections\n"},
			{output: "port error\n", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil || !strings.Contains(err.Error(), "docker port failed") {
		t.Fatalf("expected docker port error, got %v", err)
	}
}

func TestDockerRuntimeStartDockerRunUnavailable(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "Cannot connect to the Docker daemon", err: errors.New("fail")},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if _, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeStartWaitForReadyCanceled(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "container-1\n"},
			{output: ""},
			{output: ""},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := rt.Start(ctx, StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false}); err == nil {
		t.Fatalf("expected context cancellation error")
	}
}

func TestDockerRuntimeWaitForReadyTimeoutWithoutError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "nope\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.WaitForReady(context.Background(), "container-1", time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestDockerRuntimeWaitForReadyContextCancelled(t *testing.T) {
	runner := &fakeRunner{}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := rt.WaitForReady(ctx, "container-1", time.Second); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestDockerRuntimeWaitForReadyDefaultTimeout(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: "accepting connections\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.WaitForReady(context.Background(), "container-1", 0); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
}

func TestExecRunnerRunWithStdin(t *testing.T) {
	runner := execRunner{}
	cmd := "sh"
	args := []string{"-c", "read x; echo $x"}
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/V:ON", "/C", "set /p x= & echo !x!"}
	}
	input := "hello"
	output, err := runner.Run(context.Background(), cmd, args, &input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output to include stdin, got %q", output)
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

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Now() }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }
