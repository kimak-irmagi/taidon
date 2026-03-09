//go:build !windows

package runtime

func useLinuxMountPathStyle() bool {
	return false
}
