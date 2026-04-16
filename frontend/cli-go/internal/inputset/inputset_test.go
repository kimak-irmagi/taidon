package inputset

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/pathutil"
)

func TestResolverHelpersAndErrors(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	errValue := Errorf("bad_path", "bad %s", "path")
	if errValue.Code != "bad_path" || errValue.Error() != "bad path" {
		t.Fatalf("unexpected user error: %+v", errValue)
	}

	resolver := NewWorkspaceResolver("", cwd, nil)
	if !pathutil.SameLocalPath(resolver.Root, cwd) || !pathutil.SameLocalPath(resolver.BaseDir, cwd) {
		t.Fatalf("unexpected workspace resolver: %+v", resolver)
	}
	rootResolver := NewWorkspaceResolver(root, "", nil)
	if !pathutil.SameLocalPath(rootResolver.Root, root) || !pathutil.SameLocalPath(rootResolver.BaseDir, root) {
		t.Fatalf("unexpected root fallback resolver: %+v", rootResolver)
	}

	aliasResolver := NewAliasResolver(root, filepath.Join(root, "aliases", "demo.prep.s9s.yaml"))
	if !pathutil.SameLocalPath(aliasResolver.BaseDir, filepath.Join(root, "aliases")) {
		t.Fatalf("unexpected alias resolver: %+v", aliasResolver)
	}

	diffResolver := NewDiffResolver(root)
	if !pathutil.SameLocalPath(diffResolver.Root, root) || !pathutil.SameLocalPath(diffResolver.BaseDir, root) {
		t.Fatalf("unexpected diff resolver: %+v", diffResolver)
	}

	path, err := ResolvePath("file.sql", root, cwd, nil)
	if err != nil || !pathutil.SameLocalPath(path, filepath.Join(cwd, "file.sql")) {
		t.Fatalf("unexpected resolved path: %q err=%v", path, err)
	}
	path, err = ResolvePath("file.sql", "", cwd, nil)
	if err != nil || !pathutil.SameLocalPath(path, filepath.Join(cwd, "file.sql")) {
		t.Fatalf("unexpected root-fallback path: %q err=%v", path, err)
	}
	path, err = ResolvePath("file.sql", root, "", nil)
	if err != nil || !pathutil.SameLocalPath(path, filepath.Join(root, "file.sql")) {
		t.Fatalf("unexpected base-fallback path: %q err=%v", path, err)
	}

	converted, err := ResolvePath("file.sql", root, cwd, func(string) (string, error) {
		return "/mnt/c/file.sql", nil
	})
	if err != nil || converted != "/mnt/c/file.sql" {
		t.Fatalf("unexpected converted path: %q err=%v", converted, err)
	}

	if _, err := ResolvePath(" ", root, cwd, nil); err == nil {
		t.Fatalf("expected empty_path error")
	}
	if _, err := ResolvePath("../../outside.sql", root, cwd, nil); err == nil {
		t.Fatalf("expected outside-workspace error")
	}
	if _, err := (Resolver{Root: root, BaseDir: cwd, Convert: func(string) (string, error) { return "", os.ErrInvalid }}).ResolvePath("file.sql"); err == nil {
		t.Fatalf("expected convert error")
	}
}

