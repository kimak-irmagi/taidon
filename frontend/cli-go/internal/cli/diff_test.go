package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/diff"
)

func TestParseDiffKind(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "plan:psql", want: "psql"},
		{name: "prepare:psql", want: "psql"},
		{name: "plan:lb", want: "lb"},
		{name: "prepare:lb", want: "lb"},
		{name: " plan:psql ", want: "psql"},
		{name: "unknown", want: ""},
		{name: "", want: ""},
	}

	for _, tc := range cases {
		if got := parseDiffKind(tc.name); got != tc.want {
			t.Fatalf("parseDiffKind(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRunDiffRejectsUnsupportedWrappedName(t *testing.T) {
	root := t.TempDir()
	parsed := diff.ParsedDiff{
		Scope: diff.Scope{
			Kind:     diff.ScopeKindPath,
			FromPath: "from",
			ToPath:   "to",
		},
		WrappedName: "bad:kind",
	}

	err := RunDiff(&bytes.Buffer{}, parsed, root, "")
	if err == nil || !strings.Contains(err.Error(), "diff only supports") {
		t.Fatalf("RunDiff unsupported kind error = %v", err)
	}
}

func TestRunDiffPathScopeHumanAndJSON(t *testing.T) {
	t.Run("psql human", func(t *testing.T) {
		root := t.TempDir()
		left := filepath.Join(root, "left")
		right := filepath.Join(root, "right")
		if err := os.MkdirAll(left, 0o700); err != nil {
			t.Fatalf("mkdir left: %v", err)
		}
		if err := os.MkdirAll(right, 0o700); err != nil {
			t.Fatalf("mkdir right: %v", err)
		}
		if err := os.WriteFile(filepath.Join(left, "query.sql"), []byte("select 1;\n"), 0o600); err != nil {
			t.Fatalf("write left query: %v", err)
		}
		if err := os.WriteFile(filepath.Join(right, "query.sql"), []byte("select 2;\n"), 0o600); err != nil {
			t.Fatalf("write right query: %v", err)
		}

		parsed := diff.ParsedDiff{
			Scope: diff.Scope{
				Kind:     diff.ScopeKindPath,
				FromPath: "left",
				ToPath:   "right",
			},
			WrappedName: "plan:psql",
			WrappedArgs: []string{"-f", "query.sql"},
		}

		var out bytes.Buffer
		if err := RunDiff(&out, parsed, root, ""); err != nil {
			t.Fatalf("RunDiff(psql): %v", err)
		}
		rendered := out.String()
		if !strings.Contains(rendered, "Modified:") || !strings.Contains(rendered, "query.sql") {
			t.Fatalf("unexpected human diff output:\n%s", rendered)
		}
	})

	t.Run("liquibase json", func(t *testing.T) {
		root := t.TempDir()
		left := filepath.Join(root, "left")
		right := filepath.Join(root, "right")
		if err := os.MkdirAll(left, 0o700); err != nil {
			t.Fatalf("mkdir left: %v", err)
		}
		if err := os.MkdirAll(right, 0o700); err != nil {
			t.Fatalf("mkdir right: %v", err)
		}
		leftContent := `<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"></databaseChangeLog>` + "\n"
		rightContent := `<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog" logicalFilePath="master"></databaseChangeLog>` + "\n"
		if err := os.WriteFile(filepath.Join(left, "master.xml"), []byte(leftContent), 0o600); err != nil {
			t.Fatalf("write left changelog: %v", err)
		}
		if err := os.WriteFile(filepath.Join(right, "master.xml"), []byte(rightContent), 0o600); err != nil {
			t.Fatalf("write right changelog: %v", err)
		}

		parsed := diff.ParsedDiff{
			Scope: diff.Scope{
				Kind:     diff.ScopeKindPath,
				FromPath: "left",
				ToPath:   "right",
			},
			WrappedName: "prepare:lb",
			WrappedArgs: []string{"update", "--changelog-file", "master.xml"},
		}

		var out bytes.Buffer
		if err := RunDiff(&out, parsed, root, "json"); err != nil {
			t.Fatalf("RunDiff(lb json): %v", err)
		}

		var rendered diff.JSONOutput
		if err := json.Unmarshal(out.Bytes(), &rendered); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if rendered.Command != "prepare:lb" {
			t.Fatalf("Command = %q, want prepare:lb", rendered.Command)
		}
		if rendered.Summary.Modified != 1 || len(rendered.Modified) != 1 || rendered.Modified[0].Path != "master.xml" {
			t.Fatalf("unexpected JSON diff result: %+v", rendered)
		}
	})
}

func TestRunDiffResolveScopeError(t *testing.T) {
	err := RunDiff(&bytes.Buffer{}, diff.ParsedDiff{
		Scope: diff.Scope{
			Kind: "bogus",
		},
		WrappedName: "plan:psql",
	}, t.TempDir(), "")
	if err == nil || !strings.Contains(err.Error(), "unknown scope kind") {
		t.Fatalf("expected resolve scope error, got %v", err)
	}
}

func TestRunDiffRefScopeBuildErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, firstRef, secondRef := initDiffRefRepo(t)
	cwd := filepath.Join(repo, "workspace", "app")

	cases := []struct {
		name        string
		wrappedName string
		wrappedArgs []string
		fromRef     string
		toRef       string
		want        string
	}{
		{
			name:        "psql from-ref error",
			wrappedName: "plan:psql",
			wrappedArgs: []string{"-f", "query.sql"},
			fromRef:     firstRef,
			toRef:       secondRef,
			want:        "from-ref",
		},
		{
			name:        "psql to-ref error",
			wrappedName: "plan:psql",
			wrappedArgs: []string{"-f", "query.sql"},
			fromRef:     secondRef,
			toRef:       firstRef,
			want:        "to-ref",
		},
		{
			name:        "lb from-ref error",
			wrappedName: "prepare:lb",
			wrappedArgs: []string{"update", "--changelog-file", "master.xml"},
			fromRef:     firstRef,
			toRef:       secondRef,
			want:        "from-ref",
		},
		{
			name:        "lb to-ref error",
			wrappedName: "prepare:lb",
			wrappedArgs: []string{"update", "--changelog-file", "master.xml"},
			fromRef:     secondRef,
			toRef:       firstRef,
			want:        "to-ref",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RunDiff(&bytes.Buffer{}, diff.ParsedDiff{
				Scope: diff.Scope{
					Kind:    diff.ScopeKindRef,
					FromRef: tc.fromRef,
					ToRef:   tc.toRef,
				},
				WrappedName: tc.wrappedName,
				WrappedArgs: tc.wrappedArgs,
			}, cwd, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("RunDiff ref error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func initDiffRefRepo(t *testing.T) (string, string, string) {
	t.Helper()

	emptyTemplate := t.TempDir()
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	initCmd := exec.Command("git", "-C", repo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init skipped: %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")

	appDir := filepath.Join(repo, "workspace", "app")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, ".keep"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write .keep: %v", err)
	}
	runGit("add", "workspace")
	runGit("commit", "-m", "initial")

	firstOut, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse first HEAD: %v", err)
	}
	firstRef := strings.TrimSpace(string(firstOut))

	if err := os.WriteFile(filepath.Join(appDir, "query.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write query.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "master.xml"), []byte("<databaseChangeLog/>\n"), 0o600); err != nil {
		t.Fatalf("write master.xml: %v", err)
	}
	runGit("add", "workspace")
	runGit("commit", "-m", "add inputs")

	secondOut, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse second HEAD: %v", err)
	}
	return repo, firstRef, strings.TrimSpace(string(secondOut))
}
