package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
)

func TestParsePrepareArgsSplitsPsqlArgs(t *testing.T) {
	opts, showHelp, err := parsePrepareArgs([]string{"--image", "img", "--", "-f", "init.sql"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if showHelp {
		t.Fatalf("did not expect help")
	}
	if opts.Image != "img" {
		t.Fatalf("expected image img, got %q", opts.Image)
	}
	if len(opts.PsqlArgs) != 2 || opts.PsqlArgs[0] != "-f" || opts.PsqlArgs[1] != "init.sql" {
		t.Fatalf("unexpected psql args: %#v", opts.PsqlArgs)
	}
}

func TestParsePrepareArgsTreatsUnknownAsPsql(t *testing.T) {
	opts, showHelp, err := parsePrepareArgs([]string{"-f", "init.sql"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if showHelp {
		t.Fatalf("did not expect help")
	}
	if len(opts.PsqlArgs) != 2 || opts.PsqlArgs[0] != "-f" {
		t.Fatalf("unexpected psql args: %#v", opts.PsqlArgs)
	}
}

func TestParsePrepareArgsNoWatch(t *testing.T) {
	opts, showHelp, err := parsePrepareArgs([]string{"--no-watch", "--", "-c", "select 1"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if showHelp {
		t.Fatalf("did not expect help")
	}
	if opts.Watch {
		t.Fatalf("expected watch disabled")
	}
	if !opts.WatchSpecified {
		t.Fatalf("expected watch mode to be marked as explicitly set")
	}
}

func TestParsePrepareArgsWatch(t *testing.T) {
	opts, showHelp, err := parsePrepareArgs([]string{"--watch", "--", "-c", "select 1"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if showHelp {
		t.Fatalf("did not expect help")
	}
	if !opts.Watch {
		t.Fatalf("expected watch enabled")
	}
	if !opts.WatchSpecified {
		t.Fatalf("expected watch mode to be marked as explicitly set")
	}
}

func TestResolvePrepareImagePrecedence(t *testing.T) {
	temp := t.TempDir()
	globalDir := filepath.Join(temp, "config")
	if err := os.MkdirAll(globalDir, 0o700); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte("dbms:\n  image: global\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectDir := filepath.Join(temp, "project", ".sqlrs")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	projectPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(projectPath, []byte("dbms:\n  image: workspace\n"), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg := config.LoadedConfig{
		Paths:             paths.Dirs{ConfigDir: globalDir},
		ProjectConfigPath: projectPath,
	}

	image, source, err := resolvePrepareImage("", cfg)
	if err != nil {
		t.Fatalf("resolve image: %v", err)
	}
	if image != "workspace" || source != "workspace config" {
		t.Fatalf("expected workspace config, got %q (%s)", image, source)
	}

	image, source, err = resolvePrepareImage("cli", cfg)
	if err != nil {
		t.Fatalf("resolve image: %v", err)
	}
	if image != "cli" || source != "command line" {
		t.Fatalf("expected cli override, got %q (%s)", image, source)
	}
}

func TestNormalizePsqlArgsAbsoluteAndStdin(t *testing.T) {
	cwd := t.TempDir()
	args := []string{"-f", "init.sql", "-f", "-"}
	stdin := bytes.NewBufferString("select 1;")

	normalized, stdinValue, err := normalizePsqlArgs(args, cwd, cwd, stdin, nil)
	if err != nil {
		t.Fatalf("normalize args: %v", err)
	}
	if stdinValue == nil || *stdinValue != "select 1;" {
		t.Fatalf("expected stdin content, got %#v", stdinValue)
	}
	expected := filepath.Join(cwd, "init.sql")
	if normalized[0] != "-f" || normalized[1] != expected {
		t.Fatalf("expected absolute file path, got %#v", normalized)
	}
}

func TestFormatImageSource(t *testing.T) {
	out := formatImageSource("img", "global config")
	if !strings.Contains(out, "dbms.image=img") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "source: global config") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestResolveLiquibaseExecPrecedence(t *testing.T) {
	temp := t.TempDir()
	globalDir := filepath.Join(temp, "config")
	if err := os.MkdirAll(globalDir, 0o700); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte("liquibase:\n  exec: global.exe\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectDir := filepath.Join(temp, "project", ".sqlrs")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	projectPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(projectPath, []byte("liquibase:\n  exec: workspace.exe\n"), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg := config.LoadedConfig{
		Paths:             paths.Dirs{ConfigDir: globalDir},
		ProjectConfigPath: projectPath,
	}

	value, err := resolveLiquibaseExec(cfg)
	if err != nil {
		t.Fatalf("resolveLiquibaseExec: %v", err)
	}
	if value != "workspace.exe" {
		t.Fatalf("expected workspace exec, got %q", value)
	}
}

func TestResolveLiquibaseExecModePrecedence(t *testing.T) {
	temp := t.TempDir()
	globalDir := filepath.Join(temp, "config")
	if err := os.MkdirAll(globalDir, 0o700); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte("liquibase:\n  exec_mode: native\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectDir := filepath.Join(temp, "project", ".sqlrs")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	projectPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(projectPath, []byte("liquibase:\n  exec_mode: windows-bat\n"), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg := config.LoadedConfig{
		Paths:             paths.Dirs{ConfigDir: globalDir},
		ProjectConfigPath: projectPath,
	}

	value, err := resolveLiquibaseExecMode(cfg)
	if err != nil {
		t.Fatalf("resolveLiquibaseExecMode: %v", err)
	}
	if value != "windows-bat" {
		t.Fatalf("expected workspace exec_mode, got %q", value)
	}
}

func TestNormalizeWorkDir(t *testing.T) {
	out, err := normalizeWorkDir("", nil)
	if err != nil {
		t.Fatalf("normalizeWorkDir: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}

	out, err = normalizeWorkDir("C:\\work", func(path string) (string, error) {
		return "wsl:" + path, nil
	})
	if err != nil {
		t.Fatalf("normalizeWorkDir: %v", err)
	}
	if out != "wsl:C:\\work" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestResolveLiquibaseEnv(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	if env := resolveLiquibaseEnv(); env != nil {
		t.Fatalf("expected nil env when JAVA_HOME empty, got %+v", env)
	}
	t.Setenv("JAVA_HOME", "C:\\Java")
	env := resolveLiquibaseEnv()
	if env == nil || env["JAVA_HOME"] != "C:\\Java" {
		t.Fatalf("expected JAVA_HOME env, got %+v", env)
	}
	t.Setenv("JAVA_HOME", "\"C:\\Java\\\\\"")
	env = resolveLiquibaseEnv()
	if env == nil || env["JAVA_HOME"] != "C:\\Java" {
		t.Fatalf("expected sanitized JAVA_HOME env, got %+v", env)
	}
}

func TestShouldUseLiquibaseWindowsMode(t *testing.T) {
	if !shouldUseLiquibaseWindowsMode("C:\\Tools\\liquibase.bat", "") {
		t.Fatalf("expected .bat to use windows mode")
	}
	if !shouldUseLiquibaseWindowsMode("C:\\Tools\\liquibase.cmd", "auto") {
		t.Fatalf("expected .cmd to use windows mode")
	}
	if !shouldUseLiquibaseWindowsMode("liquibase", "windows-bat") {
		t.Fatalf("expected windows-bat mode to force windows")
	}
	if shouldUseLiquibaseWindowsMode("C:\\Tools\\liquibase.bat", "native") {
		t.Fatalf("expected native mode to disable windows")
	}
}

func TestNormalizeLiquibaseArgsRewritesPaths(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "examples")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	args := []string{"update", "--changelog-file", "db/changelog.xml", "--searchPath", "db,extras"}
	out, err := normalizeLiquibaseArgs(args, root, cwd, nil)
	if err != nil {
		t.Fatalf("normalizeLiquibaseArgs: %v", err)
	}

	wantChangelog := filepath.Join(cwd, "db", "changelog.xml")
	wantSearch := filepath.Join(cwd, "db") + "," + filepath.Join(cwd, "extras")

	if out[1] != "--changelog-file" || out[2] != wantChangelog {
		t.Fatalf("unexpected changelog rewrite: %v", out)
	}
	if out[3] != "--searchPath" || out[4] != wantSearch {
		t.Fatalf("unexpected searchPath rewrite: %v", out)
	}
}

func TestNormalizeLiquibaseArgsAcceptsSearchPathAlias(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "examples")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	args := []string{"update", "--search-path", "db"}
	out, err := normalizeLiquibaseArgs(args, root, cwd, nil)
	if err != nil {
		t.Fatalf("normalizeLiquibaseArgs: %v", err)
	}
	if out[1] != "--searchPath" {
		t.Fatalf("expected --searchPath, got %v", out)
	}
}

func TestNormalizeLiquibaseArgsUsesConverter(t *testing.T) {
	root := t.TempDir()
	cwd := root
	converted := func(path string) (string, error) {
		return "wsl:" + path, nil
	}

	args := []string{"update", "--defaults-file=conf/liquibase.properties"}
	out, err := normalizeLiquibaseArgs(args, root, cwd, converted)
	if err != nil {
		t.Fatalf("normalizeLiquibaseArgs: %v", err)
	}
	want := "--defaults-file=wsl:" + filepath.Join(cwd, "conf", "liquibase.properties")
	if out[1] != want {
		t.Fatalf("unexpected conversion: %v", out)
	}
}

func TestNormalizeLiquibaseArgsSkipsRemoteRefs(t *testing.T) {
	root := t.TempDir()
	cwd := root

	args := []string{"update", "--changelog-file=classpath:db/changelog.xml", "--searchPath", "classpath:db,https://example.com/db"}
	out, err := normalizeLiquibaseArgs(args, root, cwd, nil)
	if err != nil {
		t.Fatalf("normalizeLiquibaseArgs: %v", err)
	}
	if out[1] != "--changelog-file=classpath:db/changelog.xml" {
		t.Fatalf("unexpected changelog: %v", out)
	}
	if out[2] != "--searchPath" || out[3] != "classpath:db,https://example.com/db" {
		t.Fatalf("unexpected searchPath: %v", out)
	}
}

func TestNormalizeLiquibaseArgsRejectsEmptyValues(t *testing.T) {
	_, err := normalizeLiquibaseArgs([]string{"update", "--defaults-file="}, "", "", nil)
	if err == nil || !strings.Contains(err.Error(), "Missing value for --defaults-file") {
		t.Fatalf("expected defaults-file error, got %v", err)
	}

	_, err = normalizeLiquibaseArgs([]string{"update", "--searchPath", ""}, "", "", nil)
	if err == nil || !strings.Contains(err.Error(), "searchPath is empty") {
		t.Fatalf("expected searchPath error, got %v", err)
	}
}

func TestRelativizeLiquibaseArgs(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "examples")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(cwd, "db", "changelog.xml")
	args := []string{"update", "--changelog-file", path}
	out := relativizeLiquibaseArgs(args, root, cwd)
	if out[1] != "--changelog-file" || out[2] == path {
		t.Fatalf("expected relative changelog, got %v", out)
	}
	if strings.HasPrefix(out[2], "..") {
		t.Fatalf("expected within root, got %v", out)
	}
}
