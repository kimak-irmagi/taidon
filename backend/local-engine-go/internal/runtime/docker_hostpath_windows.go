//go:build windows

package runtime

import (
	"os"
	"strings"
)

func useLinuxMountPathStyle() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(dockerHostPathStyleEnv)), dockerHostPathLinux)
}
