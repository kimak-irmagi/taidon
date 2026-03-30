package alias

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli/runkind"
	"gopkg.in/yaml.v3"
)

func TestCreateControlHelpers(t *testing.T) {
	t.Run("validateCreateKind", func(t *testing.T) {
		cases := []struct {
			name    string
			class   Class
			kind    string
			wantErr string
		}{
			{name: "prepare psql", class: ClassPrepare, kind: "psql"},
			{name: "prepare lb", class: ClassPrepare, kind: "lb"},
			{name: "prepare invalid", class: ClassPrepare, kind: "pgbench", wantErr: "unknown prepare alias kind"},
			{name: "run psql", class: ClassRun, kind: runkind.KindPsql},
			{name: "run pgbench", class: ClassRun, kind: runkind.KindPgbench},
			{name: "run invalid", class: ClassRun, kind: "weird", wantErr: "unknown run alias kind"},
			{name: "missing class", class: "", kind: "psql", wantErr: "alias class is required"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := validateCreateKind(tc.class, tc.kind)
				if tc.wantErr == "" {
					if err != nil {
						t.Fatalf("validateCreateKind(%q, %q) = %v", tc.class, tc.kind, err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("validateCreateKind(%q, %q) error = %v, want %q", tc.class, tc.kind, err, tc.wantErr)
				}
			})
		}
	})

	t.Run("splitCreateArgs", func(t *testing.T) {
		cases := []struct {
			name        string
			class       Class
			args        []string
			wantControl []string
			wantPayload []string
			wantErr     string
		}{
			{
				name:        "empty",
				class:       ClassPrepare,
				args:        nil,
				wantControl: nil,
				wantPayload: nil,
			},
			{
				name:        "prepare with control and payload",
				class:       ClassPrepare,
				args:        []string{"--image=postgres:17", "--", "-c", "select 1"},
				wantControl: []string{"--image=postgres:17"},
				wantPayload: []string{"-c", "select 1"},
			},
			{
				name:        "prepare with split control value",
				class:       ClassPrepare,
				args:        []string{"--image", "postgres:17", "-c", "select 1"},
				wantControl: []string{"--image", "postgres:17"},
				wantPayload: []string{"-c", "select 1"},
			},
			{
				name:    "prepare missing control value",
				class:   ClassPrepare,
				args:    []string{"--image"},
				wantErr: "Missing value for --image",
			},
			{
				name:    "payload control args rejected",
				class:   ClassPrepare,
				args:    []string{"--image=postgres:17", "--", "--watch"},
				wantErr: "wrapped command flags must appear before tool args",
			},
			{
				name:        "run payload starts immediately",
				class:       ClassRun,
				args:        []string{"-c", "select 1"},
				wantControl: []string{},
				wantPayload: []string{"-c", "select 1"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				control, payload, err := splitCreateArgs(tc.class, tc.args)
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("splitCreateArgs(%q, %v) error = %v, want %q", tc.class, tc.args, err, tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("splitCreateArgs(%q, %v) = %v", tc.class, tc.args, err)
				}
				if !reflect.DeepEqual(control, tc.wantControl) || !reflect.DeepEqual(payload, tc.wantPayload) {
					t.Fatalf("splitCreateArgs(%q, %v) = %v, %v; want %v, %v", tc.class, tc.args, control, payload, tc.wantControl, tc.wantPayload)
				}
			})
		}
	})

	t.Run("parseCreateControlArgs", func(t *testing.T) {
		cases := []struct {
			name    string
			class   Class
			args    []string
			want    string
			wantErr string
		}{
			{name: "prepare image value", class: ClassPrepare, args: []string{"--image", " postgres:17 "}, want: "postgres:17"},
			{name: "prepare image equals", class: ClassPrepare, args: []string{"--image= postgres:17 "}, want: "postgres:17"},
			{name: "prepare unknown option", class: ClassPrepare, args: []string{"--watch"}, wantErr: "unknown prepare alias option"},
			{name: "prepare missing value", class: ClassPrepare, args: []string{"--image"}, wantErr: "Missing value for --image"},
			{name: "run control option", class: ClassRun, args: []string{"--image=postgres:17"}, wantErr: "unknown run alias option"},
			{name: "missing class", class: "", args: []string{"--image=postgres:17"}, wantErr: "alias class is required"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := parseCreateControlArgs(tc.class, tc.args)
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("parseCreateControlArgs(%q, %v) error = %v, want %q", tc.class, tc.args, err, tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("parseCreateControlArgs(%q, %v) = %v", tc.class, tc.args, err)
				}
				if got != tc.want {
					t.Fatalf("parseCreateControlArgs(%q, %v) = %q, want %q", tc.class, tc.args, got, tc.want)
				}
			})
		}
	})

	t.Run("controlArgPredicates", func(t *testing.T) {
		if !isCreateControlArg(ClassPrepare, "--image") {
			t.Fatalf("expected --image to be a prepare control arg")
		}
		if !isCreateControlArg(ClassPrepare, "--image=postgres:17") {
			t.Fatalf("expected --image= to be a prepare control arg")
		}
		if isCreateControlArg(ClassRun, "--image") {
			t.Fatalf("did not expect run aliases to accept control args")
		}
		if !createControlArgTakesValue("--image") {
			t.Fatalf("expected --image to take a value")
		}
		if createControlArgTakesValue("--image=postgres:17") {
			t.Fatalf("did not expect --image= to take a separate value")
		}
	})

	t.Run("rejectCreateControlArgsInPayload", func(t *testing.T) {
		for _, class := range []Class{ClassPrepare, ClassRun} {
			t.Run(string(class), func(t *testing.T) {
				if err := rejectCreateControlArgsInPayload(class, []string{"--verbose"}); err != nil {
					t.Fatalf("expected clean payload for %q, got %v", class, err)
				}
				for _, payload := range [][]string{
					{"--image"},
					{"--image=postgres:17"},
					{"--watch"},
					{"--no-watch"},
					{"--instance"},
					{"--instance=demo"},
				} {
					if err := rejectCreateControlArgsInPayload(class, payload); err == nil {
						t.Fatalf("expected payload %v to be rejected for %q", payload, class)
					}
				}
			})
		}
	})
}

