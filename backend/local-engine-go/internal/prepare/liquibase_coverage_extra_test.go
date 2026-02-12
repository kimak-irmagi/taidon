package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestHandleLiquibasePathFlagWindowsMode(t *testing.T) {
	args := []string{"--changelog-file", "C:\\work\\changelog.xml"}
	idx := 0
	normalized := []string{}
	mounts := []engineRuntime.Mount{}
	mountIndex := 0
	mounted := map[string]string{}

	handled, err := handleLiquibasePathFlag(args, &idx, "C:\\work", true, true, &normalized, &mounts, &mountIndex, mounted)
	if err != nil {
		t.Fatalf("handleLiquibasePathFlag: %v", err)
	}
	if !handled || len(normalized) != 2 || normalized[0] != "--changelog-file" {
		t.Fatalf("unexpected normalized: %+v", normalized)
	}
	if len(mounts) != 0 {
		t.Fatalf("expected no mounts, got %+v", mounts)
	}
}

func TestHandleLiquibasePathFlagMissingValue(t *testing.T) {
	args := []string{"--defaults-file"}
	idx := 0
	normalized := []string{}
	mounts := []engineRuntime.Mount{}
	mountIndex := 0
	mounted := map[string]string{}

	handled, err := handleLiquibasePathFlag(args, &idx, "", false, true, &normalized, &mounts, &mountIndex, mounted)
	if err == nil || !handled {
		t.Fatalf("expected missing value error")
	}
}

func TestCollectLiquibasePathsMissingValue(t *testing.T) {
	_, _, err := collectLiquibasePaths([]string{"--searchPath"}, "")
	if err == nil || !strings.Contains(err.Error(), "missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestCollectLiquibasePathsSearchPath(t *testing.T) {
	lockPaths, searchPaths, err := collectLiquibasePaths([]string{"--searchPath", "classpath:db,/tmp/path"}, "")
	if err != nil {
		t.Fatalf("collectLiquibasePaths: %v", err)
	}
	if len(lockPaths) != 0 {
		t.Fatalf("expected no lock paths, got %+v", lockPaths)
	}
	if len(searchPaths) != 1 || !strings.Contains(searchPaths[0], "/tmp/path") {
		t.Fatalf("unexpected search paths: %+v", searchPaths)
	}
}

func TestHandleLiquibasePathFlagSearchPathEqualsCoverage(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "other"), 0o700); err != nil {
		t.Fatalf("mkdir other: %v", err)
	}
	args := []string{"--searchPath=db,other"}
	idx := 0
	normalized := []string{}
	mounts := []engineRuntime.Mount{}
	mountIndex := 0
	mounted := map[string]string{}

	handled, err := handleLiquibasePathFlag(args, &idx, cwd, false, false, &normalized, &mounts, &mountIndex, mounted)
	if err != nil || !handled {
		t.Fatalf("handleLiquibasePathFlag: %v handled=%v", err, handled)
	}
	if len(normalized) != 1 || !strings.HasPrefix(normalized[0], "--searchPath=") {
		t.Fatalf("unexpected normalized: %+v", normalized)
	}
}

func TestCollectLiquibasePathsChangelogEquals(t *testing.T) {
	lockPaths, searchPaths, err := collectLiquibasePaths([]string{"--changelog-file=/tmp/changelog.xml"}, "")
	if err != nil {
		t.Fatalf("collectLiquibasePaths: %v", err)
	}
	if len(searchPaths) != 0 {
		t.Fatalf("expected no search paths, got %+v", searchPaths)
	}
	if len(lockPaths) != 1 || !strings.Contains(lockPaths[0], "/tmp/changelog.xml") {
		t.Fatalf("unexpected lock paths: %+v", lockPaths)
	}
}

func TestNormalizeLiquibasePathValuesRemoteRef(t *testing.T) {
	paths, err := normalizeLiquibasePathValues("--searchPath", "classpath:db", "")
	if err != nil {
		t.Fatalf("normalizeLiquibasePathValues: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected empty paths for remote ref, got %+v", paths)
	}
	paths, err = normalizeLiquibasePathValues("--changelog-file", "classpath:db/changelog.xml", "")
	if err != nil {
		t.Fatalf("normalizeLiquibasePathValues: %v", err)
	}
	if paths != nil {
		t.Fatalf("expected nil paths for remote ref, got %+v", paths)
	}
}

func TestNormalizeLiquibasePathRelativeNoCwd(t *testing.T) {
	if out := normalizeLiquibasePath("db/changelog.xml", ""); out != "db/changelog.xml" {
		t.Fatalf("expected unchanged relative path, got %q", out)
	}
}

func TestParseChangesetHeaderInvalid(t *testing.T) {
	if _, ok := parseChangesetHeader("no changeset"); ok {
		t.Fatalf("expected invalid header")
	}
	if _, ok := parseChangesetHeader("-- Changeset only-two::parts"); ok {
		t.Fatalf("expected invalid header parts")
	}
}

func TestParseChangesetChecksumInvalid(t *testing.T) {
	if _, ok := parseChangesetChecksum("insert into other values (1)"); ok {
		t.Fatalf("expected no checksum")
	}
	if _, ok := parseChangesetChecksum("INSERT INTO databasechangelog (ID) VALUES ('1')"); ok {
		t.Fatalf("expected no checksum from mismatched cols/vals")
	}
}

func TestParseInsertColumnsValuesInvalid(t *testing.T) {
	if _, _, ok := parseInsertColumnsValues("insert values"); ok {
		t.Fatalf("expected invalid")
	}
	if _, _, ok := parseInsertColumnsValues("insert into table values ()"); ok {
		t.Fatalf("expected invalid cols")
	}
}

func TestSplitSQLValuesQuotes(t *testing.T) {
	out := splitSQLValues("'a','b''c',d")
	if len(out) != 3 || out[0] != "a" || out[1] != "b'c" || out[2] != "d" {
		t.Fatalf("unexpected values: %+v", out)
	}
}

func TestReplaceLiquibaseCommand(t *testing.T) {
	out := replaceLiquibaseCommand([]string{"--defaults-file", "a", "update"}, "update-count")
	if !containsArg(out, "update-count") {
		t.Fatalf("expected command replaced, got %+v", out)
	}
	out = replaceLiquibaseCommand([]string{}, "update-count")
	if len(out) != 1 || out[0] != "update-count" {
		t.Fatalf("expected command inserted, got %+v", out)
	}
}

func TestSplitCommaListSkipsEmpty(t *testing.T) {
	out := splitCommaList("a, ,b")
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("unexpected list: %+v", out)
	}
}
