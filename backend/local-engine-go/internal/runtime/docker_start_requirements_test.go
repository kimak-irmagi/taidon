package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerRuntimeStartSkipsInvalidMountsAndMarksReadOnly(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},              // mkdir
			{output: ""},              // chown
			{output: ""},              // chmod
			{output: "container-1\n"}, // docker run
			{output: ""},              // test -f PG_VERSION
			{output: ""},              // pg_ctl start
			{output: "accepting connections\n"},
			{output: "0.0.0.0:5432\n"},
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.Start(context.Background(), StartRequest{
		ImageID: "postgres:17",
		DataDir: dir,
		Mounts: []Mount{
			{HostPath: "", ContainerPath: "/skip"},
			{HostPath: "/host/a", ContainerPath: "/container/a", ReadOnly: true},
			{HostPath: "/host/b", ContainerPath: "/container/b", ReadOnly: false},
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(runner.calls) < 4 {
		t.Fatalf("expected docker run call")
	}
	args := runner.calls[3].args
	if containsFlag(args, "/skip") {
		t.Fatalf("unexpected invalid mount in args: %+v", args)
	}
	if !containsFlag(args, "/host/a:/container/a:ro") {
		t.Fatalf("expected readonly mount, got %+v", args)
	}
	if !containsFlag(args, "/host/b:/container/b") {
		t.Fatalf("expected writable mount, got %+v", args)
	}
}

func TestDockerRuntimeStartPgVersionCheckErrorStopsContainer(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""}, // mkdir
			{output: ""}, // chown
			{output: ""}, // chmod
			{output: "container-1\n"},
			{
				output: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
				err:    errors.New("exec failed"),
			},
			{output: ""}, // stop
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.Start(context.Background(), StartRequest{
		ImageID: "postgres:17",
		DataDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}
}

func TestDockerRuntimeStartRejectsMissingPGVersionWhenInitdbDisabled(t *testing.T) {
	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},              // mkdir
			{output: ""},              // chown
			{output: ""},              // chmod
			{output: "container-1\n"}, // docker run
			{output: "missing", err: errors.New("exit 1")},
			{output: ""}, // stop
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.Start(context.Background(), StartRequest{
		ImageID:     "postgres:17",
		DataDir:     t.TempDir(),
		AllowInitdb: false,
	})
	if err == nil || !strings.Contains(err.Error(), "missing PG_VERSION") {
		t.Fatalf("expected missing PG_VERSION error, got %v", err)
	}
}

func TestDockerRuntimeStartHostAuthErrorStopsContainer(t *testing.T) {
	dir := t.TempDir()
	pgdata := filepath.Join(dir, "pgdata")
	if err := os.MkdirAll(pgdata, 0o700); err != nil {
		t.Fatalf("mkdir pgdata: %v", err)
	}
	// Force ensureHostAuth read failure by creating a directory where pg_hba.conf file is expected.
	if err := os.MkdirAll(filepath.Join(pgdata, "pg_hba.conf"), 0o700); err != nil {
		t.Fatalf("mkdir pg_hba.conf dir: %v", err)
	}

	runner := &fakeRunner{
		responses: []runResponse{
			{output: ""},              // mkdir
			{output: ""},              // chown
			{output: ""},              // chmod
			{output: "container-1\n"}, // docker run
			{output: ""},              // test -f PG_VERSION
			{output: ""},              // stop after host auth error
		},
	}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	_, err := rt.Start(context.Background(), StartRequest{
		ImageID: "postgres:17",
		DataDir: dir,
	})
	if err == nil || !strings.Contains(err.Error(), "pg_hba.conf") {
		t.Fatalf("expected host auth error, got %v", err)
	}
}
