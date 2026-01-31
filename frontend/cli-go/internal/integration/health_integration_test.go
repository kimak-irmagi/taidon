package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"sqlrs/cli/internal/app"
	"sqlrs/cli/internal/daemon"
	"sqlrs/cli/internal/paths"
)

func TestCLIStatusWithLocalEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("SQLRS_RUN_DOCKER_TESTS") != "1" {
		t.Skip("docker tests disabled (set SQLRS_RUN_DOCKER_TESTS=1 to enable)")
	}

	engineRoot := findEngineModuleRoot(t)
	tempDir := t.TempDir()

	engineBin := filepath.Join(tempDir, "sqlrs-engine")
	if runtime.GOOS == "windows" {
		engineBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", engineBin, "./cmd/sqlrs-engine")
	build.Dir = engineRoot
	build.Env = os.Environ()
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sqlrs-engine: %v\n%s", err, output)
	}

	t.Setenv("SQLRS_DAEMON_PATH", engineBin)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tempDir)
		t.Setenv("LOCALAPPDATA", tempDir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		t.Setenv("XDG_STATE_HOME", tempDir)
		t.Setenv("XDG_CACHE_HOME", tempDir)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}

	if err := app.Run([]string{"status"}); err != nil {
		t.Fatalf("sqlrs status failed: %v", err)
	}

	dirs, err := paths.Resolve()
	if err != nil {
		t.Fatalf("resolve dirs: %v", err)
	}

	statePath := filepath.Join(dirs.StateDir, "engine.json")
	if state, err := daemon.ReadEngineState(statePath); err == nil && state.PID > 0 {
		if proc, err := os.FindProcess(state.PID); err == nil {
			_ = proc.Kill()
		}
	}
}

func findEngineModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "backend", "local-engine-go", "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Dir(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("unable to locate backend/local-engine-go")
		}
		dir = parent
	}
}
