package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestHandleLiquibasePathFlagSearchPathEquals(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "db")
	if err := writeTempDir(sub); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	args := []string{"--searchPath=" + sub}
	idx := 0
	var normalized []string
	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	handled, err := handleLiquibasePathFlag(args, &idx, "", false, false, &normalized, &mounts, &mountIndex, mounted)
	if err != nil || !handled {
		t.Fatalf("expected handled flag, err=%v", err)
	}
	if len(normalized) != 1 || normalized[0] != "--searchPath="+filepath.Clean(sub) {
		t.Fatalf("unexpected normalized args: %v", normalized)
	}
}

func TestHandleLiquibasePathFlagSearchPathRewriteAlias(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "db")
	if err := writeTempDir(sub); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	args := []string{"--search-path=" + sub}
	idx := 0
	var normalized []string
	var mounts []engineRuntime.Mount
	mountIndex := 0
	mounted := map[string]string{}
	handled, err := handleLiquibasePathFlag(args, &idx, "", false, true, &normalized, &mounts, &mountIndex, mounted)
	if err != nil || !handled {
		t.Fatalf("expected handled search-path, err=%v", err)
	}
	if len(normalized) != 1 || !strings.HasPrefix(normalized[0], "--searchPath=/sqlrs/mnt/path1") {
		t.Fatalf("unexpected normalized args: %v", normalized)
	}
	if len(mounts) != 1 {
		t.Fatalf("expected mount, got %+v", mounts)
	}
}

func TestCollectLiquibasePathsSearchPathAliasAndRemoteDefaults(t *testing.T) {
	cwd := filepath.Join(string(filepath.Separator), "root")
	args := []string{
		"--search-path=/tmp/a,/tmp/b",
		"--defaults-file=classpath:db/lb.properties",
	}
	lockPaths, searchPaths, err := collectLiquibasePaths(args, cwd)
	if err != nil {
		t.Fatalf("collectLiquibasePaths: %v", err)
	}
	if len(lockPaths) != 0 {
		t.Fatalf("expected no lock paths for remote defaults, got %v", lockPaths)
	}
	expectedA := normalizeLiquibasePath("/tmp/a", cwd)
	expectedB := normalizeLiquibasePath("/tmp/b", cwd)
	if len(searchPaths) != 2 || searchPaths[0] != expectedA || searchPaths[1] != expectedB {
		t.Fatalf("unexpected search paths: %v", searchPaths)
	}
}

func TestCollectLiquibasePathsEmptySearchValue(t *testing.T) {
	_, _, err := collectLiquibasePaths([]string{"--searchPath="}, "")
	if err == nil || !strings.Contains(err.Error(), "path is empty") {
		t.Fatalf("expected empty path error, got %v", err)
	}
}

func TestReplaceLiquibaseCommandSkipsFlagValues(t *testing.T) {
	args := []string{"--changelog-file", "file.xml", "update", "--", "extra"}
	out := replaceLiquibaseCommand(args, "updateSQL")
	if !containsArgValue(out, "updateSQL") {
		t.Fatalf("expected replaced command, got %v", out)
	}
	if !containsArgPair(out, "--changelog-file", "file.xml") {
		t.Fatalf("expected changelog args preserved, got %v", out)
	}
	if !containsArgValue(out, "--") {
		t.Fatalf("expected -- preserved, got %v", out)
	}
}

func TestReplaceLiquibaseCommandAppendsWhenMissing(t *testing.T) {
	args := []string{"--defaults-file", "file.properties"}
	out := replaceLiquibaseCommand(args, "updateSQL")
	if out[len(out)-1] != "updateSQL" {
		t.Fatalf("expected appended command, got %v", out)
	}
}

func writeTempDir(path string) error {
	return os.MkdirAll(path, 0o700)
}
