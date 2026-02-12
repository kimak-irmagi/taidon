package app

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
	"sqlrs/cli/internal/wsl"
)

func TestRunGetwdError(t *testing.T) {
	prev := getwdFn
	getwdFn = func() (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { getwdFn = prev })

	if err := Run([]string{"status"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected getwd error, got %v", err)
	}
}

func TestRunMissingCommand(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, nil, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"status"}); err == nil || !strings.Contains(err.Error(), "missing command") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestRunInitCombined(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"init", "status"}); err == nil || !strings.Contains(err.Error(), "unknown init mode") {
		t.Fatalf("expected init mode error, got %v", err)
	}
}

func TestRunWorkspaceRootResolution(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".sqlrs", "config.yaml"), []byte(""), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	setTestDirs(t, root)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	if err := Run([]string{"--mode=remote", "--endpoint", server.URL, "status"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunResolveWSLSettingsError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path")
	}
	temp := t.TempDir()
	setTestDirs(t, temp)

	dirs, err := paths.Resolve()
	if err != nil {
		t.Fatalf("resolve dirs: %v", err)
	}
	if err := os.MkdirAll(dirs.ConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	configData := "engine:\n  wsl:\n    mode: required\n    stateDir: /mnt/sqlrs/store\n"
	if err := os.WriteFile(filepath.Join(dirs.ConfigDir, "config.yaml"), []byte(configData), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return nil, errors.New("wsl missing")
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	if err := Run([]string{"status"}); err == nil || !strings.Contains(err.Error(), "WSL unavailable") {
		t.Fatalf("expected WSL unavailable error, got %v", err)
	}
}

func TestRunPrepareHandled(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"prepare:psql", "--help"}); err != nil {
		t.Fatalf("expected help handled, got %v", err)
	}
}

func TestRunPrepareLiquibaseHandled(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"prepare:lb", "--help"}); err != nil {
		t.Fatalf("expected help handled, got %v", err)
	}
}

func TestRunPrepareError(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"prepare:psql", "--", "-c", "select 1"}); err == nil || !strings.Contains(err.Error(), "Missing base image") {
		t.Fatalf("expected prepare error, got %v", err)
	}
}

func TestRunPrepareLiquibaseError(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"prepare:lb", "--", "update"}); err == nil || !strings.Contains(err.Error(), "Missing base image") {
		t.Fatalf("expected prepare lb error, got %v", err)
	}
}

func TestRunLsCannotCombine(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "ls"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"ls"}); err == nil || !strings.Contains(err.Error(), "ls cannot be combined") {
		t.Fatalf("expected ls combine error, got %v", err)
	}
}

func TestRunRmCannotCombine(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "rm"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"rm"}); err == nil || !strings.Contains(err.Error(), "rm cannot be combined") {
		t.Fatalf("expected rm combine error, got %v", err)
	}
}

func TestRunPlanCannotCombine(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "plan:psql"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"plan:psql"}); err == nil || !strings.Contains(err.Error(), "plan cannot be combined") {
		t.Fatalf("expected plan combine error, got %v", err)
	}
}

func TestRunStatusCannotCombine(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "status"}, {Name: "ls"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"status"}); err == nil || !strings.Contains(err.Error(), "status cannot be combined") {
		t.Fatalf("expected status combine error, got %v", err)
	}
}

func TestRunConfigCannotCombine(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "config"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run([]string{"config"}); err == nil || !strings.Contains(err.Error(), "config cannot be combined") {
		t.Fatalf("expected config combine error, got %v", err)
	}
}

func TestRunWriteJSONError(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
	})

	err = Run([]string{"--mode=remote", "--endpoint", server.URL, "--output=json", "status"})
	if err == nil {
		t.Fatalf("expected writeJSON error")
	}
}

func TestStartCleanupSpinnerTerminal(t *testing.T) {
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no console output: %v", err)
	}
	defer out.Close()

	oldStdout := os.Stdout
	os.Stdout = out
	t.Cleanup(func() { os.Stdout = oldStdout })

	stop := startCleanupSpinner("inst", false)
	time.Sleep(600 * time.Millisecond)
	stop()
}

