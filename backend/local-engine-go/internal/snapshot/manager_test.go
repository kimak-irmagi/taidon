package snapshot

import (
	"context"
	"testing"
)

func TestNewManagerPreferOverlayFalseReturnsCopyManager(t *testing.T) {
	mgr := NewManager(Options{PreferOverlay: false})
	if mgr.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", mgr.Kind())
	}
}

func TestNewManagerPreferOverlayUsesOverlayWhenAvailable(t *testing.T) {
	prevSupported := overlaySupportedFn
	prevNew := newOverlayManagerFn
	defer func() {
		overlaySupportedFn = prevSupported
		newOverlayManagerFn = prevNew
	}()

	overlaySupportedFn = func() bool { return true }
	newOverlayManagerFn = func() Manager { return fakeManager{kind: "overlay"} }

	mgr := NewManager(Options{PreferOverlay: true})
	if mgr.Kind() != "overlay" {
		t.Fatalf("expected overlay manager, got %s", mgr.Kind())
	}
}

type fakeManager struct {
	kind string
}

func (f fakeManager) Kind() string {
	return f.kind
}

func (f fakeManager) Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error) {
	return CloneResult{}, nil
}

func (f fakeManager) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return nil
}

func (f fakeManager) Destroy(ctx context.Context, dir string) error {
	return nil
}
