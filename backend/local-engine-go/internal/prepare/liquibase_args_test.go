package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareLiquibaseArgsRequiresCommand(t *testing.T) {
	_, err := prepareLiquibaseArgsContainer([]string{}, "", false)
	expectValidationError(t, err, "lb command is required")

	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, changelog, "<databaseChangeLog/>")

	_, err = prepareLiquibaseArgsContainer([]string{"--changelog-file", changelog}, dir, false)
	expectValidationError(t, err, "lb command is required")
}

func TestPrepareLiquibaseArgsRejectsNonUpdateCommand(t *testing.T) {
	_, err := prepareLiquibaseArgsContainer([]string{"rollback"}, "", false)
	expectValidationError(t, err, "unsupported lb command")

	_, err = prepareLiquibaseArgsContainer([]string{"history"}, "", false)
	expectValidationError(t, err, "unsupported lb command")
}

func TestPrepareLiquibaseArgsAcceptsUpdateCommands(t *testing.T) {
	commands := []string{"update", "updateSQL", "updateSql", "updateTestingRollback", "updateCount"}
	for _, cmd := range commands {
		out, err := prepareLiquibaseArgsContainer([]string{cmd}, "", false)
		if err != nil {
			t.Fatalf("prepareLiquibaseArgs(%s): %v", cmd, err)
		}
		if len(out.normalizedArgs) == 0 || out.normalizedArgs[len(out.normalizedArgs)-1] != cmd {
			t.Fatalf("expected command %q in normalized args, got %v", cmd, out.normalizedArgs)
		}
	}
}

