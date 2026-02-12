package app

import (
	"path/filepath"
	"runtime"
	"testing"

	"sqlrs/cli/internal/cli"
)

func TestResolveStoreTypeExplicit(t *testing.T) {
	out, err := resolveStoreType("auto", "DIR")
	if err != nil {
		t.Fatalf("resolveStoreType: %v", err)
	}
	if out != "dir" {
		t.Fatalf("expected dir, got %q", out)
	}
}

func TestResolveStoreTypeAuto(t *testing.T) {
	out, err := resolveStoreType("auto", "")
	if err != nil {
		t.Fatalf("resolveStoreType: %v", err)
	}
	if runtime.GOOS == "windows" {
		if out != "image" {
			t.Fatalf("expected image on windows, got %q", out)
		}
	} else if out != "dir" {
		t.Fatalf("expected dir on non-windows, got %q", out)
	}
}

func TestResolveStorePathUsesEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", root)

	out, err := resolveStorePath("dir", "")
	if err != nil {
		t.Fatalf("resolveStorePath(dir): %v", err)
	}
	if out != root {
		t.Fatalf("expected root %q, got %q", root, out)
	}

	out, err = resolveStorePath("image", "")
	if err != nil {
		t.Fatalf("resolveStorePath(image): %v", err)
	}
	if runtime.GOOS == "windows" {
		if out != filepath.Join(root, "btrfs.vhdx") {
			t.Fatalf("expected windows vhdx path, got %q", out)
		}
	} else if out != filepath.Join(root, "btrfs.img") {
		t.Fatalf("expected img path, got %q", out)
	}
}

func TestShouldUseWSLNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior")
	}
	use, require := shouldUseWSL("auto", "image", true)
	if use || require {
		t.Fatalf("expected no WSL usage on non-windows, got use=%v require=%v", use, require)
	}
}

func TestBuildPathConverterNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior")
	}
	if conv := buildPathConverter(cli.PrepareOptions{WSLDistro: "Ubuntu"}); conv != nil {
		t.Fatalf("expected nil converter on non-windows")
	}
}

func TestBuildPathConverterWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only behavior")
	}
	if conv := buildPathConverter(cli.PrepareOptions{WSLDistro: "Ubuntu"}); conv == nil {
		t.Fatalf("expected converter on windows with WSL distro")
	}
}
