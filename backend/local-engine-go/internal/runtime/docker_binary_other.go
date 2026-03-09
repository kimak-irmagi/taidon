//go:build !linux

package runtime

func resolveDockerBinary(binary string) string {
	if binary == "" {
		return defaultDockerBinary
	}
	return binary
}
