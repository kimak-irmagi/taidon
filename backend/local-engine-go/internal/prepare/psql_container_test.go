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
