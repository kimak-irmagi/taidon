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

	aliaspkg "github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/client"
)

func TestResolveRunAliasPathFromStem(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writeRunAliasFile(t, cwd, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolveRunAliasPath(workspace, cwd, "smoke")
	if err != nil {
		t.Fatalf("resolveRunAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolveRunAliasPathParentRefFromCWD(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples", "chinook")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writeRunAliasFile(t, workspace, filepath.Join("examples", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolveRunAliasPath(workspace, cwd, "../smoke")
	if err != nil {
		t.Fatalf("resolveRunAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolveRunAliasPathExactFileEscape(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	expected := writeRunAliasFile(t, cwd, "smoke.txt", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	path, err := resolveRunAliasPath(workspace, cwd, "smoke.txt.")
	if err != nil {
		t.Fatalf("resolveRunAliasPath: %v", err)
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
}

func TestResolveRunAliasPathRejectsMissingFile(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "nested")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	_, err := resolveRunAliasPath(workspace, cwd, "missing")
	if err == nil || !strings.Contains(err.Error(), "run alias file not found") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestResolveRunAliasPathRejectsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples", "chinook")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	ref := filepath.ToSlash(filepath.Join("..", "..", "..", "outside.run.s9s.yaml")) + "."
	_, err := resolveRunAliasPath(workspace, cwd, ref)
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestLoadRunAliasRequiresKind(t *testing.T) {
	path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "args:\n  - -c\n  - select 1\n")

	_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
	if err == nil || !strings.Contains(err.Error(), "run alias kind is required") {
		t.Fatalf("expected missing kind error, got %v", err)
	}
}

func TestLoadRunAliasRequiresArgs(t *testing.T) {
	path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "kind: psql\n")

	_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
	if err == nil || !strings.Contains(err.Error(), "run alias args are required") {
		t.Fatalf("expected missing args error, got %v", err)
	}
}

func TestLoadRunAliasRejectsUnknownKind(t *testing.T) {
	path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "kind: unknown\nargs:\n  - -c\n  - select 1\n")

	_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
	if err == nil || !strings.Contains(err.Error(), "unknown run alias kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}

func TestLoadRunAliasRejectsImageField(t *testing.T) {
	path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "kind: psql\nimage: postgres:17\nargs:\n  - -c\n  - select 1\n")

	_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
	if err == nil || !strings.Contains(err.Error(), "run alias does not support image") {
		t.Fatalf("expected image-field rejection, got %v", err)
	}
}

func TestLoadRunAliasPreservesArgOrder(t *testing.T) {
	path := writeRunAliasFile(t, t.TempDir(), "ordered.run.s9s.yaml", "kind: pgbench\nargs:\n  - -c\n  - 10\n  - -T\n  - 30\n")

	alias, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
	if err != nil {
		t.Fatalf("LoadTarget: %v", err)
	}
	if got := strings.Join(alias.Args, "|"); got != "-c|10|-T|30" {
		t.Fatalf("args = %q, want %q", got, "-c|10|-T|30")
	}
}

func TestParseRunAliasArgsRequiresInstanceForStandalone(t *testing.T) {
	_, _, err := parseRunAliasArgs([]string{"smoke"}, true)
	if err == nil || !strings.Contains(err.Error(), "run alias requires --instance") {
		t.Fatalf("expected missing instance error, got %v", err)
	}
}

func TestParseRunAliasArgsAcceptsInstanceFlag(t *testing.T) {
	invocation, showHelp, err := parseRunAliasArgs([]string{"smoke", "--instance", "dev"}, true)
	if err != nil {
		t.Fatalf("parseRunAliasArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("unexpected help")
	}
	if invocation.Ref != "smoke" {
		t.Fatalf("ref = %q, want smoke", invocation.Ref)
	}
	if invocation.InstanceRef != "dev" {
		t.Fatalf("instance = %q, want dev", invocation.InstanceRef)
	}
}

func TestParseRunAliasArgsRejectsInlineToolArgs(t *testing.T) {
	_, _, err := parseRunAliasArgs([]string{"smoke", "--", "-c", "select 1"}, true)
	if err == nil || !strings.Contains(err.Error(), "run aliases do not accept inline tool args") {
		t.Fatalf("expected inline args rejection, got %v", err)
	}
}

func TestParseRunAliasArgsRejectsUnknownOption(t *testing.T) {
	_, _, err := parseRunAliasArgs([]string{"smoke", "--bad"}, true)
	if err == nil || !strings.Contains(err.Error(), "unknown run alias option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

func TestRunAliasCommandPsqlRemote(t *testing.T) {
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
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "run", "smoke", "--instance", "dev"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRequest.InstanceRef != "dev" {
		t.Fatalf("instance_ref = %q, want dev", gotRequest.InstanceRef)
	}
	if gotRequest.Kind != "psql" {
		t.Fatalf("kind = %q, want psql", gotRequest.Kind)
	}
	if len(gotRequest.Steps) != 1 || strings.Join(gotRequest.Steps[0].Args, " ") != "-c select 1" {
		t.Fatalf("unexpected run steps: %+v", gotRequest.Steps)
	}
}

func TestRunAliasCommandPgbenchRemote(t *testing.T) {
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
	writeRunAliasFile(t, workspace, "bench.run.s9s.yaml", "kind: pgbench\nargs:\n  - -c\n  - 10\n")
	withWorkingDir(t, workspace)

	if err := Run([]string{"--workspace", workspace, "run", "bench", "--instance", "perf"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotRequest.InstanceRef != "perf" {
		t.Fatalf("instance_ref = %q, want perf", gotRequest.InstanceRef)
	}
	if gotRequest.Kind != "pgbench" {
		t.Fatalf("kind = %q, want pgbench", gotRequest.Kind)
	}
	if got := strings.Join(gotRequest.Args, " "); got != "-c 10" {
		t.Fatalf("args = %q, want %q", got, "-c 10")
	}
}

func TestRunAliasMissingRef(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"run"})
	if err == nil || !strings.Contains(err.Error(), "missing run alias ref") {
		t.Fatalf("expected missing alias ref error, got %v", err)
	}
}

func TestRunAliasMissingInstance(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	err := Run([]string{"--workspace", workspace, "run", "smoke"})
	if err == nil || !strings.Contains(err.Error(), "run alias requires --instance") {
		t.Fatalf("expected missing instance error, got %v", err)
	}
}

func writeRunAliasFile(t *testing.T, workspace, relPath, contents string) string {
	t.Helper()
	path := filepath.Join(workspace, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir alias dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	return path
}
