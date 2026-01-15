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

	normalized, stdinValue, err := normalizePsqlArgs(args, cwd, stdin)
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
