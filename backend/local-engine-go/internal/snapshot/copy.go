package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type CopyManager struct{}

func (CopyManager) Kind() string {
	return "copy"
}

func (CopyManager) Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error) {
	if err := copyDir(ctx, srcDir, destDir); err != nil {
		return CloneResult{}, err
	}
	return CloneResult{
		MountDir: destDir,
		Cleanup: func() error {
			return os.RemoveAll(destDir)
		},
	}, nil
}

func (CopyManager) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return copyDir(ctx, srcDir, destDir)
}

func (CopyManager) Destroy(ctx context.Context, dir string) error {
	return os.RemoveAll(dir)
}

func copyDir(ctx context.Context, srcDir string, destDir string) error {
	srcDir = filepath.Clean(srcDir)
	destDir = filepath.Clean(destDir)
	if srcDir == "" || destDir == "" {
		return fmt.Errorf("source and destination are required")
	}
	srcAbs, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(srcAbs, destAbs)
	if err == nil {
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
			return fmt.Errorf("destination must not be inside source: %s", destDir)
		}
	}
	info, err := os.Stat(srcDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcDir)
	}
	if err := os.MkdirAll(destDir, info.Mode()); err != nil {
		return err
	}
	return filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, rel)
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src string, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
