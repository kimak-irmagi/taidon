package prepare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewritePsqlFileArgs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sql")
	path := writeSQLAt(t, dir, "init.sql", "select 1;")

	mount := &scriptMount{
		HostRoot:      root,
		ContainerRoot: containerScriptsRoot,
	}
	args := []string{"-f", path, "-c", "select 1;"}
	rewritten, workdir, err := rewritePsqlFileArgs(args, mount)
	if err != nil {
		t.Fatalf("rewritePsqlFileArgs: %v", err)
	}
	expectedPath := containerScriptsRoot + "/sql/init.sql"
	if rewritten[0] != "-f" || rewritten[1] != expectedPath {
		t.Fatalf("unexpected rewrite: %+v", rewritten)
	}
	if workdir != containerScriptsRoot+"/sql" {
		t.Fatalf("unexpected workdir: %s", workdir)
	}
}

func TestBuildPsqlExecArgsPrependsConnectionFlags(t *testing.T) {
	root := t.TempDir()
	path := writeSQLAt(t, root, "init.sql", "select 1;")
	mount := &scriptMount{
		HostRoot:      root,
		ContainerRoot: containerScriptsRoot,
	}
	args, _, err := buildPsqlExecArgs([]string{"-f", path}, mount)
	if err != nil {
		t.Fatalf("buildPsqlExecArgs: %v", err)
	}
	if len(args) < 6 || args[0] != "psql" {
		t.Fatalf("unexpected exec args: %+v", args)
	}
}

func TestRewritePsqlFileArgsMissingValueForFileFlag(t *testing.T) {
	_, _, err := rewritePsqlFileArgs([]string{"-f"}, &scriptMount{HostRoot: t.TempDir(), ContainerRoot: containerScriptsRoot})
	if err == nil {
		t.Fatalf("expected error for missing file value")
	}
}

func TestRewritePsqlFileArgsSupportsLongAndShortForms(t *testing.T) {
	root := t.TempDir()
	path := writeSQLAt(t, root, "init.sql", "select 1;")
	mount := &scriptMount{
		HostRoot:      root,
		ContainerRoot: containerScriptsRoot,
	}
	args := []string{"--file=" + path, "-f" + path}
	rewritten, _, err := rewritePsqlFileArgs(args, mount)
	if err != nil {
		t.Fatalf("rewritePsqlFileArgs: %v", err)
	}
	if rewritten[0] != "--file="+containerScriptsRoot+"/init.sql" {
		t.Fatalf("unexpected rewrite: %+v", rewritten)
	}
	if rewritten[1] != "-f"+containerScriptsRoot+"/init.sql" {
		t.Fatalf("unexpected rewrite: %+v", rewritten)
	}
}

func TestMapScriptPathErrors(t *testing.T) {
	root := t.TempDir()
	mount := &scriptMount{
		HostRoot:      root,
		ContainerRoot: containerScriptsRoot,
	}
	if _, err := mapScriptPath("rel.sql", mount); err == nil {
		t.Fatalf("expected error for relative path")
	}
	outside := filepath.Join(t.TempDir(), "outside.sql")
	if _, err := mapScriptPath(outside, mount); err == nil {
		t.Fatalf("expected error for path outside root")
	}
	if _, err := mapScriptPath(outside, nil); err == nil {
		t.Fatalf("expected error for missing mount")
	}
}

func TestScriptMountForFilesCommonDir(t *testing.T) {
	root := t.TempDir()
	pathA := writeSQLAt(t, filepath.Join(root, "a"), "a.sql", "select 1;")
	pathB := writeSQLAt(t, filepath.Join(root, "b"), "b.sql", "select 1;")
	mount, err := scriptMountForFiles([]string{pathA, pathB})
	if err != nil {
		t.Fatalf("scriptMountForFiles: %v", err)
	}
	if mount == nil || mount.HostRoot != root {
		t.Fatalf("unexpected mount: %+v", mount)
	}
}

func TestRuntimeMountsFromNil(t *testing.T) {
	if mounts := runtimeMountsFrom(nil); mounts != nil {
		t.Fatalf("expected nil mounts, got %+v", mounts)
	}
}

func TestRewritePsqlFileArgsWithoutMount(t *testing.T) {
	args := []string{"-c", "select 1"}
	rewritten, workdir, err := rewritePsqlFileArgs(args, nil)
	if err != nil {
		t.Fatalf("rewritePsqlFileArgs: %v", err)
	}
	if workdir != "" {
		t.Fatalf("expected empty workdir, got %q", workdir)
	}
	if len(rewritten) != len(args) || rewritten[0] != args[0] || rewritten[1] != args[1] {
		t.Fatalf("unexpected args: %+v", rewritten)
	}
}

func TestMapScriptPathStdin(t *testing.T) {
	mapped, err := mapScriptPath("-", nil)
	if err != nil {
		t.Fatalf("mapScriptPath: %v", err)
	}
	if mapped != "-" {
		t.Fatalf("unexpected mapped path: %s", mapped)
	}
}

func TestCommonDirEmpty(t *testing.T) {
	if _, err := commonDir(nil); err == nil {
		t.Fatalf("expected error for empty paths")
	}
}

func TestScriptMountForFilesEmpty(t *testing.T) {
	mount, err := scriptMountForFiles(nil)
	if err != nil {
		t.Fatalf("scriptMountForFiles: %v", err)
	}
	if mount != nil {
		t.Fatalf("expected nil mount, got %+v", mount)
	}
}

func TestRuntimeMountsFrom(t *testing.T) {
	mount := &scriptMount{
		HostRoot:      t.TempDir(),
		ContainerRoot: containerScriptsRoot,
	}
	mounts := runtimeMountsFrom(mount)
	if len(mounts) != 1 {
		t.Fatalf("expected one mount, got %+v", mounts)
	}
	if mounts[0].HostPath != mount.HostRoot || mounts[0].ContainerPath != mount.ContainerRoot || !mounts[0].ReadOnly {
		t.Fatalf("unexpected mount: %+v", mounts[0])
	}
}

func writeSQLAt(t *testing.T, dir string, name string, contents string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write sql: %v", err)
	}
	return path
}
