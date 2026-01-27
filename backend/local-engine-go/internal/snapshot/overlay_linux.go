//go:build linux

package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type overlayManager struct {
	runner commandRunner
}

type commandRunner interface {
	Run(ctx context.Context, name string, args []string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("%w: %s", err, trimmed)
		}
		return err
	}
	return nil
}

func newOverlayManager() Manager {
	return overlayManager{runner: execRunner{}}
}

func overlaySupported() bool {
	if _, err := exec.LookPath("mount"); err != nil {
		return false
	}
	temp, err := os.MkdirTemp("", "sqlrs-overlay-probe-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(temp)
	lower := filepath.Join(temp, "lower")
	upper := filepath.Join(temp, "upper")
	work := filepath.Join(temp, "work")
	merged := filepath.Join(temp, "merged")
	if err := os.MkdirAll(lower, 0o700); err != nil {
		return false
	}
	if err := os.MkdirAll(upper, 0o700); err != nil {
		return false
	}
	if err := os.MkdirAll(work, 0o700); err != nil {
		return false
	}
	if err := os.MkdirAll(merged, 0o700); err != nil {
		return false
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, merged).Run(); err != nil {
		return false
	}
	_ = exec.Command("umount", merged).Run()
	return true
}

func (m overlayManager) Kind() string {
	return "overlayfs"
}

func (m overlayManager) Capabilities() Capabilities {
	return Capabilities{
		RequiresDBStop:       true,
		SupportsWritableClone: true,
		SupportsSendReceive:   false,
	}
}

func (m overlayManager) Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error) {
	if srcDir == "" || destDir == "" {
		return CloneResult{}, fmt.Errorf("source and destination are required")
	}
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)
	upper := filepath.Join(destDir, "upper")
	work := filepath.Join(destDir, "work")
	merged := filepath.Join(destDir, "merged")
	if err := os.MkdirAll(upper, 0o700); err != nil {
		return CloneResult{}, err
	}
	if err := os.MkdirAll(work, 0o700); err != nil {
		return CloneResult{}, err
	}
	if err := os.MkdirAll(merged, 0o700); err != nil {
		return CloneResult{}, err
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", srcDir, upper, work)
	if err := m.runner.Run(ctx, "mount", []string{"-t", "overlay", "overlay", "-o", opts, merged}); err != nil {
		return CloneResult{}, err
	}
	cleanup := func() error {
		_ = m.runner.Run(context.Background(), "umount", []string{merged})
		return os.RemoveAll(destDir)
	}
	return CloneResult{MountDir: merged, Cleanup: cleanup}, nil
}

func (m overlayManager) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return copyDir(ctx, srcDir, destDir)
}

func (m overlayManager) Destroy(ctx context.Context, dir string) error {
	merged := filepath.Join(dir, "merged")
	_ = m.runner.Run(ctx, "umount", []string{merged})
	return os.RemoveAll(dir)
}