func TestBoundaryAndUtilityHelpers(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "missing", "child.sql")

	if got := rebasePathToRoot("", root); got != "" {
		t.Fatalf("expected empty path to stay empty, got %q", got)
	}
	if got := rebasePathToRoot(root, root); !pathutil.SameLocalPath(got, root) {
		t.Fatalf("expected root path to stay rooted, got %q", got)
	}
	if got := rebasePathToRoot(filepath.Join(root, "dir", "file.sql"), root); !pathutil.SameLocalPath(got, filepath.Join(root, "dir", "file.sql")) {
		t.Fatalf("unexpected rebased path: %q", got)
	}
	if got := rebasePathToRoot("relative/path.sql", ""); got != "relative/path.sql" {
		t.Fatalf("expected empty root to keep path, got %q", got)
	}
	if runtime.GOOS == "windows" {
		if got := rebasePathToRoot(`Z:\outside.sql`, root); got != `Z:\outside.sql` {
			t.Fatalf("expected cross-volume path to stay unchanged, got %q", got)
		}
	}
	if got := CanonicalizeBoundaryPath(nested); !strings.HasSuffix(filepath.ToSlash(got), "missing/child.sql") {
		t.Fatalf("unexpected canonicalized path: %q", got)
	}
	if got := CanonicalizeBoundaryPath(""); got != "." {
		t.Fatalf("unexpected canonicalized empty path: %q", got)
	}
	if got := CanonicalizeBoundaryPath(root); got == "" {
		t.Fatal("expected existing path to stay non-empty")
	}
	if runtime.GOOS == "windows" {
		if got := CanonicalizeBoundaryPath(`Z:\taidon\missing\child.sql`); got != `Z:\taidon\missing\child.sql` {
			t.Fatalf("expected non-existent drive path to stay unchanged, got %q", got)
		}
	}

	if !IsWithin(root, filepath.Join(root, "dir")) {
		t.Fatalf("expected nested path to be within root")
	}
	if !IsWithin(root, root) {
		t.Fatalf("expected root to be within itself")
	}
	if IsWithin(root, filepath.Join(filepath.Dir(root), "outside")) {
		t.Fatalf("expected outside path to fail boundary check")
	}
	if IsWithin(root, "relative") {
		t.Fatalf("expected relative target to fail boundary check")
	}

	if !LooksLikeLiquibaseRemoteRef("classpath:db/changelog.xml") || !LooksLikeLiquibaseRemoteRef("https://example.com/db") || LooksLikeLiquibaseRemoteRef("local/file.xml") {
		t.Fatalf("unexpected remote-ref detection")
	}

	if path, weight := SplitPgbenchFileArgValue("bench.sql@10"); path != "bench.sql" || weight != "@10" {
		t.Fatalf("unexpected weighted split: %q %q", path, weight)
	}
	if path, weight := SplitPgbenchFileArgValue("bench.sql"); path != "bench.sql" || weight != "" {
		t.Fatalf("unexpected plain split: %q %q", path, weight)
	}
	if path, weight := SplitPgbenchFileArgValue("bench.sql@"); path != "bench.sql@" || weight != "" {
		t.Fatalf("unexpected trailing-at split: %q %q", path, weight)
	}
	if path, weight := SplitPgbenchFileArgValue("bench.sql@x"); path != "bench.sql@x" || weight != "" {
		t.Fatalf("unexpected invalid-weight split: %q %q", path, weight)
	}
	if path, weight := SplitPgbenchFileArgValue("@10"); path != "@10" || weight != "" {
		t.Fatalf("unexpected empty-path split: %q %q", path, weight)
	}
}

func TestOSFileSystemAndHashContent(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.sql")
	if err := os.WriteFile(filePath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	fs := OSFileSystem{}
	if info, err := fs.Stat(filePath); err != nil || info.IsDir() {
		t.Fatalf("unexpected stat result: info=%+v err=%v", info, err)
	}
	if data, err := fs.ReadFile(filePath); err != nil || string(data) != "select 1;\n" {
		t.Fatalf("unexpected read file result: data=%q err=%v", string(data), err)
	}
	if entries, err := fs.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("unexpected read dir result: entries=%+v err=%v", entries, err)
	}

	if got := HashContent([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("unexpected hash: %q", got)
	}
}

func TestResolvePath_AbsolutePathWithinRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "dir", "file.sql")
	resolved, err := ResolvePath(target, root, filepath.Join(root, "cwd"), nil)
	if err != nil {
		t.Fatalf("ResolvePath absolute: %v", err)
	}
	if resolved != target {
		t.Fatalf("expected absolute path unchanged, got %q", resolved)
	}
}
