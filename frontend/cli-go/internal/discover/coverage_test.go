package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
)

func TestDiscoverHelperCoverage(t *testing.T) {
	t.Run("priority and string helpers", func(t *testing.T) {
		priorityCases := []struct {
			name     string
			proposal candidateProposal
			want     int
		}{
			{name: "prepare liquibase", proposal: candidateProposal{Class: alias.ClassPrepare, Kind: "lb"}, want: 0},
			{name: "prepare psql", proposal: candidateProposal{Class: alias.ClassPrepare, Kind: "psql"}, want: 1},
			{name: "run pgbench", proposal: candidateProposal{Class: alias.ClassRun, Kind: "pgbench"}, want: 2},
			{name: "run psql", proposal: candidateProposal{Class: alias.ClassRun, Kind: "psql"}, want: 3},
			{name: "default", proposal: candidateProposal{Class: alias.ClassRun, Kind: "other"}, want: 4},
		}
		for _, tc := range priorityCases {
			t.Run(tc.name, func(t *testing.T) {
				if got := proposalPriority(tc.proposal); got != tc.want {
					t.Fatalf("proposalPriority = %d, want %d", got, tc.want)
				}
			})
		}

		stemCases := []struct {
			name  string
			value string
			want  string
		}{
			{name: "empty", value: "", want: "."},
			{name: "no extension", value: "schema", want: "schema"},
			{name: "dotfile", value: ".sql", want: ".sql"},
			{name: "with extension", value: filepath.Join("dir", "schema.sql"), want: "schema"},
		}
		for _, tc := range stemCases {
			t.Run(tc.name, func(t *testing.T) {
				if got := pathStem(tc.value); got != tc.want {
					t.Fatalf("pathStem(%q) = %q, want %q", tc.value, got, tc.want)
				}
			})
		}

		ancestorCases := []struct {
			name  string
			value string
			want  bool
		}{
			{name: "empty", value: "", want: false},
			{name: "single ancestor", value: "..", want: true},
			{name: "multiple ancestors", value: "../..", want: true},
			{name: "mixed path", value: "../child", want: false},
			{name: "current dir", value: ".", want: false},
		}
		for _, tc := range ancestorCases {
			t.Run(tc.name, func(t *testing.T) {
				if got := isAncestorOnlyPath(tc.value); got != tc.want {
					t.Fatalf("isAncestorOnlyPath(%q) = %v, want %v", tc.value, got, tc.want)
				}
			})
		}

		appendCases := []struct {
			name   string
			base   string
			reason string
			want   string
		}{
			{name: "empty both", base: "", reason: "", want: ""},
			{name: "base only", base: "base", reason: "", want: "base"},
			{name: "reason only", base: "", reason: "reason", want: "reason"},
			{name: "both", base: "base", reason: "reason", want: "base; reason"},
		}
		for _, tc := range appendCases {
			t.Run(tc.name, func(t *testing.T) {
				if got := appendReason(tc.base, tc.reason); got != tc.want {
					t.Fatalf("appendReason(%q, %q) = %q, want %q", tc.base, tc.reason, got, tc.want)
				}
			})
		}

		liquibaseExtCases := []struct {
			value string
			want  bool
		}{
			{value: ".xml", want: true},
			{value: ".sql", want: true},
			{value: ".jar", want: true},
			{value: ".txt", want: false},
		}
		for _, tc := range liquibaseExtCases {
			if got := isLiquibaseCandidateExtension(tc.value); got != tc.want {
				t.Fatalf("isLiquibaseCandidateExtension(%q) = %v, want %v", tc.value, got, tc.want)
			}
		}

		if got := liquibaseRootHint("services/app/config/liquibase/master.xml"); got != "services/app" {
			t.Fatalf("liquibaseRootHint nested marker = %q", got)
		}
		if got := liquibaseRootHint("config/liquibase/master.xml"); got != "" {
			t.Fatalf("liquibaseRootHint root marker = %q", got)
		}
		if got := liquibaseRootHint("services/app/db/changelog/master.xml"); got != "services/app" {
			t.Fatalf("liquibaseRootHint db marker = %q", got)
		}
		if got := liquibaseRootHint("db/changelog/master.xml"); got != "" {
			t.Fatalf("liquibaseRootHint db root marker = %q", got)
		}
		if got := liquibaseRootHint("services/app/CONFIG/LIQUIBASE/master.xml"); got != "services/app" {
			t.Fatalf("liquibaseRootHint case-insensitive marker = %q", got)
		}

		workspace := t.TempDir()
		absPath := filepath.Join(workspace, "schema.sql")
		if got := stableDiscoverAbsPath(""); got != "." {
			t.Fatalf("stableDiscoverAbsPath(empty) = %q", got)
		}
		if got := stableDiscoverAbsPath("dir/schema.sql"); got != filepath.Clean("dir/schema.sql") {
			t.Fatalf("stableDiscoverAbsPath(relative) = %q", got)
		}
		wantAbs := filepath.Clean(inputset.CanonicalizeBoundaryPath(absPath))
		if got := stableDiscoverAbsPath(absPath); got != wantAbs {
			t.Fatalf("stableDiscoverAbsPath(abs) = %q, want %q", got, wantAbs)
		}
		if runtime.GOOS == "windows" {
			linkRoot := filepath.Join(workspace, "link")
			realRoot := filepath.Join(workspace, "real")
			if err := os.MkdirAll(realRoot, 0o755); err != nil {
				t.Fatalf("mkdir real root: %v", err)
			}
			if err := os.Symlink(realRoot, linkRoot); err != nil {
				t.Skipf("symlink unsupported: %v", err)
			}
			linkPath := filepath.Join(linkRoot, "schema.sql")
			got := stableDiscoverAbsPath(linkPath)
			if got == filepath.Clean(linkPath) {
				t.Fatalf("stableDiscoverAbsPath(symlink) did not resolve link path: %q", got)
			}
			want := filepath.Clean(inputset.CanonicalizeBoundaryPath(filepath.Join(realRoot, "schema.sql")))
			gotCanonical := filepath.Clean(inputset.CanonicalizeBoundaryPath(got))
			if gotCanonical != want {
				t.Fatalf("stableDiscoverAbsPath(symlink) = %q (canonical %q), want canonical %q", got, gotCanonical, want)
			}
		}

		aliasDir := filepath.Join(workspace, "aliases")
		nestedFile := filepath.Join(aliasDir, "nested", "file.sql")
		if got := validationPathForAliasDir(filepath.Join(aliasDir, "file.sql"), "fallback", aliasDir); got != "file.sql" {
			t.Fatalf("validationPathForAliasDir(relative) = %q", got)
		}
		if got := validationPathForAliasDir(aliasDir, "fallback", aliasDir); got != "fallback" {
			t.Fatalf("validationPathForAliasDir(fallback) = %q", got)
		}
		if got := validationPathForAliasDir(nestedFile, "fallback", aliasDir); got != filepath.ToSlash(filepath.Join("nested", "file.sql")) {
			t.Fatalf("validationPathForAliasDir(nested) = %q", got)
		}
		if runtime.GOOS == "windows" {
			if got := validationPathForAliasDir(`D:\other\file.sql`, "fallback", `C:\aliases`); got != "fallback" {
				t.Fatalf("validationPathForAliasDir cross-volume fallback = %q", got)
			}
		}

		quoteCases := []struct {
			goos  string
			value string
			want  string
		}{
			{goos: "windows", value: "sqlrs", want: "sqlrs"},
			{goos: "windows", value: "hello world", want: "'hello world'"},
			{goos: "linux", value: "sqlrs", want: "sqlrs"},
			{goos: "linux", value: "hello world", want: "'hello world'"},
			{goos: "linux", value: "", want: "''"},
		}
		for _, tc := range quoteCases {
			if got := shellQuoteForGoOS(tc.goos, tc.value); got != tc.want {
				t.Fatalf("shellQuoteForGoOS(%q, %q) = %q, want %q", tc.goos, tc.value, got, tc.want)
			}
		}
		if !isPowerShellBareWord("sqlrs") || !isPowerShellBareWord("a.b_c/d:e-f") || isPowerShellBareWord("hello world") || isPowerShellBareWord("hello+world") {
			t.Fatalf("unexpected PowerShell bare-word classification")
		}

		createCases := []struct {
			name  string
			class alias.Class
			kind  string
			want  string
		}{
			{name: "prepare psql", class: alias.ClassPrepare, kind: "psql", want: "prepare:psql"},
			{name: "prepare lb", class: alias.ClassPrepare, kind: "lb", want: "prepare:lb"},
			{name: "run psql", class: alias.ClassRun, kind: "psql", want: "run:psql"},
			{name: "run pgbench", class: alias.ClassRun, kind: "pgbench", want: "run:pgbench"},
		}
		for _, tc := range createCases {
			t.Run(tc.name, func(t *testing.T) {
				got := buildCreateCommand("demo", tc.class, tc.kind, "file.sql")
				if !strings.Contains(got, tc.want) {
					t.Fatalf("buildCreateCommand = %q, want %q", got, tc.want)
				}
			})
		}
		if got := buildCreateCommand("demo", alias.ClassRun, "unknown", "file.sql"); got != "" {
			t.Fatalf("buildCreateCommand default = %q", got)
		}
	})

	t.Run("score and ref helpers", func(t *testing.T) {
		workspace := t.TempDir()
		absPath := filepath.Join(workspace, "schema.sql")
		relativeCases := []struct {
			name string
			file fileRecord
			want string
		}{
			{
				name: "empty cwd uses workspace root",
				file: fileRecord{
					AbsPath:       absPath,
					WorkspaceRoot: workspace,
					WorkspaceRel:  "schema.sql",
					CwdRel:        "",
				},
				want: filepath.ToSlash(filepath.Join(workspace, "schema")),
			},
			{
				name: "absolute cwd with root file",
				file: fileRecord{
					AbsPath:       absPath,
					WorkspaceRoot: workspace,
					WorkspaceRel:  ".",
					CwdRel:        filepath.Join(workspace, "cwd"),
				},
				want: filepath.ToSlash(filepath.Join(workspace, "schema")),
			},
			{
				name: "absolute cwd with nested file",
				file: fileRecord{
					AbsPath:       absPath,
					WorkspaceRoot: workspace,
					WorkspaceRel:  "nested/schema.sql",
					CwdRel:        filepath.Join(workspace, "cwd"),
				},
				want: filepath.ToSlash(filepath.Join(workspace, "nested")),
			},
			{
				name: "relative cwd returns stem",
				file: fileRecord{
					AbsPath:       absPath,
					WorkspaceRoot: workspace,
					WorkspaceRel:  "schema.sql",
					CwdRel:        "subdir/schema.sql",
				},
				want: "subdir",
			},
			{
				name: "ancestor cwd keeps prefix",
				file: fileRecord{
					AbsPath:       absPath,
					WorkspaceRoot: workspace,
					WorkspaceRel:  "nested/schema.sql",
					CwdRel:        "../schema.sql",
				},
				want: filepath.ToSlash(filepath.Join("..", "schema")),
			},
		}
		for _, tc := range relativeCases {
			t.Run(tc.name, func(t *testing.T) {
				if got := suggestedAliasRef(tc.file); got != tc.want {
					t.Fatalf("suggestedAliasRef = %q, want %q", got, tc.want)
				}
			})
		}

		scoreCases := []struct {
			name string
			got  candidateProposal
			want int
		}{
			{
				name: "run pgbench",
				got: scoreRunPgbench(fileRecord{
					AbsPath:       filepath.Join(workspace, "bench", "perf.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  "bench/perf.sql",
					CwdRel:        "bench/perf.sql",
					Ext:           ".sql",
					LowerPath:     "bench/perf.sql",
					Content:       "pgbench\n\\setrandom id 1 10\n",
				}),
				want: 70,
			},
			{
				name: "run psql",
				got: scoreRunPsql(fileRecord{
					AbsPath:       filepath.Join(workspace, "queries", "report.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  "queries/report.sql",
					CwdRel:        "queries/report.sql",
					Ext:           ".sql",
					LowerPath:     "queries/report.sql",
					Content:       "select 1;\ncreate table demo(id int);\n",
				}),
				want: 75,
			},
		}
		for _, tc := range scoreCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.got.Score != tc.want {
					t.Fatalf("%s score = %d, want %d", tc.name, tc.got.Score, tc.want)
				}
				if tc.got.Ref == "" {
					t.Fatalf("%s ref should not be empty: %+v", tc.name, tc.got)
				}
			})
		}
		if got := scoreRunPgbench(fileRecord{Ext: ".txt"}); got.Score != 0 {
			t.Fatalf("scoreRunPgbench non-sql = %+v", got)
		}
		if got := scoreRunPsql(fileRecord{Ext: ".txt"}); got.Score != 0 {
			t.Fatalf("scoreRunPsql non-sql = %+v", got)
		}
	})
}

