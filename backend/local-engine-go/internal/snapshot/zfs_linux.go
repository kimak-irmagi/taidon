//go:build linux

package snapshot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const zfsSuperMagic int64 = 0x2FC12FC1

var statfsZfsFn = func(path string, stat *syscall.Statfs_t) error {
	return syscall.Statfs(path, stat)
}

var osMkdirAllZfs = os.MkdirAll
var execLookPathZfs = exec.LookPath
var osStatZfs = os.Stat

var zfsListAllFn = func() (string, error) {
	cmd := exec.Command("zfs", "list", "-H", "-o", "name,mountpoint")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var zfsDatasetForPathFn = zfsDatasetForPath

var zfsListDatasetsFn = func(ctx context.Context, dataset string) (string, error) {
	cmd := exec.CommandContext(ctx, "zfs", "list", "-H", "-r", "-o", "name", dataset)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var zfsGetOriginFn = func(ctx context.Context, dataset string) (string, error) {
	cmd := exec.CommandContext(ctx, "zfs", "get", "-H", "-o", "value", "origin", dataset)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// zfsNewSnapSuffix returns a unique suffix for snapshot names.
// Injected as a variable to allow deterministic values in tests.
var zfsNewSnapSuffix = func() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// zfsDatasetForPath resolves a filesystem path to its ZFS dataset name.
// For paths that are exact dataset mountpoints it returns the dataset name directly.
// For paths inside a dataset it appends the relative path to the closest ancestor dataset.
func zfsDatasetForPath(path string) (string, error) {
	out, err := zfsListAllFn()
	if err != nil {
		return "", fmt.Errorf("zfs list: %w", err)
	}

	bestDataset := ""
	bestMountLen := -1

	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		name, mount := parts[0], parts[1]
		if mount == "none" || mount == "-" || mount == "" {
			continue
		}
		if mount == path {
			return name, nil
		}
		prefix := mount + "/"
		if strings.HasPrefix(path, prefix) && len(mount) > bestMountLen {
			rel := path[len(prefix):]
			bestDataset = filepath.Join(name, rel)
			bestMountLen = len(mount)
		}
	}

	if bestDataset == "" {
		return "", fmt.Errorf("no ZFS dataset found for path: %s", path)
	}
	return bestDataset, nil
}

func zfsSupported(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := execLookPathZfs("zfs"); err != nil {
		return false
	}
	var stat syscall.Statfs_t
	if err := statfsZfsFn(path, &stat); err != nil {
		return false
	}
	return stat.Type == zfsSuperMagic
}

type zfsManager struct {
	runner commandRunner
}

func newZfsManager() Manager {
	return zfsManager{runner: execRunner{}}
}

func (zfsManager) Kind() string {
	return "zfs"
}

func (zfsManager) Capabilities() Capabilities {
	return Capabilities{
		RequiresDBStop:        true,
		SupportsWritableClone: true,
		SupportsSendReceive:   false,
	}
}

func (m zfsManager) Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error) {
	if strings.TrimSpace(srcDir) == "" || strings.TrimSpace(destDir) == "" {
		return CloneResult{}, fmt.Errorf("source and destination are required")
	}
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)

	log.Printf("zfs: clone start src=%s dest=%s", srcDir, destDir)

	srcDataset, err := zfsDatasetForPathFn(srcDir)
	if err != nil {
		return CloneResult{}, fmt.Errorf("zfs: resolve src dataset: %w", err)
	}

	destParent, err := zfsDatasetForPathFn(filepath.Dir(destDir))
	if err != nil {
		return CloneResult{}, fmt.Errorf("zfs: resolve dest parent dataset: %w", err)
	}
	destDataset := filepath.Join(destParent, filepath.Base(destDir))

	snapName := "taidon-clone-" + zfsNewSnapSuffix()
	fullSnap := srcDataset + "@" + snapName

	log.Printf("zfs: clone exec %s", formatCommand("zfs", []string{"snapshot", fullSnap}))
	if err := m.runner.Run(ctx, "zfs", []string{"snapshot", fullSnap}); err != nil {
		return CloneResult{}, fmt.Errorf("zfs: snapshot %s: %w", fullSnap, err)
	}

	cloneArgs := []string{"clone", "-o", "mountpoint=" + destDir, fullSnap, destDataset}
	log.Printf("zfs: clone exec %s", formatCommand("zfs", cloneArgs))
	if err := m.runner.Run(ctx, "zfs", cloneArgs); err != nil {
		_ = m.runner.Run(context.Background(), "zfs", []string{"destroy", fullSnap})
		return CloneResult{}, fmt.Errorf("zfs: clone %s: %w", destDataset, err)
	}

	log.Printf("zfs: clone complete src=%s dest=%s", srcDir, destDir)
	return CloneResult{
		MountDir: destDir,
		Cleanup: func() error {
			log.Printf("zfs: clone cleanup start dest=%s", destDir)
			log.Printf("zfs: clone cleanup exec %s", formatCommand("zfs", []string{"destroy", destDataset}))
			if err := m.runner.Run(context.Background(), "zfs", []string{"destroy", destDataset}); err != nil {
				return err
			}
			log.Printf("zfs: clone cleanup exec %s", formatCommand("zfs", []string{"destroy", fullSnap}))
			return m.runner.Run(context.Background(), "zfs", []string{"destroy", fullSnap})
		},
	}, nil
}

