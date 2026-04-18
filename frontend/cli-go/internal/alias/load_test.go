package alias

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
	"github.com/sqlrs/cli/internal/pathutil"
)

func TestLoadTargetPrepareDefinition(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nimage: postgres:17\nargs:\n  - -f\n  - prepare.sql\n")

	target, err := ResolveTarget(ResolveOptions{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Ref:           "chinook",
		Class:         ClassPrepare,
	})
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if !pathutil.SameLocalPath(target.Path, aliasPath) {
		t.Fatalf("target.Path = %q, want %q", target.Path, aliasPath)
	}

	def, err := LoadTarget(target)
	if err != nil {
		t.Fatalf("LoadTarget: %v", err)
	}
	if def.Class != ClassPrepare {
		t.Fatalf("Class = %q, want %q", def.Class, ClassPrepare)
	}
	if def.Kind != "psql" {
		t.Fatalf("Kind = %q, want %q", def.Kind, "psql")
	}
	if def.Image != "postgres:17" {
		t.Fatalf("Image = %q, want %q", def.Image, "postgres:17")
	}
	if got := strings.Join(def.Args, "|"); got != "-f|prepare.sql" {
		t.Fatalf("Args = %q, want %q", got, "-f|prepare.sql")
	}
}

func TestLoadTargetRunDefinition(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: pgbench\nargs:\n  - -c\n  - 10\n")

	target, err := ResolveTarget(ResolveOptions{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Ref:           "smoke",
		Class:         ClassRun,
	})
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if !pathutil.SameLocalPath(target.Path, aliasPath) {
		t.Fatalf("target.Path = %q, want %q", target.Path, aliasPath)
	}

	def, err := LoadTarget(target)
	if err != nil {
		t.Fatalf("LoadTarget: %v", err)
	}
	if def.Class != ClassRun {
		t.Fatalf("Class = %q, want %q", def.Class, ClassRun)
	}
	if def.Kind != "pgbench" {
		t.Fatalf("Kind = %q, want %q", def.Kind, "pgbench")
	}
	if strings.TrimSpace(def.Image) != "" {
		t.Fatalf("Image = %q, want empty", def.Image)
	}
	if got := strings.Join(def.Args, "|"); got != "-c|10" {
		t.Fatalf("Args = %q, want %q", got, "-c|10")
	}
}

func TestLoadTargetWithFSSupportsPrepareAliasesInRefContexts(t *testing.T) {
	target := Target{
		Class: ClassPrepare,
		Path:  filepath.Join("repo", "examples", "chinook.prep.s9s.yaml"),
	}
	fsStub := hookAliasFS{
		readFile: func(path string) ([]byte, error) {
			if !pathutil.SameLocalPath(path, target.Path) {
				t.Fatalf("ReadFile path = %q, want %q", path, target.Path)
			}
			return []byte("kind: lb\nimage: image\nargs:\n  - update\n"), nil
		},
	}

	def, err := LoadTargetWithFS(target, fsStub)
	if err != nil {
		t.Fatalf("LoadTargetWithFS: %v", err)
	}
	if def.Class != ClassPrepare || def.Kind != "lb" || def.Image != "image" {
		t.Fatalf("unexpected definition: %+v", def)
	}
	if got := strings.Join(def.Args, "|"); got != "update" {
		t.Fatalf("Args = %q, want %q", got, "update")
	}
}

func TestLoadTargetRejectsInvalidPrepareSchema(t *testing.T) {
	workspace := t.TempDir()
	path := writeAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: flyway\nargs:\n  - migrate\n")

	_, err := LoadTarget(Target{Class: ClassPrepare, Path: path})
	if err == nil || !strings.Contains(err.Error(), "unknown prepare alias kind") {
		t.Fatalf("expected invalid prepare schema error, got %v", err)
	}
	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
}

func TestLoadTargetRejectsInvalidRunSchema(t *testing.T) {
	workspace := t.TempDir()
	path := writeAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: psql\nimage: postgres:17\nargs:\n  - -c\n  - select 1\n")

	_, err := LoadTarget(Target{Class: ClassRun, Path: path})
	if err == nil || !strings.Contains(err.Error(), "run alias does not support image") {
		t.Fatalf("expected invalid run schema error, got %v", err)
	}
	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
}

func TestLoadTargetTreatsMalformedYAMLAsUserError(t *testing.T) {
	tests := []struct {
		name  string
		class Class
		file  string
		want  string
	}{
		{
			name:  "prepare",
			class: ClassPrepare,
			file:  "broken.prep.s9s.yaml",
			want:  "read prepare alias",
		},
		{
			name:  "run",
			class: ClassRun,
			file:  "broken.run.s9s.yaml",
			want:  "read run alias",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := writeAliasFile(t, workspace, tc.file, "kind: [\n")

			_, err := LoadTarget(Target{Class: tc.class, Path: path})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected malformed YAML error containing %q, got %v", tc.want, err)
			}
			var userErr *UserError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected UserError, got %T", err)
			}
		})
	}
}

func TestLoadTargetTreatsMalformedExactFileYAMLAsRequestedUserError(t *testing.T) {
	tests := []struct {
		name  string
		class Class
		file  string
		want  string
	}{
		{
			name:  "prepare exact file without alias suffix",
			class: ClassPrepare,
			file:  "broken.txt",
			want:  "read prepare alias",
		},
		{
			name:  "run exact file without alias suffix",
			class: ClassRun,
			file:  "broken.txt",
			want:  "read run alias",
		},
		{
			name:  "prepare exact file with run suffix",
			class: ClassPrepare,
			file:  "broken.run.s9s.yaml",
			want:  "read prepare alias",
		},
		{
			name:  "run exact file with prepare suffix",
			class: ClassRun,
			file:  "broken.prep.s9s.yaml",
			want:  "read run alias",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := writeAliasFile(t, workspace, tc.file, "kind: [\n")

			_, err := LoadTarget(Target{Class: tc.class, Path: path})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected malformed YAML error containing %q, got %v", tc.want, err)
			}
			var userErr *UserError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected UserError, got %T", err)
			}
		})
	}
}

func TestCheckTargetReusesSharedAliasDefinitionLoader(t *testing.T) {
	workspace := t.TempDir()
	path := writeAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: [\n")
	target := Target{Class: ClassRun, Path: path}

	_, loadErr := LoadTarget(target)
	if loadErr == nil {
		t.Fatal("expected LoadTarget error")
	}

	result, err := CheckTarget(target, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid result: %+v", result)
	}
	if result.Error != loadErr.Error() {
		t.Fatalf("CheckTarget error = %q, want %q", result.Error, loadErr.Error())
	}
	if len(result.Issues) != 1 || result.Issues[0].Message != loadErr.Error() {
		t.Fatalf("expected shared loader issue message, got %+v", result.Issues)
	}
}

type hookAliasFS struct {
	readFile func(string) ([]byte, error)
}

func (h hookAliasFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (h hookAliasFS) ReadFile(path string) ([]byte, error) {
	if h.readFile == nil {
		return nil, os.ErrNotExist
	}
	return h.readFile(path)
}

func (h hookAliasFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

var _ inputset.FileSystem = hookAliasFS{}
