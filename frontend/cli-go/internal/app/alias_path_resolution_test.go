package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/pathutil"
)

func TestRunPrepareAliasPsqlResolvesFileArgsRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "chinook.prep.s9s.yaml"), "kind: psql\nimage: image\nargs:\n  - -f\n  - chinook/prepare.sql\n")
	expected := writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "prepare.sql"), "select 1;\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "--no-watch", "examples/chinook"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "psql_args")
	if got := strings.Join(args, "|"); got != "-f|"+expected {
		t.Fatalf("psql_args = %q, want %q", got, "-f|"+expected)
	}
}

func TestRunPlanAliasPsqlResolvesFileArgsRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"status\",\"ts\":\"2026-01-24T00:00:00Z\",\"status\":\"succeeded\"}\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-f examples/chinook/prepare.sql","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"},{"task_id":"execute-0","type":"state_execute","input":{"kind":"image","id":"image"},"task_hash":"hash","output_state_id":"state-1","cached":false}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "chinook.prep.s9s.yaml"), "kind: psql\nimage: image\nargs:\n  - -f\n  - chinook/prepare.sql\n")
	expected := writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "prepare.sql"), "select 1;\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "plan", "examples/chinook"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "psql_args")
	if got := strings.Join(args, "|"); got != "-f|"+expected {
		t.Fatalf("psql_args = %q, want %q", got, "-f|"+expected)
	}
}

func TestRunPrepareAliasLiquibaseResolvesChangelogRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "liquibase.prep.s9s.yaml"), "kind: lb\nimage: image\nargs:\n  - update\n  - --changelog-file\n  - migrations/changelog.xml\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "--no-watch", "examples/liquibase"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "liquibase_args")
	want := "update|--changelog-file|" + filepath.Join(workspace, "examples", "migrations", "changelog.xml")
	if got := strings.Join(args, "|"); got != want {
		t.Fatalf("liquibase_args = %q, want %q", got, want)
	}
	if got, ok := gotRequest["work_dir"].(string); !ok || !pathutil.SameLocalPath(got, filepath.Join(workspace, "examples")) {
		t.Fatalf("work_dir = %v, want %q", got, filepath.Join(workspace, "examples"))
	}
}

func TestRunPrepareAliasLiquibaseResolvesDefaultsFileRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "liquibase.prep.s9s.yaml"), "kind: lb\nimage: image\nargs:\n  - update\n  - --defaults-file\n  - migrations/liquibase.properties\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "--no-watch", "examples/liquibase"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "liquibase_args")
	want := "update|--defaults-file|" + filepath.Join(workspace, "examples", "migrations", "liquibase.properties")
	if got := strings.Join(args, "|"); got != want {
		t.Fatalf("liquibase_args = %q, want %q", got, want)
	}
}

func TestRunPrepareAliasLiquibaseResolvesSearchPathRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "liquibase.prep.s9s.yaml"), "kind: lb\nimage: image\nargs:\n  - update\n  - --searchPath\n  - migrations,shared\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "--no-watch", "examples/liquibase"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "liquibase_args")
	want := "update|--searchPath|" + filepath.Join(workspace, "examples", "migrations") + "," + filepath.Join(workspace, "examples", "shared")
	if got := strings.Join(args, "|"); got != want {
		t.Fatalf("liquibase_args = %q, want %q", got, want)
	}
}

func TestRunPrepareAliasLiquibaseUsesSearchPathAsWorkDirForNestedAlias(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("liquibase", "jhipster-sample-app.prep.s9s.yaml"), "kind: lb\nimage: image\nargs:\n  - update\n  - --searchPath\n  - jhipster-sample-app\n  - --changelog-file\n  - jhipster-sample-app/config/liquibase/master.xml\n")
	writeTestFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app"), filepath.Join("config", "liquibase", "master.xml"), "<databaseChangeLog/>\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "prepare", "--no-watch", "liquibase/jhipster-sample-app"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := mustStringSliceField(t, gotRequest, "liquibase_args")
	wantSearch := filepath.Join(workspace, "liquibase", "jhipster-sample-app")
	wantChangelog := filepath.Join(wantSearch, "config", "liquibase", "master.xml")
	if got := strings.Join(args, "|"); got != "update|--searchPath|"+wantSearch+"|--changelog-file|"+wantChangelog {
		t.Fatalf("liquibase_args = %q, want %q", got, "update|--searchPath|"+wantSearch+"|--changelog-file|"+wantChangelog)
	}
	if got, ok := gotRequest["work_dir"].(string); !ok || !pathutil.SameLocalPath(got, wantSearch) {
		t.Fatalf("work_dir = %v, want %q", got, wantSearch)
	}
}