func (m zfsManager) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	if strings.TrimSpace(srcDir) == "" || strings.TrimSpace(destDir) == "" {
		return fmt.Errorf("source and destination are required")
	}
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)

	log.Printf("zfs: snapshot start src=%s dest=%s", srcDir, destDir)

	if err := osMkdirAllZfs(filepath.Dir(destDir), 0o700); err != nil {
		log.Printf("zfs: snapshot mkdir failed dest=%s err=%v", destDir, err)
		return err
	}

	srcDataset, err := zfsDatasetForPathFn(srcDir)
	if err != nil {
		return fmt.Errorf("zfs: resolve src dataset: %w", err)
	}

	destParent, err := zfsDatasetForPathFn(filepath.Dir(destDir))
	if err != nil {
		return fmt.Errorf("zfs: resolve dest parent dataset: %w", err)
	}
	destDataset := filepath.Join(destParent, filepath.Base(destDir))

	snapName := "taidon-snap-" + zfsNewSnapSuffix()
	fullSnap := srcDataset + "@" + snapName

	log.Printf("zfs: snapshot exec %s", formatCommand("zfs", []string{"snapshot", fullSnap}))
	if err := m.runner.Run(ctx, "zfs", []string{"snapshot", fullSnap}); err != nil {
		return fmt.Errorf("zfs: snapshot %s: %w", fullSnap, err)
	}

	cloneArgs := []string{"clone", "-o", "mountpoint=" + destDir, "-o", "readonly=on", fullSnap, destDataset}
	log.Printf("zfs: snapshot exec %s", formatCommand("zfs", cloneArgs))
	if err := m.runner.Run(ctx, "zfs", cloneArgs); err != nil {
		_ = m.runner.Run(context.Background(), "zfs", []string{"destroy", fullSnap})
		return fmt.Errorf("zfs: clone for snapshot %s: %w", destDataset, err)
	}

	log.Printf("zfs: snapshot complete src=%s dest=%s", srcDir, destDir)
	return nil
}

func (m zfsManager) Destroy(ctx context.Context, dir string) error {
	dir = filepath.Clean(dir)
	dataset, err := zfsDatasetForPathFn(dir)
	if err != nil {
		log.Printf("zfs: destroy could not resolve dataset for %s: %v", dir, err)
		return err
	}

	origin, _ := zfsGetOriginFn(ctx, dataset)

	log.Printf("zfs: destroy exec %s", formatCommand("zfs", []string{"destroy", dataset}))
	if destroyErr := m.runner.Run(ctx, "zfs", []string{"destroy", dataset}); destroyErr != nil {
		output, listErr := zfsListDatasetsFn(ctx, dataset)
		if listErr == nil {
			trimmed := strings.TrimSpace(output)
			if trimmed != "" {
				return fmt.Errorf("%w: nested datasets:\n%s", destroyErr, trimmed)
			}
		}
		return destroyErr
	}

	if origin != "" && origin != "-" {
		log.Printf("zfs: destroy origin exec %s", formatCommand("zfs", []string{"destroy", origin}))
		_ = m.runner.Run(context.Background(), "zfs", []string{"destroy", origin})
	}

	return nil
}

func (m zfsManager) EnsureDataset(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	path = filepath.Clean(path)

	if err := osMkdirAllZfs(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	if _, err := osStatZfs(path); err == nil {
		checkArgs := []string{"list", "-H", "-o", "name", path}
		log.Printf("zfs: ensure dataset exec %s", formatCommand("zfs", checkArgs))
		if m.runner.Run(ctx, "zfs", checkArgs) == nil {
			return nil
		}
		return fmt.Errorf("path exists but is not a ZFS dataset: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	parentDataset, err := zfsDatasetForPathFn(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("zfs: resolve parent dataset: %w", err)
	}
	dataset := filepath.Join(parentDataset, filepath.Base(path))

	createArgs := []string{"create", "-o", "mountpoint=" + path, dataset}
	log.Printf("zfs: ensure dataset exec %s", formatCommand("zfs", createArgs))
	return m.runner.Run(ctx, "zfs", createArgs)
}

func (m zfsManager) IsDataset(ctx context.Context, path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("path is required")
	}
	path = filepath.Clean(path)

	if _, err := osStatZfs(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	dataset, err := zfsDatasetForPathFn(path)
	if err != nil {
		return false, nil
	}

	checkArgs := []string{"list", "-H", "-o", "name", dataset}
	log.Printf("zfs: is dataset exec %s", formatCommand("zfs", checkArgs))
	if err := m.runner.Run(ctx, "zfs", checkArgs); err != nil {
		return false, nil
	}
	return true, nil
}
