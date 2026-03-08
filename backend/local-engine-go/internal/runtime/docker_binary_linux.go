//go:build linux

package runtime

import (
	"strings"
)

func resolveDockerBinary(binary string) string {
	if binary == "" {
		binary = defaultDockerBinary
	}
	if binary != defaultDockerBinary {
		return binary
	}
	if path, err := execLookPath(binary); err == nil {
		if strings.HasSuffix(strings.ToLower(path), ".exe") {
			if _, err := osStat("/usr/bin/docker"); err == nil {
				return "/usr/bin/docker"
			}
		}
	}
	return binary
}