func TestRunPlanAliasLiquibaseUsesAliasDirWorkDir(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"status\",\"ts\":\"2026-01-24T00:00:00Z\",\"status\":\"succeeded\"}\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"lb","image_id":"image","prepare_args_normalized":"update --changelog-file config/liquibase/master.xml","tasks":[{"task_id":"plan","type":"plan","planner_kind":"lb"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "liquibase.prep.s9s.yaml"), "kind: lb\nimage: image\nargs:\n  - update\n  - --changelog-file\n  - config/liquibase/master.xml\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("config", "liquibase", "master.xml"), "<databaseChangeLog/>\n")

	withWorkingDir(t, workspace)
	_, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "--output=json", "plan", "examples/liquibase"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got, ok := gotRequest["work_dir"].(string); !ok || !pathutil.SameLocalPath(got, filepath.Join(workspace, "examples")) {
		t.Fatalf("work_dir = %v, want %q", got, filepath.Join(workspace, "examples"))
	}
}

func TestRunAliasPsqlResolvesFileArgsRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest client.RunRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"exit\",\"ts\":\"2026-01-22T00:00:01Z\",\"exit_code\":0}\n")
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writeRunAliasFile(t, workspace, filepath.Join("examples", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - queries/smoke.sql\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("queries", "smoke.sql"), "select 42;\n")

	withWorkingDir(t, workspace)
	if err := Run([]string{"--workspace", workspace, "run", "examples/smoke", "--instance", "dev"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(gotRequest.Steps) != 1 {
		t.Fatalf("expected 1 run step, got %+v", gotRequest.Steps)
	}
	if got := strings.Join(gotRequest.Steps[0].Args, "|"); got != "-f|-" {
		t.Fatalf("step args = %q, want %q", got, "-f|-")
	}
	if gotRequest.Steps[0].Stdin == nil || *gotRequest.Steps[0].Stdin != "select 42;\n" {
		t.Fatalf("unexpected stdin: %+v", gotRequest.Steps[0].Stdin)
	}
}

func TestRunAliasPgbenchKeepsNonFileArgsUnchangedFromNestedCWD(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest client.RunRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"exit\",\"ts\":\"2026-01-22T00:00:01Z\",\"exit_code\":0}\n")
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writeRunAliasFile(t, workspace, filepath.Join("examples", "bench.run.s9s.yaml"), "kind: pgbench\nargs:\n  - -c\n  - 10\n  - -T\n  - 30\n")
	cwd := filepath.Join(workspace, "examples", "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	withWorkingDir(t, cwd)
	if err := Run([]string{"--workspace", workspace, "run", "../bench", "--instance", "perf"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := strings.Join(gotRequest.Args, "|"); got != "-c|10|-T|30" {
		t.Fatalf("args = %q, want %q", got, "-c|10|-T|30")
	}
}

func TestRunAliasPgbenchResolvesFileArgsRelativeToAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotRequest client.RunRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"exit\",\"ts\":\"2026-01-22T00:00:01Z\",\"exit_code\":0}\n")
	}))
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writeRunAliasFile(t, workspace, filepath.Join("examples", "bench.run.s9s.yaml"), "kind: pgbench\nargs:\n  - -f\n  - scripts/bench.sql\n  - -T\n  - 30\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("scripts", "bench.sql"), "\\set aid random(1, 100000)\n")
	cwd := filepath.Join(workspace, "examples", "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	withWorkingDir(t, cwd)
	if err := Run([]string{"--workspace", workspace, "run", "../bench", "--instance", "perf"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "-f|/dev/stdin|-T|30"
	if got := strings.Join(gotRequest.Args, "|"); got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if gotRequest.Stdin == nil || *gotRequest.Stdin != "\\set aid random(1, 100000)\n" {
		t.Fatalf("stdin = %+v, want file content", gotRequest.Stdin)
	}
}

func TestCompositePrepareAliasRunRawFromNestedCWD(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotPrepare map[string]any
	var gotRun client.RunRequest
	server := newCompositePathResolutionServer(t, func(payload map[string]any) {
		gotPrepare = payload
	}, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "chinook.prep.s9s.yaml"), "kind: psql\nimage: image\nargs:\n  - -f\n  - chinook/prepare.sql\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "prepare.sql"), "select 1;\n")
	writeTestFile(t, filepath.Join(workspace, "examples", "chinook"), "queries.sql", "select 2;\n")
	cwd := filepath.Join(workspace, "examples", "chinook")

	withWorkingDir(t, cwd)
	if err := Run([]string{"--workspace", workspace, "prepare", "../chinook", "run:psql", "--", "-f", "queries.sql"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	prepareArgs := mustStringSliceField(t, gotPrepare, "psql_args")
	wantPrepare := filepath.Join(workspace, "examples", "chinook", "prepare.sql")
	if got := strings.Join(prepareArgs, "|"); got != "-f|"+wantPrepare {
		t.Fatalf("prepare psql_args = %q, want %q", got, "-f|"+wantPrepare)
	}
	if len(gotRun.Steps) != 1 || gotRun.Steps[0].Stdin == nil || *gotRun.Steps[0].Stdin != "select 2;\n" {
		t.Fatalf("unexpected run request: %+v", gotRun)
	}
}

func TestCompositePrepareRawRunAliasFromNestedCWD(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotPrepare map[string]any
	var gotRun client.RunRequest
	server := newCompositePathResolutionServer(t, func(payload map[string]any) {
		gotPrepare = payload
	}, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writeRunAliasFile(t, workspace, filepath.Join("examples", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - chinook/queries.sql\n")
	cwd := filepath.Join(workspace, "examples", "chinook")
	prepareFile := writeTestFile(t, cwd, "prepare.sql", "select 1;\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "queries.sql"), "select 3;\n")

	withWorkingDir(t, cwd)
	if err := Run([]string{"--workspace", workspace, "prepare:psql", "--image", "image", "--", "-f", "prepare.sql", "run", "../smoke"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	prepareArgs := mustStringSliceField(t, gotPrepare, "psql_args")
	if got := strings.Join(prepareArgs, "|"); got != "-f|"+prepareFile {
		t.Fatalf("prepare psql_args = %q, want %q", got, "-f|"+prepareFile)
	}
	if len(gotRun.Steps) != 1 || gotRun.Steps[0].Stdin == nil || *gotRun.Steps[0].Stdin != "select 3;\n" {
		t.Fatalf("unexpected run request: %+v", gotRun)
	}
}

func TestCompositePrepareAliasRunAliasFromNestedCWD(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotPrepare map[string]any
	var gotRun client.RunRequest
	server := newCompositePathResolutionServer(t, func(payload map[string]any) {
		gotPrepare = payload
	}, func(req client.RunRequest) {
		gotRun = req
	})
	defer server.Close()

	workspace := writeAliasWorkspace(t, temp, server.URL)
	writePrepareAliasFile(t, workspace, filepath.Join("examples", "chinook.prep.s9s.yaml"), "kind: psql\nimage: image\nargs:\n  - -f\n  - chinook/prepare.sql\n")
	writeRunAliasFile(t, workspace, filepath.Join("examples", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - chinook/queries.sql\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "prepare.sql"), "select 1;\n")
	writeTestFile(t, filepath.Join(workspace, "examples"), filepath.Join("chinook", "queries.sql"), "select 4;\n")
	cwd := filepath.Join(workspace, "examples", "chinook")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	withWorkingDir(t, cwd)
	if err := Run([]string{"--workspace", workspace, "prepare", "../chinook", "run", "../smoke"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	prepareArgs := mustStringSliceField(t, gotPrepare, "psql_args")
	wantPrepare := filepath.Join(workspace, "examples", "chinook", "prepare.sql")
	if got := strings.Join(prepareArgs, "|"); got != "-f|"+wantPrepare {
		t.Fatalf("prepare psql_args = %q, want %q", got, "-f|"+wantPrepare)
	}
	if len(gotRun.Steps) != 1 || gotRun.Steps[0].Stdin == nil || *gotRun.Steps[0].Stdin != "select 4;\n" {
		t.Fatalf("unexpected run request: %+v", gotRun)
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		restoreWorkingDirForTest(cwd)
	})
}

func writeTestFile(t *testing.T, baseDir, relPath, contents string) string {
	t.Helper()
	path := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func mustStringSliceField(t *testing.T, payload map[string]any, key string) []string {
	t.Helper()
	raw, ok := payload[key].([]any)
	if !ok {
		t.Fatalf("payload[%q] = %+v, want []any", key, payload[key])
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("payload[%q] item = %#v, want string", key, item)
		}
		out = append(out, value)
	}
	return out
}

func newCompositePathResolutionServer(t *testing.T, onPrepare func(map[string]any), onRun func(client.RunRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			if onPrepare != nil {
				var payload map[string]any
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &payload)
				onPrepare(payload)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"status\",\"ts\":\"2026-01-24T00:00:00Z\",\"status\":\"succeeded\"}\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-f prepare.sql"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			if onRun != nil {
				var req client.RunRequest
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &req)
				onRun(req)
			}
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, "{\"type\":\"exit\",\"exit_code\":0}\n")
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"dry_run":false,"outcome":"deleted","root":{"kind":"instance","id":"inst"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
