//go:build linux

package app

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
)

func TestPlanLocalBtrfsStoreDir(t *testing.T) {
	plan, err := planLocalBtrfsStore("dir", "/var/lib/sqlrs/store")
	if err != nil {
		t.Fatalf("planLocalBtrfsStore: %v", err)
	}
	if plan.storeDir != "/var/lib/sqlrs/store" {
		t.Fatalf("expected storeDir /var/lib/sqlrs/store, got %q", plan.storeDir)
	}
	if plan.imagePath != "/var/lib/sqlrs/store.btrfs.img" {
		t.Fatalf("expected sibling image path, got %q", plan.imagePath)
	}
	if plan.devicePath != "" {
		t.Fatalf("expected empty devicePath, got %q", plan.devicePath)
	}
}

func TestPlanLocalBtrfsStoreImagePath(t *testing.T) {
	plan, err := planLocalBtrfsStore("image", "/var/lib/sqlrs/custom.img")
	if err != nil {
		t.Fatalf("planLocalBtrfsStore: %v", err)
	}
	if plan.storeDir != "/var/lib/sqlrs" {
		t.Fatalf("expected storeDir /var/lib/sqlrs, got %q", plan.storeDir)
	}
	if plan.imagePath != "/var/lib/sqlrs/custom.img" {
		t.Fatalf("expected imagePath /var/lib/sqlrs/custom.img, got %q", plan.imagePath)
	}
	if plan.devicePath != "" {
		t.Fatalf("expected empty devicePath, got %q", plan.devicePath)
	}
}

func TestPlanLocalBtrfsStoreDevicePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", root)

	plan, err := planLocalBtrfsStore("device", "/dev/loop7")
	if err != nil {
		t.Fatalf("planLocalBtrfsStore: %v", err)
	}
	if plan.devicePath != "/dev/loop7" {
		t.Fatalf("expected devicePath /dev/loop7, got %q", plan.devicePath)
	}
	if plan.storeDir != root {
		t.Fatalf("expected storeDir %q, got %q", root, plan.storeDir)
	}
	if plan.imagePath != "" {
		t.Fatalf("expected empty imagePath, got %q", plan.imagePath)
	}
}

func TestRunInitUsesLocalBtrfsStoreResult(t *testing.T) {
	workspace := t.TempDir()
	imagePath := filepath.Join(workspace, "custom.img")
	expectedStore := filepath.Join(workspace, "mounted-store")

	var captured localBtrfsInitOptions
	withInitLocalBtrfsStub(t, func(opts localBtrfsInitOptions) (localBtrfsInitResult, error) {
		captured = opts
		return localBtrfsInitResult{StorePath: expectedStore}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{
		"local",
		"--snapshot", "btrfs",
		"--store", "image", imagePath,
	}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	if captured.StoreType != "image" {
		t.Fatalf("expected captured store type image, got %q", captured.StoreType)
	}
	if captured.StorePath != imagePath {
		t.Fatalf("expected captured store path %q, got %q", imagePath, captured.StorePath)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "engine", "storePath"); got != expectedStore {
		t.Fatalf("expected engine.storePath %q, got %q", expectedStore, got)
	}
}

func TestRunInitReturnsExitErrorWhenLinuxBtrfsInitFails(t *testing.T) {
	workspace := t.TempDir()
	withInitLocalBtrfsStub(t, func(opts localBtrfsInitOptions) (localBtrfsInitResult, error) {
		return localBtrfsInitResult{}, errors.New("btrfs setup failed")
	})

	var out bytes.Buffer
	err := runInit(&out, workspace, "", []string{
		"local",
		"--snapshot", "btrfs",
		"--store", "image", filepath.Join(workspace, "custom.img"),
	}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}
