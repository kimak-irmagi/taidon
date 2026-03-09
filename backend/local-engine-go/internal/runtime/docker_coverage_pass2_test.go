package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDockerRuntimeEnsureDataDirOwnerReturnsLaterCommandErrors(t *testing.T) {
	t.Run("chown", func(t *testing.T) {
		runner := &fakeRunner{
			responses: []runResponse{
				{output: ""},
				{output: "boom\n", err: errors.New("fail")},
			},
		}
		rt := NewDocker(Options{Binary: "docker", Runner: runner})
		if err := rt.ensureDataDirOwner(context.Background(), "image", t.TempDir()); err == nil || !strings.Contains(err.Error(), "data directory setup failed") {
			t.Fatalf("expected chown failure, got %v", err)
		}
	})

	t.Run("chmod", func(t *testing.T) {
		runner := &fakeRunner{
			responses: []runResponse{
				{output: ""},
				{output: ""},
				{output: "boom\n", err: errors.New("fail")},
			},
		}
		rt := NewDocker(Options{Binary: "docker", Runner: runner})
		if err := rt.ensureDataDirOwner(context.Background(), "image", t.TempDir()); err == nil || !strings.Contains(err.Error(), "data directory setup failed") {
			t.Fatalf("expected chmod failure, got %v", err)
		}
	})
}

func TestDockerRuntimeInitBaseHandlesRemainingPgVersionBranches(t *testing.T) {
	t.Run("pg version appears after initdb failure", func(t *testing.T) {
		runner := &fakeRunner{responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: "initdb boom\n", err: errors.New("fail")},
			{output: ""},
		}}
		rt := NewDocker(Options{Binary: "docker", Runner: runner})
		if err := rt.InitBase(context.Background(), "image", t.TempDir()); err != nil {
			t.Fatalf("expected initdb fallback success, got %v", err)
		}
	})

	t.Run("pg version check errors after initdb", func(t *testing.T) {
		runner := &fakeRunner{responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: ""},
			{output: "Cannot connect to the Docker daemon", err: errors.New("fail")},
		}}
		rt := NewDocker(Options{Binary: "docker", Runner: runner})
		if err := rt.InitBase(context.Background(), "image", t.TempDir()); err == nil || !strings.Contains(err.Error(), "docker is not running") {
			t.Fatalf("expected pg version check error, got %v", err)
		}
	})

	t.Run("pg version still missing after initdb", func(t *testing.T) {
		runner := &fakeRunner{responses: []runResponse{
			{output: ""}, {output: ""}, {output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
			{output: ""},
			{output: "missing\n", err: errors.New("exit 1")},
		}}
		rt := NewDocker(Options{Binary: "docker", Runner: runner})
		if err := rt.InitBase(context.Background(), "image", t.TempDir()); err == nil || !strings.Contains(err.Error(), "did not produce PG_VERSION") {
			t.Fatalf("expected missing PG_VERSION error, got %v", err)
		}
	})
}

func TestDockerRuntimeEnsureContainerHostAuthBranches(t *testing.T) {
	rt := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{}})
	if err := rt.ensureContainerHostAuth(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "container id is required") {
		t.Fatalf("expected empty container id error, got %v", err)
	}

	unavailable := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{responses: []runResponse{{output: "Cannot connect to the Docker daemon", err: errors.New("fail")}}}})
	if err := unavailable.ensureContainerHostAuth(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("expected docker unavailable error, got %v", err)
	}

	generic := NewDocker(Options{Binary: "docker", Runner: &fakeRunner{responses: []runResponse{{output: "boom\n", err: errors.New("fail")}}}})
	if err := generic.ensureContainerHostAuth(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "pg_hba.conf update failed") {
		t.Fatalf("expected pg_hba update error, got %v", err)
	}
}

func TestDockerRuntimeStopReturnsRunnerError(t *testing.T) {
	runner := &fakeRunner{responses: []runResponse{{output: "boom\n", err: errors.New("fail")}}}
	rt := NewDocker(Options{Binary: "docker", Runner: runner})
	if err := rt.Stop(context.Background(), "container-1"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected stop error, got %v", err)
	}
}
