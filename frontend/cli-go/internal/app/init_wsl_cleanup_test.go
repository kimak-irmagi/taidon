package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveSystemdMountUnitRunsCommands(t *testing.T) {
	var descs []string
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		descs = append(descs, desc)
		return "", nil
	})

	removeSystemdMountUnit(context.Background(), "Ubuntu", "sqlrs-state-store.mount", false)

	want := []string{
		"stop mount unit (root)",
		"disable mount unit (root)",
		"remove mount unit (root)",
		"reload systemd (root)",
	}
	if len(descs) != len(want) {
		t.Fatalf("expected %d calls, got %d (%v)", len(want), len(descs), descs)
	}
	for i, expected := range want {
		if descs[i] != expected {
			t.Fatalf("expected call %d to be %q, got %q", i, expected, descs[i])
		}
	}
}

func TestReinitWSLStoreUnmountsAndRemovesVHDX(t *testing.T) {
	vhdxPath := filepath.Join(t.TempDir(), "store.vhdx")
	if err := os.WriteFile(vhdxPath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write vhdx: %v", err)
	}

	var wslDescs []string
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		wslDescs = append(wslDescs, desc)
		if strings.Contains(desc, "findmnt") {
			return "btrfs\n", nil
		}
		return "", nil
	})

	var hostDescs []string
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		hostDescs = append(hostDescs, desc)
		return "", nil
	})

	if err := reinitWSLStore(context.Background(), "Ubuntu", "/var/lib/sqlrs", vhdxPath, "sqlrs-state-store.mount", false); err != nil {
		t.Fatalf("reinitWSLStore: %v", err)
	}

	if _, err := os.Stat(vhdxPath); !os.IsNotExist(err) {
		t.Fatalf("expected vhdx removed, got err=%v", err)
	}

	if !containsString(hostDescs, "unmount VHDX from WSL") {
		t.Fatalf("expected host unmount from WSL, got %v", hostDescs)
	}
	if !containsString(hostDescs, "unmount VHDX on host") {
		t.Fatalf("expected host unmount, got %v", hostDescs)
	}
	if !containsString(wslDescs, "unmount btrfs (root)") {
		t.Fatalf("expected WSL unmount, got %v", wslDescs)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
