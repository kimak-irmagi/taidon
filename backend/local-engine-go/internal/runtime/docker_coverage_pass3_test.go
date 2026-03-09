package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasPGVersionVariants(t *testing.T) {
	if hasPGVersion("") {
		t.Fatalf("expected empty data dir to be false")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !hasPGVersion(dir) {
		t.Fatalf("expected PG_VERSION in data dir root to be detected")
	}
}

func TestDockerRuntimeStartReturnsPostgresStartError(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},
			{output: ""},
			{output: ""},
			{output: "container-1\n"},
			{output: ""},
			{output: ""},
			{output: "pg_ctl boom\n", err: errors.New("fail")},
			{output: ""},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})

	_, err := rt.Start(context.Background(), StartRequest{ImageID: "image", DataDir: "/data", AllowInitdb: false})
	if err == nil || !strings.Contains(err.Error(), "postgres start failed") {
		t.Fatalf("expected postgres start failure, got %v", err)
	}
}

func TestDockerRuntimeInitdbInContainerBranches(t *testing.T) {
	t.Run("docker unavailable", func(t *testing.T) {
		rt := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{responses: []runResponse{{output: "Cannot connect to the Docker daemon", err: errors.New("fail")}}}})
		if err := rt.initdbInContainer(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
			t.Fatalf("expected docker unavailable error, got %v", err)
		}
	})

	t.Run("missing pg version after initdb", func(t *testing.T) {
		rt := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{responses: []runResponse{{output: ""}, {output: "missing\n", err: errors.New("exit 1")}}}})
		if err := rt.initdbInContainer(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "did not produce PG_VERSION") {
			t.Fatalf("expected PG_VERSION error, got %v", err)
		}
	})

	t.Run("readiness docker unavailable", func(t *testing.T) {
		rt := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{responses: []runResponse{{output: ""}, {output: "Cannot connect to the Docker daemon", err: errors.New("fail")}}}})
		if err := rt.initdbInContainer(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
			t.Fatalf("expected docker unavailable readiness error, got %v", err)
		}
	})
}

func TestDockerRuntimeExecSkipsBlankEnvKeys(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: "ok\n"}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})

	if _, err := rt.Exec(context.Background(), "container-1", ExecRequest{Env: map[string]string{"": "skip", "FOO": "bar"}, Args: []string{"env"}}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected a single runner call, got %+v", runner.calls)
	}
	args := runner.calls[0].args
	if strings.Contains(strings.Join(args, " "), "=skip") {
		t.Fatalf("expected blank env key to be skipped, got %+v", args)
	}
	if !containsArg(args, "-e", "FOO=bar") {
		t.Fatalf("expected FOO env to be passed, got %+v", args)
	}
}

func TestDockerRuntimeRunContainerSkipsBlankEntries(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: "ok\n"}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})

	if _, err := rt.RunContainer(context.Background(), RunRequest{
		ImageID: "image",
		Env:     map[string]string{" ": "skip", "FOO": "bar"},
		Mounts: []Mount{
			{HostPath: " ", ContainerPath: "/mnt/skip"},
			{HostPath: "/host", ContainerPath: " ", ReadOnly: true},
			{HostPath: "/host", ContainerPath: "/mnt", ReadOnly: true},
		},
	}); err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	args := runner.calls[0].args
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "=skip") {
		t.Fatalf("expected blank env key to be skipped, got %+v", args)
	}
	if strings.Count(joined, "-v") != 1 {
		t.Fatalf("expected only one valid mount, got %+v", args)
	}
}

func TestNormalizeDockerHostPathReturnsOriginalWhenNoConversionMatches(t *testing.T) {
	t.Setenv(dockerHostPathStyleEnv, dockerHostPathLinux)
	path := `\\server\share\path`
	if got := normalizeDockerHostPath(path); got != path {
		t.Fatalf("expected unmatched path to remain unchanged, got %q", got)
	}
}
