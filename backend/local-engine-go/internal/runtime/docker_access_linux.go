//go:build linux

package runtime

import (
	"context"
	"os"
)

func (r *DockerRuntime) ensureHostReadableDataDir(ctx context.Context, imageID string, dataDir string) error {
	args := []string{
		"run", "--rm",
		"-v", dockerBindSpec(dataDir, PostgresDataDirRoot, false),
		imageID,
		"chmod", "-R", "a+rX", PostgresDataDir,
	}
	if err := r.runPermissionCommand(ctx, args); err != nil {
		return err
	}
	return nil
}

func ensureHostDataDirAccess(dataDir string) {
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		return
	}
	// Best effort: allow container postgres user to traverse mounted base dir.
	_ = os.Chmod(dataDir, 0o755)
}