func TestDiscoverScanAndAliasCoverageBranches(t *testing.T) {
	t.Run("alias coverage and validation", func(t *testing.T) {
		workspace := t.TempDir()
		aliasDir := filepath.Join(workspace, "aliases")
		if err := os.MkdirAll(filepath.Join(aliasDir, "queries"), 0o755); err != nil {
			t.Fatalf("mkdir queries: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(aliasDir, "bench"), 0o755); err != nil {
			t.Fatalf("mkdir bench: %v", err)
		}

		write := func(path string, content string) {
			t.Helper()
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}

		write(filepath.Join(aliasDir, "queries", "root.sql"), "select 1;\n")
		write(filepath.Join(aliasDir, "bench", "bench.sql"), "\\setrandom id 1 10\n")
		write(filepath.Join(aliasDir, "good.prep.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - queries/root.sql\n")
		write(filepath.Join(aliasDir, "good.run.s9s.yaml"), "kind: pgbench\nargs:\n  - -f\n  - bench/bench.sql\n")
		write(filepath.Join(aliasDir, "broken.prep.s9s.yaml"), ": not yaml\n")
		write(filepath.Join(aliasDir, "empty.prep.s9s.yaml"), "kind: psql\nargs: []\n")

		coverage, err := loadAliasCoverage(workspace)
		if err != nil {
			t.Fatalf("loadAliasCoverage: %v", err)
		}
		if _, ok := coverage["aliases/queries/root.sql"]; !ok {
			t.Fatalf("expected psql closure key in coverage: %+v", coverage)
		}
		if _, ok := coverage["aliases/bench/bench.sql"]; !ok {
			t.Fatalf("expected pgbench closure key in coverage: %+v", coverage)
		}

		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.Class("other"), Path: filepath.Join(aliasDir, "good.prep.s9s.yaml")}); err != nil || got != nil {
			t.Fatalf("aliasCoveragePaths unsupported class = %+v, %v", got, err)
		}
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: filepath.Join(aliasDir, "empty.prep.s9s.yaml")}); err != nil || got != nil {
			t.Fatalf("aliasCoveragePaths empty args = %+v, %v", got, err)
		}
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: ""}); err != nil || got != nil {
			t.Fatalf("aliasCoveragePaths empty path = %+v, %v", got, err)
		}
		write(filepath.Join(aliasDir, "unknown.prep.s9s.yaml"), "kind: flyway\nargs:\n  - migrate\n")
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: filepath.Join(aliasDir, "unknown.prep.s9s.yaml")}); err != nil || got != nil {
			t.Fatalf("aliasCoveragePaths unsupported kind = %+v, %v", got, err)
		}
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: filepath.Join(aliasDir, "good.prep.s9s.yaml")}); err != nil || got == nil {
			t.Fatalf("aliasCoveragePaths prepare psql = %+v, %v", got, err)
		}
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassRun, Path: filepath.Join(aliasDir, "good.run.s9s.yaml")}); err != nil || got == nil {
			t.Fatalf("aliasCoveragePaths run pgbench = %+v, %v", got, err)
		}
		if err := os.MkdirAll(filepath.Join(aliasDir, "lb"), 0o755); err != nil {
			t.Fatalf("mkdir lb: %v", err)
		}
		write(filepath.Join(aliasDir, "lb", "root.xml"), `<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"/>`)
		write(filepath.Join(aliasDir, "lb.prep.s9s.yaml"), "kind: lb\nargs:\n  - update\n  - --changelog-file\n  - lb/root.xml\n")
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: filepath.Join(aliasDir, "lb.prep.s9s.yaml")}); err != nil || got == nil {
			t.Fatalf("aliasCoveragePaths prepare lb = %+v, %v", got, err)
		}
		write(filepath.Join(aliasDir, "psql.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - queries/root.sql\n")
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassRun, Path: filepath.Join(aliasDir, "psql.run.s9s.yaml")}); err != nil || got == nil {
			t.Fatalf("aliasCoveragePaths run psql = %+v, %v", got, err)
		}
		if _, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassPrepare, Path: filepath.Join(aliasDir, "broken.prep.s9s.yaml")}); err == nil {
			t.Fatalf("expected invalid YAML error")
		}

		result, err := validateCandidate(candidateProposal{
			fileRecord: fileRecord{
				AbsPath:       filepath.Join(aliasDir, "queries", "root.sql"),
				WorkspaceRoot: workspace,
				WorkspaceRel:  "aliases/queries/root.sql",
				CwdRel:        "aliases/queries/root.sql",
			},
			Class: alias.ClassPrepare,
			Kind:  "weird",
			Ref:   "aliases/queries/root",
		}, workspace, workspace)
		if err != nil {
			t.Fatalf("validateCandidate unsupported kind: %v", err)
		}
		if result.Valid || result.Error == "" || result.Command != "" {
			t.Fatalf("unexpected unsupported candidate result: %+v", result)
		}
	})

	t.Run("walk, classify, and snippet helpers", func(t *testing.T) {
		workspace := t.TempDir()
		for _, dir := range []string{".sqlrs", ".git", "node_modules", "vendor"} {
			skipped := filepath.Join(workspace, dir, "nested")
			if err := os.MkdirAll(skipped, 0o755); err != nil {
				t.Fatalf("mkdir skipped %s: %v", dir, err)
			}
			if err := os.WriteFile(filepath.Join(skipped, "skip.sql"), []byte("select 1;\n"), 0o600); err != nil {
				t.Fatalf("write skipped %s: %v", dir, err)
			}
		}
		for i := 0; i < 64; i++ {
			if err := os.WriteFile(filepath.Join(workspace, fmt.Sprintf("file-%02d.sql", i)), []byte("select 1;\n"), 0o600); err != nil {
				t.Fatalf("write root file %d: %v", i, err)
			}
		}

		progress := &recordingProgress{}
		records, scanned, err := walkDiscoverFiles(workspace, workspace, progress)
		if err != nil {
			t.Fatalf("walkDiscoverFiles: %v", err)
		}
		if scanned != 64 {
			t.Fatalf("unexpected scanned count: %d", scanned)
		}
		if len(progress.events) == 0 {
			t.Fatalf("expected scan progress events")
		}

		notes := filepath.Join(workspace, "notes.txt")
		if err := os.WriteFile(notes, []byte("not supported\n"), 0o600); err != nil {
			t.Fatalf("write notes: %v", err)
		}
		if rec, ok := classifyDiscoverFile(workspace, workspace, notes); ok || rec.AbsPath != "" {
			t.Fatalf("expected unsupported extension to be skipped: %+v ok=%v", rec, ok)
		}

		classPath := filepath.Join(workspace, "build", "A.class")
		if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
			t.Fatalf("mkdir class dir: %v", err)
		}
		if err := os.WriteFile(classPath, []byte("binary"), 0o600); err != nil {
			t.Fatalf("write class file: %v", err)
		}
		if rec, ok := classifyDiscoverFile(workspace, workspace, classPath); !ok || !rec.BinaryOnly || rec.Content != "" {
			t.Fatalf("expected binary discover record: %+v ok=%v", rec, ok)
		}

		sqlPath := filepath.Join(workspace, "sql", "query.sql")
		if err := os.MkdirAll(filepath.Dir(sqlPath), 0o755); err != nil {
			t.Fatalf("mkdir sql dir: %v", err)
		}
		if err := os.WriteFile(sqlPath, []byte("select 1;\n"), 0o600); err != nil {
			t.Fatalf("write sql file: %v", err)
		}
		if rec, ok := classifyDiscoverFile(workspace, workspace, sqlPath); !ok || rec.BinaryOnly || rec.Content == "" {
			t.Fatalf("expected sql discover record: %+v ok=%v", rec, ok)
		}
		if runtime.GOOS == "windows" {
			if rec, ok := classifyDiscoverFile(workspace, `D:\cwd`, sqlPath); !ok || rec.CwdRel != filepath.ToSlash(sqlPath) {
				t.Fatalf("expected cross-drive cwd fallback, got %+v ok=%v", rec, ok)
			}
		}

		if got, ok := stableDiscoverRelativePath(workspace, sqlPath, false); !ok || got != filepath.ToSlash(filepath.Join("sql", "query.sql")) {
			t.Fatalf("stableDiscoverRelativePath = %q, ok=%v", got, ok)
		}
		if runtime.GOOS == "windows" {
			if got, ok := stableDiscoverRelativePath(`C:\base`, `D:\target.sql`, true); !ok || got != `D:/target.sql` {
				t.Fatalf("stableDiscoverRelativePath fallback = %q, ok=%v", got, ok)
			}
			if _, ok := stableDiscoverRelativePath(`C:\base`, `D:\target.sql`, false); ok {
				t.Fatalf("expected cross-volume relative path to fail without fallback")
			}
		}

		largePath := filepath.Join(workspace, "big.sql")
		if err := os.WriteFile(largePath, []byte(strings.Repeat("x", 32*1024+1)), 0o600); err != nil {
			t.Fatalf("write large snippet: %v", err)
		}
		if got, err := readDiscoverSnippet(largePath); err != nil || len(got) != 32*1024 {
			t.Fatalf("readDiscoverSnippet large = %q, %v", got, err)
		}
		if _, err := readDiscoverSnippet(filepath.Join(workspace, "missing.sql")); err == nil {
			t.Fatalf("expected missing snippet error")
		}

		if len(records) != 64 {
			t.Fatalf("expected 64 scanned records, got %d", len(records))
		}
		if len(progress.events) < 2 {
			t.Fatalf("expected scan heartbeat events, got %+v", progress.events)
		}
	})
}