func TestCreateRelativizeHelpers(t *testing.T) {
	workspace := t.TempDir()
	cwd := workspace
	aliasDir := filepath.Join(workspace, "db")
	mkdirAll(t, aliasDir)
	mkdirAll(t, filepath.Join(aliasDir, "migrations"))
	mkdirAll(t, filepath.Join(aliasDir, "shared"))
	writePlainFile(t, aliasDir, "changelog.xml", "<xml/>\n")
	writePlainFile(t, aliasDir, "defaults.properties", "x=y\n")
	writePlainFile(t, aliasDir, "seed.sql", "select 1;\n")
	writePlainFile(t, aliasDir, "bench.sql", "select 1;\n")

	t.Run("relativizeCreatePathArg", func(t *testing.T) {
		cases := []struct {
			name     string
			value    string
			aliasDir string
			want     string
			wantErr  string
		}{
			{name: "empty", value: "", aliasDir: aliasDir, want: ""},
			{name: "stdin", value: "-", aliasDir: aliasDir, want: "-"},
			{name: "local path", value: "db/seed.sql", aliasDir: aliasDir, want: "seed.sql"},
			{name: "outside workspace", value: filepath.Join("..", "outside.sql"), aliasDir: aliasDir, wantErr: "within workspace root"},
			{name: "fallback to absolute when alias dir is on a different volume", value: "db/seed.sql", aliasDir: `Z:\alias`, want: filepath.ToSlash(filepath.Join(workspace, "db", "seed.sql"))},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := relativizeCreatePathArg(tc.value, workspace, cwd, tc.aliasDir)
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("relativizeCreatePathArg(%q) error = %v, want %q", tc.value, err, tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("relativizeCreatePathArg(%q) = %v", tc.value, err)
				}
				if got != tc.want {
					t.Fatalf("relativizeCreatePathArg(%q) = %q, want %q", tc.value, got, tc.want)
				}
			})
		}
	})

	t.Run("relativizePsqlCreateArgs", func(t *testing.T) {
		args := []string{"-f", "db/seed.sql", "--file", "db/seed.sql", "--file=db/seed.sql", "-fdb/seed.sql", "--verbose"}
		got, err := relativizePsqlCreateArgs(args, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizePsqlCreateArgs: %v", err)
		}
		want := []string{"-f", "seed.sql", "--file", "seed.sql", "--file=seed.sql", "-fseed.sql", "--verbose"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("relativizePsqlCreateArgs = %v, want %v", got, want)
		}
		got, err = relativizePsqlCreateArgs([]string{"--file"}, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizePsqlCreateArgs trailing flag: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"--file"}) {
			t.Fatalf("relativizePsqlCreateArgs trailing flag = %v", got)
		}
	})

	t.Run("relativizePgbenchCreateArgs", func(t *testing.T) {
		args := []string{"-f", "db/bench.sql", "--file", "db/bench.sql", "--file=db/bench.sql", "-fdb/bench.sql", "--verbose"}
		got, err := relativizePgbenchCreateArgs(args, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizePgbenchCreateArgs: %v", err)
		}
		want := []string{"-f", "bench.sql", "--file", "bench.sql", "--file=bench.sql", "-fbench.sql", "--verbose"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("relativizePgbenchCreateArgs = %v, want %v", got, want)
		}
		got, err = relativizePgbenchCreateArgs([]string{"--file"}, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizePgbenchCreateArgs trailing flag: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"--file"}) {
			t.Fatalf("relativizePgbenchCreateArgs trailing flag = %v", got)
		}
	})

	t.Run("relativizeLiquibaseCreateArgs", func(t *testing.T) {
		args := []string{
			"--changelog-file", "db/changelog.xml",
			"--defaults-file=db/defaults.properties",
			"--searchPath", "db/migrations,classpath:db,https://example.com/db",
			"--search-path=db/shared",
			"--verbose",
		}
		got, err := relativizeLiquibaseCreateArgs(args, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizeLiquibaseCreateArgs: %v", err)
		}
		want := []string{
			"--changelog-file", "changelog.xml",
			"--defaults-file=defaults.properties",
			"--searchPath", "migrations,classpath:db,https://example.com/db",
			"--search-path=shared",
			"--verbose",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("relativizeLiquibaseCreateArgs = %v, want %v", got, want)
		}
		got, err = relativizeLiquibaseCreateArgs([]string{"--searchPath"}, workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizeLiquibaseCreateArgs trailing flag: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"--searchPath"}) {
			t.Fatalf("relativizeLiquibaseCreateArgs trailing flag = %v", got)
		}
	})

	t.Run("relativizeLiquibaseValue", func(t *testing.T) {
		cases := []struct {
			value   string
			want    string
			wantErr bool
		}{
			{value: "", want: ""},
			{value: "classpath:db", want: "classpath:db"},
			{value: "https://example.com/db", want: "https://example.com/db"},
			{value: "db/changelog.xml", want: "changelog.xml"},
		}
		for _, tc := range cases {
			got, err := relativizeCreateLiquibaseValue(tc.value, workspace, cwd, aliasDir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.value)
				}
				continue
			}
			if err != nil {
				t.Fatalf("relativizeCreateLiquibaseValue(%q) = %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("relativizeCreateLiquibaseValue(%q) = %q, want %q", tc.value, got, tc.want)
			}
		}
	})

	t.Run("relativizeLiquibaseSearchPath", func(t *testing.T) {
		got, err := relativizeCreateLiquibaseSearchPath("db/migrations,,classpath:db,https://example.com/db", workspace, cwd, aliasDir)
		if err != nil {
			t.Fatalf("relativizeCreateLiquibaseSearchPath: %v", err)
		}
		if got != "migrations,,classpath:db,https://example.com/db" {
			t.Fatalf("relativizeCreateLiquibaseSearchPath = %q", got)
		}
	})

	t.Run("relativizePgbenchValue", func(t *testing.T) {
		cases := []struct {
			value string
			want  string
		}{
			{value: "", want: ""},
			{value: "-", want: "-"},
			{value: "db/bench.sql@10", want: "bench.sql@10"},
		}
		for _, tc := range cases {
			got, err := relativizeCreatePgbenchValue(tc.value, workspace, cwd, aliasDir)
			if err != nil {
				t.Fatalf("relativizeCreatePgbenchValue(%q) = %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("relativizeCreatePgbenchValue(%q) = %q, want %q", tc.value, got, tc.want)
			}
		}
	})
}

