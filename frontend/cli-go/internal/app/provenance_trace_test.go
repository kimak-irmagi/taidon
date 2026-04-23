package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/inputset"
	"github.com/sqlrs/cli/internal/refctx"
)

func TestCollectPrepareTraceCapturesPsqlInvocationInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		stdin    *string
		setup    func(t *testing.T, cwd string)
		wantPath string
	}{
		{
			name: "command include",
			args: []string{"-c", `\i schema.sql`},
			setup: func(t *testing.T, cwd string) {
				writeTraceFile(t, filepath.Join(cwd, "schema.sql"), "select 1;\n")
			},
			wantPath: "app/schema.sql",
		},
		{
			name:  "stdin include",
			args:  []string{"-f", "-"},
			stdin: stringPtr("\\i includes/seed.sql\n"),
			setup: func(t *testing.T, cwd string) {
				writeTraceFile(t, filepath.Join(cwd, "includes", "seed.sql"), "select 1;\n")
			},
			wantPath: "app/includes/seed.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "app")
			if err := os.MkdirAll(cwd, 0o700); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			tt.setup(t, cwd)

			trace, err := collectPrepareTrace(stageRunRequest{
				mode:          stageModePlan,
				class:         "raw",
				kind:          "psql",
				parsed:        prepareArgs{PsqlArgs: tt.args},
				workspaceRoot: root,
				cwd:           cwd,
				invocationCwd: cwd,
			}, cli.PrepareOptions{
				PrepareKind: "psql",
				ImageID:     "img",
				PsqlArgs:    tt.args,
				Stdin:       tt.stdin,
			}, nil)
			if err != nil {
				t.Fatalf("collectPrepareTrace: %v", err)
			}
			if len(trace.Inputs) != 1 {
				t.Fatalf("inputs = %+v, want 1 entry", trace.Inputs)
			}
			if trace.Inputs[0].Path != tt.wantPath {
				t.Fatalf("inputs[0].Path = %q, want %q", trace.Inputs[0].Path, tt.wantPath)
			}
		})
	}
}

func TestCollectPrepareTracePreservesCallerWorkspaceRootForRefWorktree(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	cwd := filepath.Join(workspaceRoot, "app")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("MkdirAll workspace cwd: %v", err)
	}

	projectedRoot := t.TempDir()
	aliasPath := filepath.Join(projectedRoot, "aliases", "demo.prep.s9s.yaml")
	writeTraceFile(t, aliasPath, "kind: psql\nimage: img\nargs:\n  - -c\n  - select 1\n")

	actualRef := &refctx.Context{
		RepoRoot:       projectedRoot,
		WorkspaceRoot:  projectedRoot,
		BaseDir:        filepath.Join(projectedRoot, "app"),
		GitRef:         "HEAD^",
		ResolvedCommit: "abc123",
		RefMode:        "worktree",
		FileSystem:     inputset.OSFileSystem{},
	}

	trace, err := collectPrepareTrace(stageRunRequest{
		mode:          stageModePlan,
		class:         "alias",
		kind:          "psql",
		parsed:        prepareArgs{PsqlArgs: []string{"-c", "select 1"}},
		workspaceRoot: workspaceRoot,
		cwd:           cwd,
		invocationCwd: cwd,
		aliasPath:     aliasPath,
		ref:           actualRef,
	}, cli.PrepareOptions{
		PrepareKind: "psql",
		ImageID:     "img",
		PsqlArgs:    []string{"-c", "select 1"},
	}, actualRef)
	if err != nil {
		t.Fatalf("collectPrepareTrace: %v", err)
	}

	if trace.WorkspaceRoot != workspaceRoot {
		t.Fatalf("WorkspaceRoot = %q, want %q", trace.WorkspaceRoot, workspaceRoot)
	}
	if trace.CWD != cwd {
		t.Fatalf("CWD = %q, want %q", trace.CWD, cwd)
	}
	if trace.AliasPath != "aliases/demo.prep.s9s.yaml" {
		t.Fatalf("AliasPath = %q, want %q", trace.AliasPath, "aliases/demo.prep.s9s.yaml")
	}
	if len(trace.Inputs) != 1 {
		t.Fatalf("inputs = %+v, want 1 entry", trace.Inputs)
	}
	if trace.Inputs[0].Path != "aliases/demo.prep.s9s.yaml" {
		t.Fatalf("inputs[0].Path = %q, want %q", trace.Inputs[0].Path, "aliases/demo.prep.s9s.yaml")
	}
	if trace.RefContext == nil || trace.RefContext.Mode != "worktree" {
		t.Fatalf("RefContext = %+v, want worktree context", trace.RefContext)
	}
}

func writeTraceFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
