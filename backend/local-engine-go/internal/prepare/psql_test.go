package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparePsqlArgsAddsDefaults(t *testing.T) {
	path := writeTempSQL(t, "select 1;")
	out, err := preparePsqlArgs([]string{"-f", path}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if !containsArg(out.normalizedArgs, "-X") {
		t.Fatalf("expected -X to be added")
	}
	if !containsArg(out.normalizedArgs, "ON_ERROR_STOP=1") {
		t.Fatalf("expected ON_ERROR_STOP=1 to be added")
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "file" {
		t.Fatalf("expected file hash, got %+v", out.inputHashes)
	}
	if len(out.filePaths) != 1 || out.filePaths[0] != path {
		t.Fatalf("expected file path tracking, got %+v", out.filePaths)
	}
}

func TestPreparePsqlArgsRespectsProvidedDefaults(t *testing.T) {
	path := writeTempSQL(t, "select 1;")
	out, err := preparePsqlArgs([]string{"-X", "-v", "ON_ERROR_STOP=1", "-f", path}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if countArg(out.normalizedArgs, "-X") != 1 {
		t.Fatalf("expected single -X, got %v", out.normalizedArgs)
	}
	if countArg(out.normalizedArgs, "ON_ERROR_STOP=1") != 1 {
		t.Fatalf("expected single ON_ERROR_STOP=1, got %v", out.normalizedArgs)
	}
}

func TestPreparePsqlArgsRejectsConnectionFlags(t *testing.T) {
	flags := []string{
		"-h", "--host", "--host=localhost",
		"-p", "--port", "-p5432", "--port=5432",
		"-U", "--username", "-Uuser", "--username=postgres",
		"-d", "-dpostgres", "--dbname", "--dbname=postgres",
		"--database", "--database=test",
	}
	for _, flag := range flags {
		_, err := preparePsqlArgs([]string{flag}, nil)
		expectValidationError(t, err, "connection flags are not allowed")
	}
}

func TestPreparePsqlArgsRejectsPositionalArgs(t *testing.T) {
	_, err := preparePsqlArgs([]string{"db"}, nil)
	expectValidationError(t, err, "positional database arguments are not allowed")

	_, err = preparePsqlArgs([]string{"--", "db"}, nil)
	expectValidationError(t, err, "positional arguments are not allowed")
}

func TestPreparePsqlArgsHandlesStdin(t *testing.T) {
	input := "select 1;"
	out, err := preparePsqlArgs([]string{"-f", "-"}, &input)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "stdin" {
		t.Fatalf("expected stdin hash, got %+v", out.inputHashes)
	}

	_, err = preparePsqlArgs([]string{"-f", "-"}, nil)
	expectValidationError(t, err, "stdin is required when using -f -")

	path := writeTempSQL(t, "select 1;")
	_, err = preparePsqlArgs([]string{"-f", path}, &input)
	expectValidationError(t, err, "stdin is only valid with -f -")
}

func TestPreparePsqlArgsRejectsInvalidFileFlag(t *testing.T) {
	_, err := preparePsqlArgs([]string{"-f"}, nil)
	expectValidationError(t, err, "missing value for file flag")

	_, err = preparePsqlArgs([]string{"--file"}, nil)
	expectValidationError(t, err, "missing value for file flag")

	_, err = preparePsqlArgs([]string{"--file="}, nil)
	expectValidationError(t, err, "missing value for file flag")

	_, err = preparePsqlArgs([]string{"-f", "relative.sql"}, nil)
	expectValidationError(t, err, "file path must be absolute")
}

func TestPreparePsqlArgsHandlesCommandFlags(t *testing.T) {
	out, err := preparePsqlArgs([]string{"-c", "select 1;"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "command" {
		t.Fatalf("expected command hash, got %+v", out.inputHashes)
	}

	_, err = preparePsqlArgs([]string{"-c"}, nil)
	expectValidationError(t, err, "missing value for command flag")

	out, err = preparePsqlArgs([]string{"-cselect 1;"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "command" {
		t.Fatalf("expected inline command hash, got %+v", out.inputHashes)
	}

	out, err = preparePsqlArgs([]string{"--command=select 1;"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "command" {
		t.Fatalf("expected command hash, got %+v", out.inputHashes)
	}
}

func TestPreparePsqlArgsHandlesVarFlags(t *testing.T) {
	_, err := preparePsqlArgs([]string{"-v"}, nil)
	expectValidationError(t, err, "missing value for variable flag")

	_, err = preparePsqlArgs([]string{"--set"}, nil)
	expectValidationError(t, err, "missing value for variable flag")

	_, err = preparePsqlArgs([]string{"--variable"}, nil)
	expectValidationError(t, err, "missing value for variable flag")

	_, err = preparePsqlArgs([]string{"-v", "ON_ERROR_STOP"}, nil)
	expectValidationError(t, err, "ON_ERROR_STOP must be set to 1")

	_, err = preparePsqlArgs([]string{"-v", "FOO"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}

	_, err = preparePsqlArgs([]string{"-v", "ON_ERROR_STOP=0"}, nil)
	expectValidationError(t, err, "ON_ERROR_STOP must be set to 1")

	_, err = preparePsqlArgs([]string{"-vON_ERROR_STOP=1"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}

	_, err = preparePsqlArgs([]string{"--variable=ON_ERROR_STOP=1"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}

	_, err = preparePsqlArgs([]string{"--set=ON_ERROR_STOP=1"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}

	_, err = preparePsqlArgs([]string{"--set=ON_ERROR_STOP=0"}, nil)
	expectValidationError(t, err, "ON_ERROR_STOP must be set to 1")

	_, err = preparePsqlArgs([]string{"--set", "FOO=bar"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}

	_, err = preparePsqlArgs([]string{"--variable=ON_ERROR_STOP=0"}, nil)
	expectValidationError(t, err, "ON_ERROR_STOP must be set to 1")

	_, err = preparePsqlArgs([]string{"--variable", "FOO=bar"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
}

func TestPreparePsqlArgsIgnoresMiscFlags(t *testing.T) {
	out, err := preparePsqlArgs([]string{"", "-q"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if !containsArg(out.normalizedArgs, "-X") {
		t.Fatalf("expected -X to be added")
	}
	if !containsArg(out.normalizedArgs, "ON_ERROR_STOP=1") {
		t.Fatalf("expected ON_ERROR_STOP=1 to be added")
	}
}

func TestPreparePsqlArgsAllowsTerminator(t *testing.T) {
	out, err := preparePsqlArgs([]string{"--"}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if !containsArg(out.normalizedArgs, "-X") {
		t.Fatalf("expected -X to be added")
	}
	if !containsArg(out.normalizedArgs, "ON_ERROR_STOP=1") {
		t.Fatalf("expected ON_ERROR_STOP=1 to be added")
	}
}

func TestPreparePsqlArgsHandlesInlineFileFlag(t *testing.T) {
	path := writeTempSQL(t, "select 1;")
	out, err := preparePsqlArgs([]string{"-f" + path}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "file" {
		t.Fatalf("expected file hash, got %+v", out.inputHashes)
	}
}

func TestPreparePsqlArgsHandlesLongFileFlag(t *testing.T) {
	path := writeTempSQL(t, "select 1;")
	out, err := preparePsqlArgs([]string{"--file", path}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "file" {
		t.Fatalf("expected file hash, got %+v", out.inputHashes)
	}
}

func TestPreparePsqlArgsHandlesFileFlagEquals(t *testing.T) {
	path := writeTempSQL(t, "select 1;")
	out, err := preparePsqlArgs([]string{"--file=" + path}, nil)
	if err != nil {
		t.Fatalf("preparePsqlArgs: %v", err)
	}
	if len(out.inputHashes) != 1 || out.inputHashes[0].Kind != "file" {
		t.Fatalf("expected file hash, got %+v", out.inputHashes)
	}
}

func TestPreparePsqlArgsRejectsInlineRelativeFile(t *testing.T) {
	_, err := preparePsqlArgs([]string{"-frelative.sql"}, nil)
	expectValidationError(t, err, "file path must be absolute")
}

func TestPreparePsqlArgsRejectsEmptyFilePath(t *testing.T) {
	_, err := preparePsqlArgs([]string{"-f", ""}, nil)
	expectValidationError(t, err, "file path is empty")
}

func TestPreparePsqlArgsRejectsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.sql")
	_, err := preparePsqlArgs([]string{"-f", path}, nil)
	expectValidationError(t, err, "cannot read file")
}

func writeTempSQL(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "init.sql")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func expectValidationError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, contains) && !strings.Contains(verr.Error(), contains) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func countArg(args []string, value string) int {
	count := 0
	for _, arg := range args {
		if arg == value {
			count++
		}
	}
	return count
}