func TestCreateEndToEndCoverage(t *testing.T) {
	t.Run("prepare liquibase alias", func(t *testing.T) {
		workspace := t.TempDir()
		cwd := workspace
		aliasDir := filepath.Join(workspace, "db")
		mkdirAll(t, aliasDir)
		writePlainFile(t, aliasDir, "changelog.xml", "<xml/>\n")
		writePlainFile(t, aliasDir, "defaults.properties", "x=y\n")
		mkdirAll(t, filepath.Join(aliasDir, "migrations"))
		mkdirAll(t, filepath.Join(aliasDir, "shared"))

		result, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           cwd,
			Ref:           "db/chinook",
			Class:         ClassPrepare,
			Kind:          "lb",
			Args: []string{
				"--image=postgres:17",
				"--",
				"update",
				"--changelog-file",
				"db/changelog.xml",
				"--defaults-file=db/defaults.properties",
				"--searchPath",
				"db/migrations,classpath:db,https://example.com/db",
				"--search-path=db/shared",
			},
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if result.Type != ClassPrepare || result.Kind != "lb" || result.Image != "postgres:17" {
			t.Fatalf("unexpected result: %+v", result)
		}
		if result.File != filepath.ToSlash(filepath.Join("db", "chinook.prep.s9s.yaml")) {
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
		wantArgs := []string{
			"update",
			"--changelog-file", "changelog.xml",
			"--defaults-file=defaults.properties",
			"--searchPath", "migrations,classpath:db,https://example.com/db",
			"--search-path=shared",
		}
		if rendered.Kind != "lb" || rendered.Image != "postgres:17" || !reflect.DeepEqual(rendered.Args, wantArgs) {
			t.Fatalf("unexpected payload: %+v", rendered)
		}
	})

	t.Run("run psql alias", func(t *testing.T) {
		workspace := t.TempDir()
		result, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "db/exec",
			Class:         ClassRun,
			Kind:          "psql",
			Args:          []string{"--", "-c", "select 1"},
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if result.Type != ClassRun || result.Kind != "psql" || result.Image != "" {
			t.Fatalf("unexpected result: %+v", result)
		}
		if result.File != filepath.ToSlash(filepath.Join("db", "exec.run.s9s.yaml")) {
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
		if rendered.Kind != "psql" || rendered.Image != "" || !reflect.DeepEqual(rendered.Args, []string{"-c", "select 1"}) {
			t.Fatalf("unexpected payload: %+v", rendered)
		}
	})

	t.Run("reject missing payload args", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "demo",
			Class:         ClassPrepare,
			Kind:          "psql",
		})
		if err == nil || !strings.Contains(err.Error(), "wrapped command args are required") {
			t.Fatalf("expected payload error, got %v", err)
		}
	})

	t.Run("reject invalid kind", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "demo",
			Class:         ClassPrepare,
			Kind:          "weird",
			Args:          []string{"--", "-c", "select 1"},
		})
		if err == nil || !strings.Contains(err.Error(), "unknown prepare alias kind") {
			t.Fatalf("expected kind validation error, got %v", err)
		}
	})

	t.Run("reject missing control value", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "demo",
			Class:         ClassPrepare,
			Kind:          "psql",
			Args:          []string{"--image"},
		})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --image") {
			t.Fatalf("expected missing control value error, got %v", err)
		}
	})

	t.Run("reject control flags in payload", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "demo",
			Class:         ClassPrepare,
			Kind:          "psql",
			Args:          []string{"--", "--watch"},
		})
		if err == nil || !strings.Contains(err.Error(), "wrapped command flags must appear before tool args") {
			t.Fatalf("expected payload guard error, got %v", err)
		}
	})

	t.Run("reject rewrite error", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "demo",
			Class:         ClassPrepare,
			Kind:          "psql",
			Args:          []string{"--", "-f", filepath.Join("..", "outside.sql")},
		})
		if err == nil || !strings.Contains(err.Error(), "within workspace root") {
			t.Fatalf("expected workspace-bound rewrite error, got %v", err)
		}
	})

	t.Run("reject mkdir conflict", func(t *testing.T) {
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, "aliases"), []byte("file"), 0o600); err != nil {
			t.Fatalf("write blocker: %v", err)
		}
		_, err := Create(CreateOptions{
			WorkspaceRoot: workspace,
			CWD:           workspace,
			Ref:           "aliases/chinook",
			Class:         ClassPrepare,
			Kind:          "psql",
			Args:          []string{"--", "-c", "select 1"},
		})
		if err == nil {
			t.Fatalf("expected mkdir error")
		}
	})
}