func TestAnalyzeAliasesSuppressesDuplicateAliasPath(t *testing.T) {
	workspace := t.TempDir()
	dupDir := filepath.Join(workspace, "dup")
	if err := os.MkdirAll(dupDir, 0o755); err != nil {
		t.Fatalf("mkdir dup dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dupDir, "a.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write a.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dupDir, "b.sql"), []byte("select 2;\n"), 0o600); err != nil {
		t.Fatalf("write b.sql: %v", err)
	}

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected duplicate alias path suppression, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one remaining finding, got %+v", report)
	}
	if report.Findings[0].AliasPath != filepath.ToSlash(filepath.Join("dup.run.s9s.yaml")) {
		t.Fatalf("unexpected alias path: %+v", report.Findings[0])
	}
}

func TestDiscoverErrorBranches(t *testing.T) {
	t.Run("analyze aliases input and cwd handling", func(t *testing.T) {
		workspace := t.TempDir()

		t.Run("missing workspace root", func(t *testing.T) {
			if _, err := AnalyzeAliases(Options{}); err == nil {
				t.Fatal("expected missing workspace root error")
			}
		})

		t.Run("default cwd branch", func(t *testing.T) {
			report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace})
			if err != nil {
				t.Fatalf("AnalyzeAliases default cwd: %v", err)
			}
			if report.Scanned != 0 {
				t.Fatalf("unexpected report: %+v", report)
			}
		})
	})

	t.Run("load and scan error branches", func(t *testing.T) {
		missingRoot := filepath.Join(t.TempDir(), "missing")
		if _, err := loadAliasCoverage(missingRoot); err == nil {
			t.Fatal("expected loadAliasCoverage error")
		}
		if _, _, err := walkDiscoverFiles(missingRoot, missingRoot, nil); err == nil {
			t.Fatal("expected walkDiscoverFiles error")
		}
	})

	t.Run("alias and helper error branches", func(t *testing.T) {
		workspace := t.TempDir()
		aliasDir := filepath.Join(workspace, "aliases")
		if err := os.MkdirAll(aliasDir, 0o755); err != nil {
			t.Fatalf("mkdir aliases: %v", err)
		}

		missingAlias := alias.Entry{Class: alias.ClassRun, Path: filepath.Join(aliasDir, "missing.yaml")}
		if _, err := aliasCoveragePaths(workspace, missingAlias); err == nil {
			t.Fatal("expected aliasCoveragePaths read error")
		}

		unsupportedAlias := filepath.Join(aliasDir, "unsupported.run.s9s.yaml")
		if err := os.WriteFile(unsupportedAlias, []byte("kind: flyway\nargs:\n  - migrate\n"), 0o600); err != nil {
			t.Fatalf("write unsupported alias: %v", err)
		}
		if got, err := aliasCoveragePaths(workspace, alias.Entry{Class: alias.ClassRun, Path: unsupportedAlias}); err != nil || got != nil {
			t.Fatalf("expected unsupported run kind to be ignored: %+v, %v", got, err)
		}

		if rec, ok := classifyDiscoverFile(workspace, workspace, filepath.Join(aliasDir, "missing.sql")); ok || rec.AbsPath != "" {
			t.Fatalf("expected missing discover file to be skipped: %+v ok=%v", rec, ok)
		}
		if got := shellQuoteForGoOS("linux", ""); got != "''" {
			t.Fatalf("shellQuoteForGoOS empty = %q", got)
		}
		if isPowerShellBareWord("") {
			t.Fatal("expected empty PowerShell bare word to be rejected")
		}
		if got := liquibaseRootHint(""); got != "" {
			t.Fatalf("liquibaseRootHint empty = %q", got)
		}
		if got := scorePrepareLiquibase(fileRecord{Ext: ".txt"}); got.Score != 0 {
			t.Fatalf("scorePrepareLiquibase unsupported ext = %+v", got)
		}
		if inbound := inboundEdges([]validatedCandidate{{
			candidateProposal: candidateProposal{
				fileRecord: fileRecord{WorkspaceRoot: workspace, WorkspaceRel: "query.sql", AbsPath: filepath.Join(workspace, "query.sql")},
				Class:      alias.ClassRun,
				Kind:       "psql",
			},
			Closure: nil,
		}}); len(inbound) != 0 {
			t.Fatalf("expected empty inbound edges, got %+v", inbound)
		}
	})

	t.Run("analyze aliases validation failure", func(t *testing.T) {
		workspace := t.TempDir()
		file := filepath.Join(workspace, "queries", "broken.sql")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("mkdir queries: %v", err)
		}
		if err := os.WriteFile(file, []byte("\\i queries/broken.sql\n"), 0o600); err != nil {
			t.Fatalf("write broken.sql: %v", err)
		}

		report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
		if err != nil {
			t.Fatalf("AnalyzeAliases: %v", err)
		}
		if len(report.Findings) != 1 {
			t.Fatalf("expected one finding, got %+v", report)
		}
		if report.Findings[0].Valid || !strings.Contains(report.Findings[0].Error, "recursive include") {
			t.Fatalf("expected recursive include validation failure, got %+v", report.Findings[0])
		}
	})
}
