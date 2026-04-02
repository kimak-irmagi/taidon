package app

import (
	"os"
)

var stableTestWorkingDir = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}()

func restoreWorkingDirForTest(target string) {
	if err := os.Chdir(target); err == nil {
		return
	}
	_ = os.Chdir(stableTestWorkingDir)
}
