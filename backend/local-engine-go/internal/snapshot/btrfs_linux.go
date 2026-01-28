//go:build linux

package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const btrfsSuperMagic int64 = 0x9123683E

var statfsFn = func(path string, stat *syscall.Statfs_t) error {
	return syscall.Statfs(path, stat)
}

var osMkdirAllBtrfs = os.MkdirAll
var execLookPathBtrfs = exec.LookPath

type btrfsManager struct {
	runner commandRunner
}

func newBtrfsManager() Manager {
	return btrfsManager{runner: execRunner{}}
}

func btrfsSupported(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := execLookPathBtrfs("btrfs"); err != nil {
		return false
	}
	var stat syscall.Statfs_t
	if err := statfsFn(path, &stat); err != nil {
		return false
	}
	return stat.Type == btrfsSuperMagic
}

func (btrfsManager) Kind() string {
	return "btrfs"
}

func (btrfsManager) Capabilities() Capabilities {
	return Capabilities{
		RequiresDBStop:       true,
		SupportsWritableClone: true,
		SupportsSendReceive:   false,
	}
}

func (m btrfsManager) Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error) {
	if strings.TrimSpace(srcDir) == "" || strings.TrimSpace(destDir) == "" {
		return CloneResult{}, fmt.Errorf("source and destination are required")
	}
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)
	if err := osMkdirAllBtrfs(filepath.Dir(destDir), 0o700); err != nil {
		return CloneResult{}, err
	}
	if err := m.runner.Run(ctx, "btrfs", []string{"subvolume", "snapshot", srcDir, destDir}); err != nil {
		return CloneResult{}, err
	}
	return CloneResult{
		MountDir: destDir,
		Cleanup: func() error {
			return m.runner.Run(context.Background(), "btrfs", []string{"subvolume", "delete", destDir})
		},
	}, nil
}

func (m btrfsManager) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	if strings.TrimSpace(srcDir) == "" || strings.TrimSpace(destDir) == "" {
		return fmt.Errorf("source and destination are required")
	}
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)
	if err := osMkdirAllBtrfs(filepath.Dir(destDir), 0o700); err != nil {
		return err
	}
	return m.runner.Run(ctx, "btrfs", []string{"subvolume", "snapshot", "-r", srcDir, destDir})
}

func (m btrfsManager) Destroy(ctx context.Context, dir string) error {
	return m.runner.Run(ctx, "btrfs", []string{"subvolume", "delete", dir})
}
