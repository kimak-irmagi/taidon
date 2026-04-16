package app

import (
	"github.com/sqlrs/cli/internal/pathutil"
)

func isWithin(base, target string) bool {
	return pathutil.IsWithin(base, target)
}