func TestPrepareLiquibaseArgsIgnoresSeparatorsAndBlankArgs(t *testing.T) {
	out, err := prepareLiquibaseArgsContainer([]string{" ", "--", "update", "--", " "}, "", false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.normalizedArgs) != 1 || out.normalizedArgs[0] != "update" {
		t.Fatalf("expected only update command after normalization, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsRejectsConnectionFlags(t *testing.T) {
	flags := []string{
		"--url", "--url=jdbc:postgresql://localhost/postgres",
		"--username", "--username=postgres",
		"--password", "--password=postgres",
	}
	for _, flag := range flags {
		_, err := prepareLiquibaseArgsContainer([]string{"update", flag}, "", false)
		expectValidationError(t, err, "connection flags are not allowed")
	}
}

func TestPrepareLiquibaseArgsRejectsRuntimeFlags(t *testing.T) {
	flags := []string{
		"--classpath", "--classpath=./drivers",
		"--driver", "--driver=org.postgresql.Driver",
	}
	for _, flag := range flags {
		_, err := prepareLiquibaseArgsContainer([]string{"update", flag}, "", false)
		expectValidationError(t, err, "runtime flags are not allowed")
	}
}

func TestPrepareLiquibaseArgsRewritesChangelogPath(t *testing.T) {
	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, changelog, "<databaseChangeLog/>")

	out, err := prepareLiquibaseArgsContainer([]string{"update", "--changelog-file", changelog}, dir, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 1 {
		t.Fatalf("expected one mount, got %+v", out.mounts)
	}
	if out.mounts[0].HostPath != changelog || out.mounts[0].ContainerPath != "/sqlrs/mnt/path1" {
		t.Fatalf("unexpected mount mapping: %+v", out.mounts[0])
	}
	if !containsArg(out.normalizedArgs, "--changelog-file") {
		t.Fatalf("expected changelog arg in normalized args: %v", out.normalizedArgs)
	}
	if !containsArg(out.normalizedArgs, "/sqlrs/mnt/path1") {
		t.Fatalf("expected rewritten changelog path, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsRewritesDefaultsFile(t *testing.T) {
	dir := t.TempDir()
	defaults := filepath.Join(dir, "lb.properties")
	writeTempFile(t, defaults, "classpath=.")

	out, err := prepareLiquibaseArgsContainer([]string{"update", "--defaults-file", defaults}, dir, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 1 {
		t.Fatalf("expected one mount, got %+v", out.mounts)
	}
	if !containsArg(out.normalizedArgs, "--defaults-file") || !containsArg(out.normalizedArgs, "/sqlrs/mnt/path1") {
		t.Fatalf("expected rewritten defaults path, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsRewritesSearchPath(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	arg := strings.Join([]string{dirA, dirB}, ",")

	out, err := prepareLiquibaseArgsContainer([]string{"update", "--searchPath", arg}, "", false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 2 {
		t.Fatalf("expected two mounts, got %+v", out.mounts)
	}
	if !containsArg(out.normalizedArgs, "--searchPath") {
		t.Fatalf("expected searchPath arg, got %v", out.normalizedArgs)
	}
	if !containsArg(out.normalizedArgs, "/sqlrs/mnt/path1,/sqlrs/mnt/path2") {
		t.Fatalf("expected rewritten searchPath, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsAcceptsSearchPathAlias(t *testing.T) {
	dir := t.TempDir()
	out, err := prepareLiquibaseArgsContainer([]string{"update", "--search-path", dir}, "", false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if !containsArg(out.normalizedArgs, "--searchPath") {
		t.Fatalf("expected searchPath flag, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsRejectsMissingSearchPathEntry(t *testing.T) {
	_, err := prepareLiquibaseArgsContainer([]string{"update", "--searchPath", ""}, "", false)
	expectValidationError(t, err, "searchPath is empty")
}

func TestPrepareLiquibaseArgsRejectsUnknownSearchPathEntry(t *testing.T) {
	_, err := prepareLiquibaseArgsContainer([]string{"update", "--searchPath", "/missing/path"}, "", false)
	expectValidationError(t, err, "searchPath path does not exist")
}

func TestPrepareLiquibaseArgsResolvesRelativeSearchPath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "db")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out, err := prepareLiquibaseArgsContainer([]string{"update", "--searchPath", "db"}, root, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 1 || out.mounts[0].HostPath != sub {
		t.Fatalf("expected resolved mount to %s, got %+v", sub, out.mounts)
	}
}

func TestPrepareLiquibaseArgsMountsCwdWhenNoLocalPaths(t *testing.T) {
	cwd := t.TempDir()
	out, err := prepareLiquibaseArgsContainer([]string{"update"}, cwd, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 1 {
		t.Fatalf("expected one mount, got %+v", out.mounts)
	}
	if out.mounts[0].HostPath != cwd {
		t.Fatalf("expected cwd mount, got %+v", out.mounts[0])
	}
}

func TestPrepareLiquibaseArgsWindowsModeSkipsMounts(t *testing.T) {
	out, err := prepareLiquibaseArgsContainer([]string{"update", "--changelog-file", "C:\\work\\changelog.xml"}, "C:\\work", true)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 0 {
		t.Fatalf("expected no mounts, got %+v", out.mounts)
	}
	if len(out.normalizedArgs) < 2 || out.normalizedArgs[1] != "C:\\work\\changelog.xml" {
		t.Fatalf("expected windows path preserved, got %+v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsWindowsModeNormalizesSearchPathAliasEquals(t *testing.T) {
	out, err := prepareLiquibaseArgsContainer([]string{"update", "--search-path=C:\\work\\db"}, "C:\\work", true)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if !containsArg(out.normalizedArgs, "--searchPath=C:\\work\\db") {
		t.Fatalf("expected --search-path= alias to normalize to --searchPath=, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsWindowsModeCollectsSearchPathValueErrors(t *testing.T) {
	_, err := prepareLiquibaseArgsContainer([]string{"update", "--searchPath="}, "C:\\work", true)
	expectValidationError(t, err, "path is empty")
}

func TestPrepareLiquibaseArgsKeepsPreAndPostCommandFlags(t *testing.T) {
	out, err := prepareLiquibaseArgsContainer([]string{"--log-level=info", "update", "--label-filter=dev"}, t.TempDir(), false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	expected := []string{"--log-level=info", "update", "--label-filter=dev"}
	for i, want := range expected {
		if i >= len(out.normalizedArgs) || out.normalizedArgs[i] != want {
			t.Fatalf("expected normalized args %v, got %v", expected, out.normalizedArgs)
		}
	}
}

func TestPrepareLiquibaseArgsReusesMountForSamePath(t *testing.T) {
	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, changelog, "<databaseChangeLog/>")

	out, err := prepareLiquibaseArgsContainer([]string{
		"update",
		"--changelog-file", changelog,
		"--defaults-file", changelog,
	}, dir, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 1 {
		t.Fatalf("expected single mount reused for duplicate path, got %+v", out.mounts)
	}
	if strings.Count(strings.Join(out.normalizedArgs, " "), "/sqlrs/mnt/path1") < 2 {
		t.Fatalf("expected same mapped path reused in args, got %v", out.normalizedArgs)
	}
}

func TestPrepareLiquibaseArgsHostModeKeepsPaths(t *testing.T) {
	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	writeTempFile(t, changelog, "<databaseChangeLog/>")

	out, err := prepareLiquibaseArgs([]string{"update", "--changelog-file", changelog}, dir, false, false)
	if err != nil {
		t.Fatalf("prepareLiquibaseArgs: %v", err)
	}
	if len(out.mounts) != 0 {
		t.Fatalf("expected no mounts, got %+v", out.mounts)
	}
	if !containsArg(out.normalizedArgs, changelog) {
		t.Fatalf("expected changelog path preserved, got %v", out.normalizedArgs)
	}
	for _, arg := range out.normalizedArgs {
		if strings.HasPrefix(arg, "/sqlrs/mnt/") {
			t.Fatalf("expected no container path rewrite, got %v", out.normalizedArgs)
		}
	}
}

func prepareLiquibaseArgsContainer(args []string, cwd string, windowsMode bool) (liquibasePrepared, error) {
	return prepareLiquibaseArgs(args, cwd, windowsMode, true)
}
