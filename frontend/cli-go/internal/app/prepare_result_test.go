package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
)

func TestResolvePrepareImageFromProjectConfig(t *testing.T) {
	root := t.TempDir()
	projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(projectConfig, []byte("dbms:\n  image: postgres:17\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := config.LoadedConfig{
		ProjectConfigPath: projectConfig,
		Paths:             paths.Dirs{ConfigDir: t.TempDir()},
	}
	image, source, err := resolvePrepareImage("", cfg)
	if err != nil {
		t.Fatalf("resolvePrepareImage: %v", err)
	}
	if image != "postgres:17" || source != "workspace config" {
		t.Fatalf("unexpected image/source: %q %q", image, source)
	}
}

func TestResolvePrepareImageFromCLI(t *testing.T) {
	cfg := config.LoadedConfig{
		ProjectConfigPath: "",
		Paths:             paths.Dirs{ConfigDir: t.TempDir()},
	}
	image, source, err := resolvePrepareImage("postgres:14", cfg)
	if err != nil {
		t.Fatalf("resolvePrepareImage: %v", err)
	}
	if image != "postgres:14" || source != "command line" {
		t.Fatalf("unexpected image/source: %q %q", image, source)
	}
}

func TestResolvePrepareImageFromGlobalConfig(t *testing.T) {
	root := t.TempDir()
	globalConfig := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(globalConfig, []byte("dbms:\n  image: postgres:15\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := config.LoadedConfig{
		ProjectConfigPath: "",
		Paths:             paths.Dirs{ConfigDir: root},
	}
	image, source, err := resolvePrepareImage("", cfg)
	if err != nil {
		t.Fatalf("resolvePrepareImage: %v", err)
	}
	if image != "postgres:15" || source != "global config" {
		t.Fatalf("unexpected image/source: %q %q", image, source)
	}
}

func TestFormatImageSourceEmptyInPrepareResult(t *testing.T) {
	if got := formatImageSource("", "workspace config"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := formatImageSource("postgres:17", ""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestNormalizePsqlArgsMissingFileValue(t *testing.T) {
	_, _, err := normalizePsqlArgs([]string{"-f"}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestNormalizePsqlArgsFileForms(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(filePath, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	args, _, err := normalizePsqlArgs([]string{"--file=query.sql", "-fquery.sql"}, dir, dir, strings.NewReader(""))
	if err != nil {
		t.Fatalf("normalizePsqlArgs: %v", err)
	}
	if len(args) != 2 {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestNormalizeFilePathOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "project")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, _, err := normalizeFilePath(filepath.Join(root, "..", "outside.sql"), root, cwd)
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}