func TestResolveCreateTargetCoverage(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)

	cases := []struct {
		name     string
		opts     CreateOptions
		wantErr  string
		wantRef  string
		wantFile string
	}{
		{
			name:    "missing workspace root",
			opts:    CreateOptions{Ref: "demo", Class: ClassPrepare},
			wantErr: "workspace root is required to create aliases",
		},
		{
			name:    "missing ref",
			opts:    CreateOptions{WorkspaceRoot: workspace, CWD: cwd, Class: ClassPrepare},
			wantErr: "alias ref is required",
		},
		{
			name:    "suffix ref",
			opts:    CreateOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "demo.prep.s9s.yaml", Class: ClassPrepare},
			wantErr: "logical stem",
		},
		{
			name:    "dot ref",
			opts:    CreateOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "demo.", Class: ClassPrepare},
			wantErr: "logical stem",
		},
		{
			name:    "missing class",
			opts:    CreateOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "demo"},
			wantErr: "alias class is required",
		},
		{
			name:    "outside workspace",
			opts:    CreateOptions{WorkspaceRoot: workspace, CWD: filepath.Join(filepath.Dir(workspace), "outside"), Ref: "demo", Class: ClassPrepare},
			wantErr: "within workspace root",
		},
		{
			name:     "prepare defaults cwd to workspace",
			opts:     CreateOptions{WorkspaceRoot: workspace, Ref: "demo", Class: ClassPrepare},
			wantRef:  "demo",
			wantFile: filepath.ToSlash("demo.prep.s9s.yaml"),
		},
		{
			name:     "run target",
			opts:     CreateOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "nested/demo", Class: ClassRun},
			wantRef:  "nested/demo",
			wantFile: filepath.ToSlash(filepath.Join("examples", "nested", "demo.run.s9s.yaml")),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveCreateTarget(tc.opts)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ResolveCreateTarget(%+v) error = %v, want %q", tc.opts, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveCreateTarget(%+v) = %v", tc.opts, err)
			}
			if got.Ref != tc.wantRef || got.File != tc.wantFile {
				t.Fatalf("ResolveCreateTarget(%+v) = %+v, want ref %q file %q", tc.opts, got, tc.wantRef, tc.wantFile)
			}
		})
	}
}

