package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
)

func TestPrepareResultLiquibaseDetachedCompositeRun(t *testing.T) {
	prevRunPrepare := runPrepareFn
	runPrepareFn = func(context.Context, cli.PrepareOptions) (client.PrepareJobResult, error) {
		return client.PrepareJobResult{}, &cli.PrepareDetachedError{JobID: "job-lb-detached"}
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	root := t.TempDir()
	var out bytes.Buffer
	_, handled, err := prepareResultLiquibase(
		stdoutAndErr{stdout: &out, stderr: io.Discard},
		cli.PrepareOptions{CompositeRun: true},
		config.LoadedConfig{},
		root,
		root,
		[]string{"--image", "image", "--", "update", "--changelog-file", "changelog.xml"},
	)
	if err != nil {
		t.Fatalf("prepareResultLiquibase: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	text := out.String()
	if !strings.Contains(text, "JOB_ID=job-lb-detached") || !strings.Contains(text, "RUN_SKIPPED=prepare_detached") {
		t.Fatalf("unexpected output: %q", text)
	}
}

func TestBuildPathConverterWithoutWSLDistro(t *testing.T) {
	if converter := buildPathConverter(cli.PrepareOptions{}); converter != nil {
		t.Fatalf("expected nil converter without WSL distro")
	}
}

func TestNormalizeFilePathResolvesExistingPathBeforeBoundaryCheck(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "query.sql")
	if err := os.WriteFile(filePath, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	normalized, useStdin, err := normalizeFilePath("query.sql", root, root, nil)
	if err != nil {
		t.Fatalf("normalizeFilePath: %v", err)
	}
	if useStdin || normalized != filePath {
		t.Fatalf("unexpected normalized path: %q useStdin=%v", normalized, useStdin)
	}
}

func TestNormalizeFilePathPreservesSymlinkPathForExistingFile(t *testing.T) {
	temp := t.TempDir()
	realRoot := filepath.Join(temp, "real")
	linkRoot := filepath.Join(temp, "link")
	if err := os.MkdirAll(realRoot, 0o700); err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "query.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	normalized, useStdin, err := normalizeFilePath("query.sql", linkRoot, linkRoot, nil)
	if err != nil {
		t.Fatalf("normalizeFilePath: %v", err)
	}
	want := filepath.Join(linkRoot, "query.sql")
	if useStdin || normalized != want {
		t.Fatalf("unexpected normalized path: %q want %q useStdin=%v", normalized, want, useStdin)
	}
}

func TestNormalizeFilePathAllowsMissingLeafWithinSymlinkedWorkspace(t *testing.T) {
	temp := t.TempDir()
	realRoot := filepath.Join(temp, "real")
	linkRoot := filepath.Join(temp, "link")
	if err := os.MkdirAll(realRoot, 0o700); err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	normalized, useStdin, err := normalizeFilePath("missing.sql", linkRoot, linkRoot, nil)
	if err != nil {
		t.Fatalf("normalizeFilePath: %v", err)
	}
	want := filepath.Join(linkRoot, "missing.sql")
	if useStdin || normalized != want {
		t.Fatalf("unexpected normalized path: %q want %q useStdin=%v", normalized, want, useStdin)
	}
}

func TestNormalizeFilePathPreservesWorkspaceSpellingForCanonicalizedCWD(t *testing.T) {
	temp := t.TempDir()
	realRoot := filepath.Join(temp, "real")
	linkRoot := filepath.Join(temp, "link")
	realCWD := filepath.Join(realRoot, "examples", "chinook")
	if err := os.MkdirAll(realCWD, 0o700); err != nil {
		t.Fatalf("mkdir real cwd: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realCWD, "prepare.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	normalized, useStdin, err := normalizeFilePath("prepare.sql", linkRoot, realCWD, nil)
	if err != nil {
		t.Fatalf("normalizeFilePath: %v", err)
	}
	want := filepath.Join(linkRoot, "examples", "chinook", "prepare.sql")
	if useStdin || normalized != want {
		t.Fatalf("unexpected normalized path: %q want %q useStdin=%v", normalized, want, useStdin)
	}
}
