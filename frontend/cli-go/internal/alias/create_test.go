package alias

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveCreateTarget(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}

	target, err := ResolveCreateTarget(CreateOptions{
		WorkspaceRoot: workspace,
		CWD:           cwd,
		Ref:           "aliases/chinook",
		Class:         ClassPrepare,
	})
	if err != nil {
		t.Fatalf("ResolveCreateTarget: %v", err)
	}
	if target.Class != ClassPrepare {
		t.Fatalf("unexpected class: %+v", target)
	}
	if target.Ref != "aliases/chinook" {
		t.Fatalf("unexpected ref: %+v", target)
	}
	if target.File != filepath.ToSlash(filepath.Join("aliases", "chinook.prep.s9s.yaml")) {
		t.Fatalf("unexpected file: %+v", target)
	}
	if !strings.HasSuffix(target.Path, filepath.Join("aliases", "chinook.prep.s9s.yaml")) {
		t.Fatalf("unexpected path: %+v", target)
	}
}

func TestResolveCreateTargetRejectsSuffixRef(t *testing.T) {
	workspace := t.TempDir()
	if _, err := ResolveCreateTarget(CreateOptions{
		WorkspaceRoot: workspace,
		Ref:           "chinook.prep.s9s.yaml",
		Class:         ClassPrepare,
	}); err == nil || !strings.Contains(err.Error(), "logical stem") {
		t.Fatalf("expected logical stem error, got %v", err)
	}
}

func TestCreateWritesPrepareAlias(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "inputs"), 0o700); err != nil {
		t.Fatalf("mkdir inputs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "inputs", "seed.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write seed.sql: %v", err)
	}

	result, err := Create(CreateOptions{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Ref:           "aliases/chinook",
		Class:         ClassPrepare,
		Kind:          "psql",
		Args: []string{
			"--image", "postgres:17",
			"--",
			"-f", "inputs/seed.sql",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Type != ClassPrepare || result.Kind != "psql" || result.Image != "postgres:17" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.File != filepath.ToSlash(filepath.Join("aliases", "chinook.prep.s9s.yaml")) {
		t.Fatalf("unexpected file: %+v", result)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var rendered struct {
		Kind  string   `yaml:"kind"`
		Image string   `yaml:"image"`
		Args  []string `yaml:"args"`
	}
	if err := yaml.Unmarshal(data, &rendered); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if rendered.Kind != "psql" {
		t.Fatalf("unexpected kind: %+v", rendered)
	}
	if rendered.Image != "postgres:17" {
		t.Fatalf("unexpected image: %+v", rendered)
	}
	if got := strings.Join(rendered.Args, "|"); got != "-f|../inputs/seed.sql" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestCreateWritesRunAlias(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "bench.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write bench.sql: %v", err)
	}

	result, err := Create(CreateOptions{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Ref:           "aliases/load",
		Class:         ClassRun,
		Kind:          "pgbench",
		Args: []string{
			"--",
			"-f", "bench.sql",
			"-T", "1",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Type != ClassRun || result.Kind != "pgbench" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Image != "" {
		t.Fatalf("did not expect image: %+v", result)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var rendered struct {
		Kind  string   `yaml:"kind"`
		Image string   `yaml:"image"`
		Args  []string `yaml:"args"`
	}
	if err := yaml.Unmarshal(data, &rendered); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if rendered.Kind != "pgbench" {
		t.Fatalf("unexpected kind: %+v", rendered)
	}
	if rendered.Image != "" {
		t.Fatalf("unexpected image: %+v", rendered)
	}
	if got := strings.Join(rendered.Args, "|"); got != "-f|../bench.sql|-T|1" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestCreateRejectsOverwrite(t *testing.T) {
	workspace := t.TempDir()
	targetPath := filepath.Join(workspace, "demo.prep.s9s.yaml")
	if err := os.WriteFile(targetPath, []byte("kind: psql\nargs:\n  - -c\n  - select 1\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}

	_, err := Create(CreateOptions{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Ref:           "demo",
		Class:         ClassPrepare,
		Kind:          "psql",
		Args:          []string{"-c", "select 1"},
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
}