func TestAliasPathAndScanCoverage(t *testing.T) {
	t.Run("portable path helpers", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, "base")
		target := filepath.Join(base, "sub", "file.txt")
		mkdirAll(t, filepath.Dir(target))

		got, err := portableRelativePath(base, target)
		if err != nil {
			t.Fatalf("portableRelativePath: %v", err)
		}
		if filepath.ToSlash(got) != "sub/file.txt" {
			t.Fatalf("portableRelativePath = %q", got)
		}
		if _, err := portableRelativePath(`C:\base`, `D:\base\file.txt`); err == nil {
			t.Fatalf("expected portableRelativePath to reject different roots")
		}

		got, err = portableWindowsRelativePath(`C:\base`, `C:\base\sub\file.txt`, "C:", "C:")
		if err != nil {
			t.Fatalf("portableWindowsRelativePath: %v", err)
		}
		if got != "sub/file.txt" {
			t.Fatalf("portableWindowsRelativePath = %q", got)
		}
		if _, err := portableWindowsRelativePath(`C:\base`, `D:\base\file.txt`, "C:", "D:"); err == nil {
			t.Fatalf("expected different-root error")
		}

		if windowsVolumeName(`C:\base`) != "C:" {
			t.Fatalf("expected drive letter volume")
		}
		if windowsVolumeName(`base`) != "" {
			t.Fatalf("expected empty volume name")
		}

		if normalizeWindowsLikePath("") != "/" {
			t.Fatalf("expected empty path to normalize to root slash")
		}
		if normalizeWindowsLikePath(`foo\bar`) != "/foo/bar" {
			t.Fatalf("unexpected normalized path")
		}
		if normalizeWindowsLikePath(`/foo`) != "/foo" {
			t.Fatalf("unexpected normalized absolute path")
		}

		if got := slashRelativePath("/A/B", "/a/b"); got != "." {
			t.Fatalf("slashRelativePath(equal) = %q", got)
		}
		if got := slashRelativePath("/A/B", "/a/c"); got != "../c" {
			t.Fatalf("slashRelativePath = %q", got)
		}

		if got := splitSlashPath("/"); got != nil {
			t.Fatalf("splitSlashPath(root) = %v", got)
		}
		if got := splitSlashPath("/a/b"); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("splitSlashPath = %v", got)
		}

		if portablePathBase(`C:\dir\file.txt\`) != "file.txt" {
			t.Fatalf("portablePathBase failed for windows path")
		}
		if portablePathBase("/dir/file.txt/") != "file.txt" {
			t.Fatalf("portablePathBase failed for slash path")
		}
		if portablePathBase("file.txt") != "file.txt" {
			t.Fatalf("portablePathBase failed for bare filename")
		}
		if portablePathBase("") != "" {
			t.Fatalf("portablePathBase failed for empty path")
		}
	})

	t.Run("path boundary helpers", func(t *testing.T) {
		root := t.TempDir()
		if !isWithin(root, root) {
			t.Fatalf("expected boundary path to be considered within itself")
		}
		if isWithin(root, filepath.Join(filepath.Dir(root), "outside")) {
			t.Fatalf("expected outside path to be rejected")
		}

		missingLeaf := filepath.Join(root, "missing", "leaf")
		got := canonicalizeBoundaryPath(missingLeaf)
		if got != filepath.Clean(missingLeaf) {
			t.Fatalf("canonicalizeBoundaryPath = %q, want %q", got, filepath.Clean(missingLeaf))
		}
	})

	t.Run("check and scan helpers", func(t *testing.T) {
		if _, err := CheckScan(ScanOptions{}); err == nil || !strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("expected CheckScan workspace error, got %v", err)
		}

		if err := walkDirectory(filepath.Join(t.TempDir(), "missing"), 0, DepthRecursive, func(string) error { return nil }); err == nil {
			t.Fatalf("expected walkDirectory to fail for missing directory")
		}

		if got := inventoryReadError("", os.ErrInvalid); !reflect.DeepEqual(got, os.ErrInvalid) {
			t.Fatalf("inventoryReadError default = %v", got)
		}

		workspace := t.TempDir()
		cwd := filepath.Join(workspace, "examples")
		mkdirAll(t, cwd)
		if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "missing"}); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected missing stem error, got %v", err)
		}

		aliasPath := writeAliasFile(t, workspace, filepath.Join("scripts", "demo.alias.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")
		absPath, err := filepath.Abs(aliasPath)
		if err != nil {
			t.Fatalf("filepath.Abs: %v", err)
		}
		target, err := ResolveTarget(ResolveOptions{
			WorkspaceRoot: workspace,
			CWD:           `Z:\cwd`,
			Ref:           filepath.ToSlash(absPath) + ".",
			Class:         ClassRun,
		})
		if err != nil {
			t.Fatalf("ResolveTarget exact fallback: %v", err)
		}
		if target.Class != ClassRun || target.Ref != "demo.alias.yaml" {
			t.Fatalf("unexpected exact target fallback: %+v", target)
		}
	})
}
