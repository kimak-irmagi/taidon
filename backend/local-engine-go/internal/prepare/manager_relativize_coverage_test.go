package prepare

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestRelativizeLiquibaseHostPathCoverage(t *testing.T) {
	base, insidePath, outsidePath := relativizePathsForCurrentOS(t)
	insideExpected := filepath.Join("db", "changelog.xml")
	if got := relativizeLiquibaseHostPath(insidePath, base); got != insideExpected {
		t.Fatalf("expected inside path %q, got %q", insideExpected, got)
	}

	if got := relativizeLiquibaseHostPath(base, base); got != base {
		t.Fatalf("expected base path to stay absolute, got %q", got)
	}

	if got := relativizeLiquibaseHostPath(outsidePath, base); got != outsidePath {
		t.Fatalf("expected outside path to stay absolute, got %q", got)
	}

	blank := "   "
	if got := relativizeLiquibaseHostPath(blank, base); got != blank {
		t.Fatalf("expected blank value to stay unchanged, got %q", got)
	}

	remote := "s3://bucket/changelog.xml"
	if got := relativizeLiquibaseHostPath(remote, base); got != remote {
		t.Fatalf("expected remote ref to stay unchanged, got %q", got)
	}

	relative := "db/changelog.xml"
	if got := relativizeLiquibaseHostPath(relative, base); got != relative {
		t.Fatalf("expected relative path to stay unchanged, got %q", got)
	}
}

func TestRelativizeLiquibaseHostFileArgsCoverage(t *testing.T) {
	base, changelogPath, outsidePath := relativizePathsForCurrentOS(t)
	defaultsPath := filepath.Join(filepath.Dir(changelogPath), "liquibase.properties")

	args := []string{
		"update",
		"--changelog-file", changelogPath,
		"--defaults-file=" + defaultsPath,
		"--changelog-file=" + outsidePath,
		"--defaults-file",
	}
	got := relativizeLiquibaseHostFileArgs(args, base)
	want := []string{
		"update",
		"--changelog-file", filepath.Join("db", "changelog.xml"),
		"--defaults-file=" + filepath.Join("db", "liquibase.properties"),
		"--changelog-file=" + outsidePath,
		"--defaults-file",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized args:\nwant: %#v\ngot:  %#v", want, got)
	}

	unchanged := relativizeLiquibaseHostFileArgs(args, "   ")
	if !reflect.DeepEqual(unchanged, args) {
		t.Fatalf("expected args unchanged for empty base:\nwant: %#v\ngot:  %#v", args, unchanged)
	}
}

func relativizePathsForCurrentOS(t *testing.T) (base string, inside string, outside string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		base = `\\workspace\project`
		inside = `\\workspace\project\db\changelog.xml`
		outside = `\\workspace\other\outside.xml`
		return base, inside, outside
	}

	base = t.TempDir()
	dbDir := filepath.Join(base, "db")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	inside = filepath.Join(dbDir, "changelog.xml")
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatalf("write inside file: %v", err)
	}
	outsideRoot := t.TempDir()
	outside = filepath.Join(outsideRoot, "outside.xml")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	return base, inside, outside
}
