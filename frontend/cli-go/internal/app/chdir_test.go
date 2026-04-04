package app

import (
	"os"
	"path/filepath"
)

var stableTestWorkingDir = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}()

var isolatedTestWorkingDirRoot = func() string {
	dir, err := os.MkdirTemp("", "sqlrs-cli-app-cwd-*")
	if err != nil {
		panic(err)
	}
	return dir
}()

func restoreWorkingDirForTest(target string) {
	if err := os.Chdir(target); err == nil {
		return
	}
	_ = os.Chdir(stableTestWorkingDir)
}

func newIsolatedTestWorkingDir() (string, error) {
	return os.MkdirTemp(isolatedTestWorkingDirRoot, "cwd-*")
}

func cleanupIsolatedTestWorkingDir(path string) {
	cleaned := filepath.Clean(path)
	if cleaned == "" || cleaned == "." {
		return
	}
	_ = os.RemoveAll(cleaned)
}
