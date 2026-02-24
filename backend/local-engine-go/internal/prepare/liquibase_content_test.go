package prepare

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveLiquibaseChangesetPath(t *testing.T) {
	dir := t.TempDir()
	workFile := filepath.Join(dir, "change.sql")
	if err := os.WriteFile(workFile, []byte("sql"), 0o600); err != nil {
		t.Fatalf("write work file: %v", err)
	}
	searchDir := filepath.Join(dir, "search")
	if err := os.MkdirAll(searchDir, 0o700); err != nil {
		t.Fatalf("mkdir search: %v", err)
	}
	searchFile := filepath.Join(searchDir, "search.sql")
	if err := os.WriteFile(searchFile, []byte("sql"), 0o600); err != nil {
		t.Fatalf("write search file: %v", err)
	}
	prepared := preparedRequest{
		liquibaseWorkDir:     dir,
		liquibaseSearchPaths: []string{searchDir},
	}
	if got := resolveLiquibaseChangesetPath("change.sql", prepared); got != workFile {
		t.Fatalf("expected work dir path, got %q", got)
	}
	if got := resolveLiquibaseChangesetPath("search.sql", prepared); got != searchFile {
		t.Fatalf("expected search path, got %q", got)
	}
	if got := resolveLiquibaseChangesetPath(searchFile, prepared); got != searchFile {
		t.Fatalf("expected absolute path, got %q", got)
	}
}

func TestResolveLiquibaseChangesetPathRemote(t *testing.T) {
	prepared := preparedRequest{}
	if got := resolveLiquibaseChangesetPath("classpath:db/changelog.xml", prepared); got != "" {
		t.Fatalf("expected empty for remote ref, got %q", got)
	}
}

func TestNormalizeLockPathAndLockInputs(t *testing.T) {
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("WSL_DISTRO_NAME", "")

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.sql")
	fileB := filepath.Join(dir, "b.sql")
	if err := os.WriteFile(fileA, []byte("a"), 0o600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0o600); err != nil {
		t.Fatalf("write b: %v", err)
	}
	if out := normalizeLockPath(" "); out != "" {
		t.Fatalf("expected empty normalized path, got %q", out)
	}

	prevLock := lockFileSharedFn
	prevUnlock := unlockFileSharedFn
	lockFileSharedFn = func(*os.File) error { return nil }
	unlockFileSharedFn = func(*os.File) error { return nil }
	t.Cleanup(func() {
		lockFileSharedFn = prevLock
		unlockFileSharedFn = prevUnlock
	})

	prepared := preparedRequest{
		liquibaseLockPaths: []string{fileA},
		liquibaseWorkDir:   dir,
	}
	lock, err := lockLiquibaseInputs(prepared, "b.sql")
	if err != nil {
		t.Fatalf("lockLiquibaseInputs: %v", err)
	}
	if lock == nil {
		t.Fatalf("expected lock")
	}
	if len(lock.files) != 2 {
		t.Fatalf("expected 2 locked files, got %d", len(lock.files))
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("close lock: %v", err)
	}
}

func TestEnsureLiquibaseContentLockError(t *testing.T) {
	prevLock := lockFileSharedFn
	lockFileSharedFn = func(*os.File) error { return os.ErrPermission }
	t.Cleanup(func() { lockFileSharedFn = prevLock })

	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	prepared := preparedRequest{liquibaseLockPaths: []string{path}}
	_, errResp := ensureLiquibaseContentLock(prepared, "")
	if errResp == nil {
		t.Fatalf("expected lock error response")
	}
}

func TestLockLiquibaseInputsSkipsRemoteAndDirectories(t *testing.T) {
	prevLock := lockFileSharedFn
	prevUnlock := unlockFileSharedFn
	lockFileSharedFn = func(*os.File) error { return nil }
	unlockFileSharedFn = func(*os.File) error { return nil }
	t.Cleanup(func() {
		lockFileSharedFn = prevLock
		unlockFileSharedFn = prevUnlock
	})

	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.sql")
	if err := os.WriteFile(filePath, []byte("sql"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	lock, err := lockLiquibaseInputs(preparedRequest{
		liquibaseLockPaths: []string{
			filePath,
			"classpath:db/changelog.xml",
			dir,
			filepath.Join(dir, "missing.sql"),
		},
	}, "classpath:db/next.xml")
	if err != nil {
		t.Fatalf("lockLiquibaseInputs: %v", err)
	}
	if lock == nil {
		t.Fatalf("expected lock")
	}
	if len(lock.files) != 1 {
		t.Fatalf("expected one locked file, got %d", len(lock.files))
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("lock close: %v", err)
	}
}

func TestResolveLiquibaseChangesetPathAdditionalBranches(t *testing.T) {
	absLike := `\\server\share\abs-change.xml`
	if got := resolveLiquibaseChangesetPath(absLike, preparedRequest{}); got != filepath.Clean(absLike) {
		t.Fatalf("expected cleaned absolute-like path, got %q", got)
	}

	prepared := preparedRequest{liquibaseSearchPaths: []string{" ", t.TempDir()}}
	if got := resolveLiquibaseChangesetPath("not-found.xml", prepared); got != "" {
		t.Fatalf("expected empty result for missing changeset, got %q", got)
	}
}

func TestNormalizeLockPathAdditionalBranches(t *testing.T) {
	prevExec := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo /mnt/c/work/changelog.xml")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo /mnt/c/work/changelog.xml")
	}
	t.Cleanup(func() { execCommand = prevExec })
	setWSLForTest(t, true)

	if got := normalizeLockPath(`C:\work\changelog.xml`); got != "/mnt/c/work/changelog.xml" {
		t.Fatalf("expected WSL-mapped lock path, got %q", got)
	}

	absLike := string(filepath.Separator) + filepath.Join("tmp", "local", "file.sql")
	if got := normalizeLockPath(absLike); got != filepath.Clean(absLike) {
		t.Fatalf("expected cleaned absolute-like lock path, got %q", got)
	}
	if got := normalizeLockPath("relative/path.sql"); got != "relative/path.sql" {
		t.Fatalf("expected relative lock path unchanged, got %q", got)
	}
}
