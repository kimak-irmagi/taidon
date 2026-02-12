package prepare

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestNormalizeHostPathBranches(t *testing.T) {
	if _, err := normalizeHostPath(" ", "", "path does not exist"); err == nil {
		t.Fatalf("expected error for empty path")
	}

	out, err := normalizeHostPath("classpath:db/changelog.xml", "", "path does not exist")
	if err != nil || out != "classpath:db/changelog.xml" {
		t.Fatalf("expected remote ref passthrough, got %q err=%v", out, err)
	}

	if _, err := normalizeHostPath("relative.sql", "", "path does not exist"); err == nil || !strings.Contains(err.Error(), "relative path requires working directory") {
		t.Fatalf("expected cwd error, got %v", err)
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, file, "<databaseChangeLog/>")
	out, err = normalizeHostPath(file, "", "path does not exist")
	if err != nil || out != filepath.Clean(file) {
		t.Fatalf("expected cleaned path %q, got %q err=%v", filepath.Clean(file), out, err)
	}

	missing := filepath.Join(dir, "missing.xml")
	_, err = normalizeHostPath(missing, "", "path does not exist")
	var vErr ValidationError
	if !errors.As(err, &vErr) || vErr.Message != "path does not exist" {
		t.Fatalf("expected not found validation error, got %v", err)
	}
}

func TestNormalizeLiquibaseHostPathValueSearchPath(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	value := strings.Join([]string{dirA, dirB}, ",")
	out, err := normalizeLiquibaseHostPathValue("--searchPath", value, "")
	if err != nil {
		t.Fatalf("normalizeLiquibaseHostPathValue: %v", err)
	}
	expected := strings.Join([]string{filepath.Clean(dirA), filepath.Clean(dirB)}, ",")
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}

	if _, err := normalizeLiquibaseHostPathValue("--searchPath", dirA+",", ""); err == nil || !strings.Contains(err.Error(), "searchPath is empty") {
		t.Fatalf("expected searchPath empty error, got %v", err)
	}
}

func TestNormalizeLiquibaseHostPathValueSinglePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, file, "<databaseChangeLog/>")
	out, err := normalizeLiquibaseHostPathValue("--changelog-file", "changelog.xml", dir)
	if err != nil {
		t.Fatalf("normalizeLiquibaseHostPathValue: %v", err)
	}
	if out != file {
		t.Fatalf("expected resolved path %q, got %q", file, out)
	}
}

func TestCollectLiquibasePaths(t *testing.T) {
	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, changelog, "<databaseChangeLog/>")
	defaults := filepath.Join(dir, "lb.properties")
	writeTempFile(t, defaults, "classpath=.")
	searchA := filepath.Join(dir, "db")
	searchB := filepath.Join(dir, "db2")
	if err := os.MkdirAll(searchA, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(searchB, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	args := []string{
		"--changelog-file", changelog,
		"--defaults-file=" + defaults,
		"--searchPath", strings.Join([]string{searchA, searchB}, ","),
	}
	lockPaths, searchPaths, err := collectLiquibasePaths(args, "")
	if err != nil {
		t.Fatalf("collectLiquibasePaths: %v", err)
	}
	if !containsArgValue(lockPaths, filepath.Clean(changelog)) || !containsArgValue(lockPaths, filepath.Clean(defaults)) {
		t.Fatalf("expected lock paths to include changelog/defaults, got %v", lockPaths)
	}
	if len(searchPaths) != 2 {
		t.Fatalf("expected two search paths, got %v", searchPaths)
	}
}

func TestCollectLiquibasePathsMissingValueExtra(t *testing.T) {
	if _, _, err := collectLiquibasePaths([]string{"--changelog-file"}, ""); err == nil {
		t.Fatalf("expected missing value error")
	}
}

func TestHandleLiquibasePathFlagWindowsModeSearchPathAlias(t *testing.T) {
	args := []string{"--search-path", "C:\\work"}
	idx := 0
	var normalized []string
	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	handled, err := handleLiquibasePathFlag(args, &idx, "", true, false, &normalized, &mounts, &mountIndex, mounted)
	if err != nil || !handled {
		t.Fatalf("expected handled flag, err=%v", err)
	}
	if idx != 1 {
		t.Fatalf("expected index to advance, got %d", idx)
	}
	if !containsArgPair(normalized, "--searchPath", "C:\\work") {
		t.Fatalf("expected normalized searchPath, got %v", normalized)
	}
	if len(mounts) != 0 {
		t.Fatalf("expected no mounts in windows mode, got %+v", mounts)
	}
}

func TestHandleLiquibasePathFlagHostModeRequiresCwd(t *testing.T) {
	args := []string{"--changelog-file", "relative.xml"}
	idx := 0
	var normalized []string
	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	handled, err := handleLiquibasePathFlag(args, &idx, "", false, false, &normalized, &mounts, &mountIndex, mounted)
	if !handled {
		t.Fatalf("expected flag to be handled")
	}
	if err == nil || !strings.Contains(err.Error(), "relative path requires working directory") {
		t.Fatalf("expected cwd error, got %v", err)
	}
}

func TestRewriteSinglePathAddsMountAndReuses(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, file, "<databaseChangeLog/>")

	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	mapped, err := rewriteSinglePath(file, "", &mounts, &mountIndex, mounted, "path does not exist")
	if err != nil {
		t.Fatalf("rewriteSinglePath: %v", err)
	}
	if mapped != "/sqlrs/mnt/path1" {
		t.Fatalf("expected mapped path1, got %q", mapped)
	}
	if len(mounts) != 1 || mounts[0].HostPath != file {
		t.Fatalf("expected mount entry, got %+v", mounts)
	}

	mappedAgain, err := rewriteSinglePath(file, "", &mounts, &mountIndex, mounted, "path does not exist")
	if err != nil {
		t.Fatalf("rewriteSinglePath repeat: %v", err)
	}
	if mappedAgain != mapped || len(mounts) != 1 {
		t.Fatalf("expected mapped reuse, got %q mounts=%d", mappedAgain, len(mounts))
	}
}