func TestClearLineOutCoverage(t *testing.T) {
	clearLineOut(nil, 0)
	var buf bytes.Buffer
	clearLineOut(&buf, 0)
	if !strings.Contains(buf.String(), "\r") {
		t.Fatalf("expected carriage returns, got %q", buf.String())
	}
}

func TestResolveWSLSettingsRequiredMissing(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	_, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err == nil {
		t.Fatalf("expected missing distro/stateDir error")
	}
}

func TestResolveWSLSettingsMissingMountUnit(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.Distro = "Ubuntu"
	cfg.Engine.WSL.StateDir = "/mnt/sqlrs/store"
	_, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err == nil {
		t.Fatalf("expected missing mount unit error")
	}
}

func TestResolveWSLSettingsInvalidMode(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "bogus"
	daemon, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if daemon != "daemon" {
		t.Fatalf("unexpected daemon path: %q", daemon)
	}
}

func TestResolveWSLSettingsEmptyMode(t *testing.T) {
	cfg := config.Config{}
	daemon, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if daemon != "daemon" {
		t.Fatalf("unexpected daemon path: %q", daemon)
	}
}

func TestResolveWSLSettingsListDistroFailure(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.StateDir = "/mnt/sqlrs/store"
	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })
	_, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err == nil || !strings.Contains(err.Error(), "WSL unavailable") {
		t.Fatalf("expected list distro error, got %v", err)
	}
}

func TestResolveWSLSettingsSelectDistroFailure(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.StateDir = "/mnt/sqlrs/store"
	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })
	_, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{}, "daemon")
	if err == nil || !strings.Contains(err.Error(), "distro resolution") {
		t.Fatalf("expected distro resolution error, got %v", err)
	}
}

func TestResolveWSLSettingsWindowsPathError(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.Distro = "Ubuntu"
	cfg.Engine.WSL.StateDir = "/mnt/sqlrs/store"
	cfg.Engine.WSL.Mount.Unit = "sqlrs-store.mount"
	cfg.Engine.WSL.EnginePath = "relative.exe"
	_, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{StateDir: "C:\\state"}, "daemon")
	if err == nil {
		t.Fatalf("expected windows path error")
	}
}

func TestWindowsToWSLPathCoverage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path conversion")
	}
	if _, err := windowsToWSLPath(" "); err == nil {
		t.Fatalf("expected empty path error")
	}
	if out, err := windowsToWSLPath("/mnt/c/temp"); err != nil || out != "/mnt/c/temp" {
		t.Fatalf("expected passthrough path, got %q err=%v", out, err)
	}
	if _, err := windowsToWSLPath("relative"); err == nil {
		t.Fatalf("expected absolute path error")
	}
	out, err := windowsToWSLPath("C:\\temp\\file.sql")
	if err != nil || out != "/mnt/c/temp/file.sql" {
		t.Fatalf("unexpected windows conversion: %q err=%v", out, err)
	}
	out, err = windowsToWSLPath("D:\\")
	if err != nil || out != "/mnt/d" {
		t.Fatalf("unexpected root conversion: %q err=%v", out, err)
	}
}

func TestRunMissingRunKind(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	if err := Run([]string{"run:"}); err == nil || !strings.Contains(err.Error(), "missing run kind") {
		t.Fatalf("expected missing run kind error, got %v", err)
	}
}

func TestCompositeRunCleanupErrorVerbose(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	server := compositeRunServer(t, http.StatusInternalServerError, `{"message":"boom"}`)
	t.Cleanup(server.Close)

	err := Run([]string{
		"--mode=remote",
		"--endpoint", server.URL,
		"--verbose",
		"prepare:psql", "--image", "image", "--", "-c", "select 1",
		"run:psql", "--", "-c", "select 1",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestCompositeRunCleanupBlockedNonVerbose(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	server := compositeRunServer(t, http.StatusConflict, `{"dry_run":false,"outcome":"blocked","root":{"kind":"instance","id":"inst","blocked":"busy","connections":0}}`)
	t.Cleanup(server.Close)

	err := Run([]string{
		"--mode=remote",
		"--endpoint", server.URL,
		"prepare:psql", "--image", "image", "--", "-c", "select 1",
		"run:pgbench",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func compositeRunServer(t *testing.T, deleteStatus int, deleteBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"exit","exit_code":0}`+"\n")
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(deleteStatus)
			io.WriteString(w, deleteBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
