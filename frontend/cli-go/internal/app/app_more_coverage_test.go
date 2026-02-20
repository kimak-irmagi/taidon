package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
)

func TestRunInitCannotCombineCoverage(t *testing.T) {
	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "init"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run(nil); err == nil || !strings.Contains(err.Error(), "init cannot be combined") {
		t.Fatalf("expected init combine error, got %v", err)
	}
}

func TestRunCompositePrepareBranchesCoverage(t *testing.T) {
	t.Run("psql handled", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}, []cli.Command{
				{Name: "prepare:psql", Args: []string{"--help"}},
				{Name: "run:psql", Args: []string{}},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("psql error", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}, []cli.Command{
				{Name: "prepare:psql", Args: []string{}},
				{Name: "run:psql", Args: []string{}},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "Missing base image id") {
			t.Fatalf("expected prepare error, got %v", err)
		}
	})

	t.Run("lb handled", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}, []cli.Command{
				{Name: "prepare:lb", Args: []string{"--help"}},
				{Name: "run:psql", Args: []string{}},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("lb error", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}, []cli.Command{
				{Name: "prepare:lb", Args: []string{"--image", "img"}},
				{Name: "run:psql", Args: []string{}},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "liquibase command is required") {
			t.Fatalf("expected prepare lb error, got %v", err)
		}
	})

	t.Run("lb success", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"img","prepare_kind":"lb","prepare_args_normalized":"update"}}`)
			case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
				w.Header().Set("Content-Type", "application/x-ndjson")
				io.WriteString(w, `{"type":"exit","exit_code":0}`+"\n")
			case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"dry_run":false,"outcome":"deleted","root":{"kind":"instance","id":"inst"}}`)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"prepare:lb", "--image", "img", "--", "update",
			"run:psql", "--", "-c", "select 1",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestRunPlanLBCannotCombineCoverage(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	prev := parseArgsFn
	parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
		return cli.GlobalOptions{}, []cli.Command{{Name: "plan:lb"}, {Name: "status"}}, nil
	}
	t.Cleanup(func() { parseArgsFn = prev })

	if err := Run(nil); err == nil || !strings.Contains(err.Error(), "plan cannot be combined") {
		t.Fatalf("expected plan combine error, got %v", err)
	}
}

func TestRunCompositeCleanupCoverage(t *testing.T) {
	t.Run("psql cleanup error non-verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusInternalServerError, `{"message":"boom"}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:psql", "--", "-c", "select 1",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("psql cleanup blocked non-verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusConflict, `{"dry_run":false,"outcome":"blocked","root":{"kind":"instance","id":"inst","blocked":"busy","connections":1}}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:psql", "--", "-c", "select 1",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("psql cleanup blocked verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusConflict, `{"dry_run":false,"outcome":"blocked","root":{"kind":"instance","id":"inst","blocked":"busy","connections":1}}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"--verbose",
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:psql", "--", "-c", "select 1",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("pgbench cleanup error verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusInternalServerError, `{"message":"boom"}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"--verbose",
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:pgbench",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("pgbench cleanup blocked verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusConflict, `{"dry_run":false,"outcome":"blocked","root":{"kind":"instance","id":"inst","blocked":"busy","connections":1}}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"--verbose",
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:pgbench",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	t.Run("pgbench cleanup error non-verbose", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		server := compositeCleanupServer(t, http.StatusInternalServerError, `{"message":"boom"}`)
		t.Cleanup(server.Close)

		err := Run([]string{
			"--mode=remote",
			"--endpoint", server.URL,
			"prepare:psql", "--image", "img", "--", "-c", "select 1",
			"run:pgbench",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestRunPgbenchRunErrorCoverage(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/runs" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"message":"run failed"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	if err := Run([]string{"--mode=remote", "--endpoint", server.URL, "run:pgbench"}); err == nil {
		t.Fatalf("expected run error")
	}
}

func TestResolveWSLSettingsStatePathConversionCoverage(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.Distro = "Ubuntu"
	cfg.Engine.WSL.StateDir = "/mnt/sqlrs/store"
	cfg.Engine.WSL.Mount.Unit = "sqlrs.mount"
	cfg.Engine.WSL.EnginePath = "C:\\sqlrs\\sqlrs-engine.exe"

	if _, _, _, _, _, _, _, err := resolveWSLSettings(cfg, paths.Dirs{StateDir: "relative"}, "C:\\sqlrs\\sqlrs-engine.exe"); err == nil {
		t.Fatalf("expected required mode error for non-absolute state path")
	}

	cfg.Engine.WSL.Mode = "auto"
	daemonPath, runDir, statePath, stateDir, distro, unit, fs, err := resolveWSLSettings(cfg, paths.Dirs{StateDir: "relative"}, "C:\\sqlrs\\sqlrs-engine.exe")
	if err != nil {
		t.Fatalf("resolveWSLSettings: %v", err)
	}
	if daemonPath != "C:\\sqlrs\\sqlrs-engine.exe" || runDir != "" || statePath != "" || stateDir != "" || distro != "" || unit != "" || fs != "" {
		t.Fatalf("expected auto fallback, got daemon=%q runDir=%q statePath=%q stateDir=%q distro=%q unit=%q fs=%q", daemonPath, runDir, statePath, stateDir, distro, unit, fs)
	}
}

func TestWindowsToWSLPathUNCVolumeCoverage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only behavior")
	}
	out, err := windowsToWSLPath(`\\server\share\folder\file.sql`)
	if err != nil {
		t.Fatalf("windowsToWSLPath: %v", err)
	}
	if !strings.HasPrefix(out, "/mnt/") {
		t.Fatalf("unexpected converted path: %q", out)
	}
}

func compositeCleanupServer(t *testing.T, deleteStatus int, deleteBody string) *httptest.Server {
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
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
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