func TestRewriteSinglePathRequiresCwd(t *testing.T) {
	_, err := rewriteSinglePath("relative.sql", "", &[]engineRuntime.Mount{}, new(int), map[string]string{}, "path does not exist")
	if err == nil || !strings.Contains(err.Error(), "relative path requires working directory") {
		t.Fatalf("expected cwd error, got %v", err)
	}
}

func TestNormalizeLiquibasePathValuesSkipsRemote(t *testing.T) {
	paths, err := normalizeLiquibasePathValues("--searchPath", "classpath:db,/tmp", "")
	if err != nil {
		t.Fatalf("normalizeLiquibasePathValues: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/tmp" {
		t.Fatalf("expected local path only, got %v", paths)
	}
}

func TestNormalizeLiquibasePathValuesRemoteValue(t *testing.T) {
	paths, err := normalizeLiquibasePathValues("--changelog-file", "classpath:db/changelog.xml", "")
	if err != nil {
		t.Fatalf("normalizeLiquibasePathValues: %v", err)
	}
	if paths != nil {
		t.Fatalf("expected nil paths for remote value, got %v", paths)
	}
}

func TestNormalizeLiquibasePathBranches(t *testing.T) {
	if out := normalizeLiquibasePath("C:\\work\\file.xml", ""); out != "C:\\work\\file.xml" {
		t.Fatalf("expected windows path unchanged, got %q", out)
	}
	if out := normalizeLiquibasePath("/tmp/file.xml", ""); out != "/tmp/file.xml" {
		t.Fatalf("expected absolute path cleaned, got %q", out)
	}
	if out := normalizeLiquibasePath("db/file.xml", "/root"); out != filepath.Join("/root", "db", "file.xml") {
		t.Fatalf("expected cwd join, got %q", out)
	}
}

func TestHandleLiquibasePathFlagRewritePaths(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "lb.properties")
	writeTempFile(t, file, "classpath=.")

	args := []string{"--defaults-file=" + file}
	idx := 0
	var normalized []string
	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	handled, err := handleLiquibasePathFlag(args, &idx, "", false, true, &normalized, &mounts, &mountIndex, mounted)
	if err != nil || !handled {
		t.Fatalf("expected handled defaults file, err=%v", err)
	}
	if len(mounts) != 1 {
		t.Fatalf("expected one mount, got %+v", mounts)
	}
	if len(normalized) != 1 || !strings.HasPrefix(normalized[0], "--defaults-file=/sqlrs/mnt/path1") {
		t.Fatalf("unexpected normalized args: %v", normalized)
	}
}
