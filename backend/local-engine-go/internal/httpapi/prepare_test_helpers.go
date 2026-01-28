package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/prepare/queue"
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/snapshot"
	"sqlrs/engine/internal/store"
)

type fakeRuntime struct{}

func (f *fakeRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("17"), 0o600)
}

func (f *fakeRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	return imageID + "@sha256:resolved", nil
}

func (f *fakeRuntime) Start(ctx context.Context, req engineRuntime.StartRequest) (engineRuntime.Instance, error) {
	return engineRuntime.Instance{ID: "container-1", Host: "127.0.0.1", Port: 5432}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRuntime) Exec(ctx context.Context, id string, req engineRuntime.ExecRequest) (string, error) {
	return "", nil
}

func (f *fakeRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

type fakeSnapshot struct{}

func (f *fakeSnapshot) Kind() string {
	return "fake"
}

func (f *fakeSnapshot) Capabilities() snapshot.Capabilities {
	return snapshot.Capabilities{
		RequiresDBStop:       true,
		SupportsWritableClone: true,
		SupportsSendReceive:   false,
	}
}

func (f *fakeSnapshot) Clone(ctx context.Context, srcDir string, destDir string) (snapshot.CloneResult, error) {
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return snapshot.CloneResult{}, err
	}
	return snapshot.CloneResult{
		MountDir: destDir,
		Cleanup: func() error {
			return os.RemoveAll(destDir)
		},
	}, nil
}

func (f *fakeSnapshot) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return os.MkdirAll(destDir, 0o700)
}

func (f *fakeSnapshot) Destroy(ctx context.Context, dir string) error {
	return os.RemoveAll(dir)
}

type fakeDBMS struct{}

func (f *fakeDBMS) PrepareSnapshot(ctx context.Context, instance engineRuntime.Instance) error {
	return nil
}

func (f *fakeDBMS) ResumeSnapshot(ctx context.Context, instance engineRuntime.Instance) error {
	return nil
}

type fakePsqlRunner struct{}

func (f *fakePsqlRunner) Run(ctx context.Context, instance engineRuntime.Instance, req prepare.PsqlRunRequest) (string, error) {
	return "", nil
}

func newPrepareManager(t *testing.T, store store.Store, queueStore queue.Store, opts ...func(*prepare.Options)) *prepare.Manager {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state-store")
	options := prepare.Options{
		Store:          store,
		Queue:          queueStore,
		Runtime:        &fakeRuntime{},
		Snapshot:       &fakeSnapshot{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: stateRoot,
		Psql:           &fakePsqlRunner{},
		Version:        "test",
		Async:          false,
	}
	for _, opt := range opts {
		opt(&options)
	}
	mgr, err := prepare.NewManager(options)
	if err != nil {
		t.Fatalf("prepare manager: %v", err)
	}
	return mgr
}
