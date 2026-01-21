package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
)

type blockingRuntime struct {
	initCalls int
	started   chan struct{}
	proceed   chan struct{}
}

func (b *blockingRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	b.initCalls++
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-b.proceed
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("17"), 0o600)
}

func (b *blockingRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	return imageID, nil
}

func (b *blockingRuntime) Start(ctx context.Context, req engineRuntime.StartRequest) (engineRuntime.Instance, error) {
	return engineRuntime.Instance{}, nil
}

func (b *blockingRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (b *blockingRuntime) Exec(ctx context.Context, id string, req engineRuntime.ExecRequest) (string, error) {
	return "", nil
}

func (b *blockingRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

func TestEnsureBaseStateUsesInitMarker(t *testing.T) {
	runtime := &fakeRuntime{}
	mgr := newManagerWithRuntime(t, runtime)
	baseDir := filepath.Join(t.TempDir(), "base")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, baseInitMarkerName), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := mgr.ensureBaseState(context.Background(), "image-1", baseDir); err != nil {
		t.Fatalf("ensureBaseState: %v", err)
	}
	if len(runtime.initCalls) != 0 {
		t.Fatalf("expected no init calls, got %d", len(runtime.initCalls))
	}
}

func TestEnsureBaseStateCreatesInitMarker(t *testing.T) {
	runtime := &fakeRuntime{}
	mgr := newManagerWithRuntime(t, runtime)
	baseDir := filepath.Join(t.TempDir(), "base")
	if err := mgr.ensureBaseState(context.Background(), "image-1", baseDir); err != nil {
		t.Fatalf("ensureBaseState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, baseInitMarkerName)); err != nil {
		t.Fatalf("expected marker, got %v", err)
	}
	if len(runtime.initCalls) != 1 {
		t.Fatalf("expected init call, got %d", len(runtime.initCalls))
	}
}

func TestEnsureBaseStateSerializesInit(t *testing.T) {
	runtime := &blockingRuntime{
		started: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	mgr := newManagerWithRuntime(t, runtime)
	baseDir := filepath.Join(t.TempDir(), "base")

	done1 := make(chan error, 1)
	go func() {
		done1 <- mgr.ensureBaseState(context.Background(), "image-1", baseDir)
	}()

	select {
	case <-runtime.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("init did not start")
	}

	done2 := make(chan error, 1)
	go func() {
		done2 <- mgr.ensureBaseState(context.Background(), "image-1", baseDir)
	}()

	select {
	case <-done2:
		t.Fatalf("expected second call to wait")
	case <-time.After(100 * time.Millisecond):
	}
	if runtime.initCalls != 1 {
		t.Fatalf("expected single init call, got %d", runtime.initCalls)
	}

	close(runtime.proceed)

	select {
	case err := <-done1:
		if err != nil {
			t.Fatalf("ensureBaseState: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first init")
	}
	select {
	case err := <-done2:
		if err != nil {
			t.Fatalf("ensureBaseState: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for second init")
	}
	if runtime.initCalls != 1 {
		t.Fatalf("expected single init call, got %d", runtime.initCalls)
	}
}

func newManagerWithRuntime(t *testing.T, rt engineRuntime.Runtime) *Manager {
	t.Helper()
	mgr, err := NewManager(Options{
		Store:          &fakeStore{},
		Queue:          newQueueStore(t),
		Runtime:        rt,
		Snapshot:       &fakeSnapshot{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: filepath.Join(t.TempDir(), "state-store"),
		Version:        "v1",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}
