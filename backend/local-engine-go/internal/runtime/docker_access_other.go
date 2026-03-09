//go:build !linux

package runtime

import "context"

func (r *DockerRuntime) ensureHostReadableDataDir(ctx context.Context, imageID string, dataDir string) error {
	return nil
}

func ensureHostDataDirAccess(dataDir string) {}
