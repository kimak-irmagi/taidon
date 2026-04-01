package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
)

func TestBuildPsqlRunStepsSharedArgsAndFiles(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(filePath, []byte("select 2;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	steps, err := buildPsqlRunSteps([]string{
		"-v", "ON_ERROR_STOP=1",
		"-c", "select 1",
		"-f", "query.sql",
	}, dir, dir, strings.NewReader(""))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if strings.Join(steps[0].Args, " ") != "-v ON_ERROR_STOP=1 -c select 1" {
		t.Fatalf("unexpected step args: %v", steps[0].Args)
	}
	if strings.Join(steps[1].Args, " ") != "-v ON_ERROR_STOP=1 -f -" {
		t.Fatalf("unexpected file step args: %v", steps[1].Args)
	}
	if steps[1].Stdin == nil || *steps[1].Stdin != "select 2;" {
		t.Fatalf("expected file content stdin, got %+v", steps[1].Stdin)
	}
}

func TestBuildPsqlRunStepsCommandForms(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{
		"--command=select 1",
		"-cselect 2",
	}, "", "", strings.NewReader(""))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if strings.Join(steps[0].Args, " ") != "-c select 1" {
		t.Fatalf("unexpected args: %v", steps[0].Args)
	}
	if strings.Join(steps[1].Args, " ") != "-c select 2" {
		t.Fatalf("unexpected args: %v", steps[1].Args)
	}
}

func TestBuildPsqlRunStepsEmptyCommand(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{"--command="}, "", "", strings.NewReader(""))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if strings.Join(steps[0].Args, " ") != "-c " {
		t.Fatalf("unexpected args: %v", steps[0].Args)
	}
}

func TestBuildPsqlRunStepsFileForms(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.sql"), []byte("select 2;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	steps, err := buildPsqlRunSteps([]string{
		"--file=a.sql",
		"-fb.sql",
	}, dir, dir, strings.NewReader(""))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
}

func TestBuildPsqlRunStepsStdinOnlyOnce(t *testing.T) {
	_, err := buildPsqlRunSteps([]string{"-f", "-", "-f", "-"}, "", "", strings.NewReader("data"))
	if err == nil || !strings.Contains(err.Error(), "Multiple stdin") {
		t.Fatalf("expected stdin error, got %v", err)
	}
}

func TestBuildPsqlRunStepsStdinOnlyOnceFileEquals(t *testing.T) {
	_, err := buildPsqlRunSteps([]string{"--file=-", "--file=-"}, "", "", strings.NewReader("data"))
	if err == nil || !strings.Contains(err.Error(), "Multiple stdin") {
		t.Fatalf("expected stdin error, got %v", err)
	}
}

func TestBuildPsqlRunStepsStdinOnlyOnceShortFlag(t *testing.T) {
	_, err := buildPsqlRunSteps([]string{"-f-", "-f-"}, "", "", strings.NewReader("data"))
	if err == nil || !strings.Contains(err.Error(), "Multiple stdin") {
		t.Fatalf("expected stdin error, got %v", err)
	}
}

func TestBuildPsqlRunStepsReadsStdin(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{"-f", "-"}, "", "", strings.NewReader("stdin data"))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Stdin == nil || *steps[0].Stdin != "stdin data" {
		t.Fatalf("unexpected stdin: %+v", steps[0].Stdin)
	}
}

func TestBuildPsqlRunStepsFileEqualsStdin(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{"--file=-"}, "", "", strings.NewReader("stdin data"))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Stdin == nil || *steps[0].Stdin != "stdin data" {
		t.Fatalf("unexpected stdin: %+v", steps[0].Stdin)
	}
}

func TestBuildPsqlRunStepsFileShortStdin(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{"-f-"}, "", "", strings.NewReader("stdin data"))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Stdin == nil || *steps[0].Stdin != "stdin data" {
		t.Fatalf("unexpected stdin: %+v", steps[0].Stdin)
	}
}

func TestBuildPsqlRunStepsFileShortError(t *testing.T) {
	root := t.TempDir()
	_, err := buildPsqlRunSteps([]string{"-f.."}, root, root, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}

func TestBuildPsqlRunStepsMissingValues(t *testing.T) {
	_, err := buildPsqlRunSteps([]string{"-c"}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
	_, err = buildPsqlRunSteps([]string{"--command"}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
	_, err = buildPsqlRunSteps([]string{"-f"}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
	_, err = buildPsqlRunSteps([]string{"--file"}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
	_, err = buildPsqlRunSteps([]string{"--file="}, "", "", strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestBuildPsqlRunStepsFileStepError(t *testing.T) {
	root := t.TempDir()
	_, err := buildPsqlRunSteps([]string{"-f", ".."}, root, root, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}

func TestParseRunArgsHelp(t *testing.T) {
	_, showHelp, err := parseRunArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if !showHelp {
		t.Fatalf("expected help")
	}
}

func TestParseRunArgsInstanceAndCommand(t *testing.T) {
	parsed, showHelp, err := parseRunArgs([]string{"--instance", "staging", "--", "psql", "-c", "select 1"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("unexpected help")
	}
	if parsed.InstanceRef != "staging" {
		t.Fatalf("unexpected instance: %s", parsed.InstanceRef)
	}
	if parsed.Command != "psql" {
		t.Fatalf("unexpected command: %s", parsed.Command)
	}
	if strings.Join(parsed.Args, " ") != "-c select 1" {
		t.Fatalf("unexpected args: %v", parsed.Args)
	}
}

func TestParseRunArgsInstanceEquals(t *testing.T) {
	parsed, showHelp, err := parseRunArgs([]string{"--instance=dev", "-c", "select 1"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("unexpected help")
	}
	if parsed.InstanceRef != "dev" {
		t.Fatalf("unexpected instance: %s", parsed.InstanceRef)
	}
	if strings.Join(parsed.Args, " ") != "-c select 1" {
		t.Fatalf("unexpected args: %v", parsed.Args)
	}
}

func TestParseRunArgsMissingInstanceValue(t *testing.T) {
	_, _, err := parseRunArgs([]string{"--instance"})
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestParseRunArgsMissingInstanceValueEquals(t *testing.T) {
	_, _, err := parseRunArgs([]string{"--instance="})
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestParseRunArgsMissingInstanceValueWhitespace(t *testing.T) {
	_, _, err := parseRunArgs([]string{"--instance", "  "})
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestParseRunArgsUnicodeDashHint(t *testing.T) {
	_, _, err := parseRunArgs([]string{"—instance", "dev"})
	if err == nil || !strings.Contains(err.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash hint, got %v", err)
	}
	if !strings.Contains(err.Error(), "--instance") {
		t.Fatalf("expected normalized suggestion, got %v", err)
	}
}

func TestParseRunArgsLeadingDashCommand(t *testing.T) {
	parsed, showHelp, err := parseRunArgs([]string{"-c", "select 1"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("unexpected help")
	}
	if parsed.Command != "" {
		t.Fatalf("expected empty command, got %q", parsed.Command)
	}
	if strings.Join(parsed.Args, " ") != "-c select 1" {
		t.Fatalf("unexpected args: %v", parsed.Args)
	}
}

func TestParseRunCommandEmptyArgs(t *testing.T) {
	parsed, showHelp, err := parseRunCommand(runArgs{}, nil)
	if err != nil {
		t.Fatalf("parseRunCommand: %v", err)
	}
	if showHelp {
		t.Fatalf("unexpected help")
	}
	if parsed.Command != "" || len(parsed.Args) != 0 {
		t.Fatalf("unexpected parsed result: %+v", parsed)
	}
}

func TestBuildFileStepStdinMarker(t *testing.T) {
	step, isStdin, err := buildFileStep([]string{"-v", "ON_ERROR_STOP=1"}, "-", "", "")
	if err != nil {
		t.Fatalf("buildFileStep: %v", err)
	}
	if !isStdin {
		t.Fatalf("expected stdin marker")
	}
	if strings.Join(step.Args, " ") != "-v ON_ERROR_STOP=1 -f -" {
		t.Fatalf("unexpected args: %v", step.Args)
	}
}

func TestBuildFileStepMissingPath(t *testing.T) {
	_, _, err := buildFileStep(nil, " ", "", "")
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestBuildFileStepNormalizeFilePathError(t *testing.T) {
	root := t.TempDir()
	_, _, err := buildFileStep(nil, "..", root, root)
	if err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}

func TestBuildFileStepReadFileError(t *testing.T) {
	root := t.TempDir()
	_, _, err := buildFileStep(nil, "missing.sql", root, root)
	if err == nil {
		t.Fatalf("expected read error")
	}
}

func TestRunStepHelpersAdditionalCoverage(t *testing.T) {
	t.Run("build file step reads content", func(t *testing.T) {
		root := t.TempDir()
		filePath := filepath.Join(root, "query.sql")
		if err := os.WriteFile(filePath, []byte("select 1;\n"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		step, isStdin, err := buildFileStep([]string{"-v"}, "query.sql", root, root)
		if err != nil {
			t.Fatalf("buildFileStep: %v", err)
		}
		if isStdin {
			t.Fatalf("expected file-backed step, got stdin marker")
		}
		if got := strings.Join(step.Args, " "); got != "-v -f -" {
			t.Fatalf("unexpected args: %q", got)
		}
		if step.Stdin == nil || *step.Stdin != "select 1;\n" {
			t.Fatalf("unexpected stdin: %+v", step.Stdin)
		}
	})

	t.Run("pgbench helpers", func(t *testing.T) {
		root := t.TempDir()
		filePath := filepath.Join(root, "bench.sql")
		if err := os.WriteFile(filePath, []byte("\\set aid random(1, 100000)\n"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		out, source, err := rewritePgbenchFileArg("bench.sql@3", root, root)
		if err != nil {
			t.Fatalf("rewritePgbenchFileArg: %v", err)
		}
		if out != pgbenchStdinPath+"@3" || source == nil || source.Path != filePath {
			t.Fatalf("unexpected rewrite result: out=%q source=%+v", out, source)
		}

		out, source, err = rewritePgbenchFileArg("-", root, root)
		if err != nil {
			t.Fatalf("rewritePgbenchFileArg stdin: %v", err)
		}
		if out != pgbenchStdinPath || source == nil || !source.UsesStdin {
			t.Fatalf("unexpected stdin rewrite result: out=%q source=%+v", out, source)
		}

		if got, err := readPgbenchFileSource(pgbenchFileSource{UsesStdin: true}, strings.NewReader("stdin data")); err != nil || got != "stdin data" {
			t.Fatalf("unexpected stdin read result: %q err=%v", got, err)
		}
		if got, err := readPgbenchFileSource(pgbenchFileSource{Path: filePath}, nil); err != nil || got != "\\set aid random(1, 100000)\n" {
			t.Fatalf("unexpected file read result: %q err=%v", got, err)
		}

		if path, suffix := splitPgbenchFileArgValue("bench.sql@3"); path != "bench.sql" || suffix != "@3" {
			t.Fatalf("unexpected split result: path=%q suffix=%q", path, suffix)
		}
		if _, _, err := rewritePgbenchFileArg(" ", root, root); err == nil {
			t.Fatalf("expected missing pgbench file error")
		}
	})

	t.Run("runRun pgbench materialize error", func(t *testing.T) {
		err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{InstanceRef: "inst"}, "pgbench", []string{"-f"}, "", "")
		if err == nil || !strings.Contains(err.Error(), "Missing value") {
			t.Fatalf("expected pgbench materialize error, got %v", err)
		}
	})
}

func TestBuildPsqlRunStepsNoSourcesUsesShared(t *testing.T) {
	steps, err := buildPsqlRunSteps([]string{"-v", "ON_ERROR_STOP=1"}, "", "", bytes.NewBuffer(nil))
	if err != nil {
		t.Fatalf("buildPsqlRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if strings.Join(steps[0].Args, " ") != "-v ON_ERROR_STOP=1" {
		t.Fatalf("unexpected args: %v", steps[0].Args)
	}
}

func TestRunRunPgbenchUsesArgs(t *testing.T) {
	var gotArgs []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if args, ok := payload["args"].([]any); ok {
			gotArgs = args
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
	}, "pgbench", []string{"--instance", "staging", "--", "-c", "10"}, "", "")
	if err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if len(gotArgs) == 0 || gotArgs[0] != "-c" {
		t.Fatalf("unexpected args: %+v", gotArgs)
	}
}

func TestRunRunPgbenchMaterializesFileArgToStdin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bench.sql"), []byte("\\set aid random(1, 100000)\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
	}, "pgbench", []string{"--instance", "staging", "--", "-f", "bench.sql", "-T", "30"}, dir, dir)
	if err != nil {
		t.Fatalf("runRun: %v", err)
	}

	args, ok := gotRequest["args"].([]any)
	if !ok || len(args) != 4 || args[0] != "-f" || args[1] != "/dev/stdin" || args[2] != "-T" || args[3] != "30" {
		t.Fatalf("unexpected args: %+v", gotRequest["args"])
	}
	if stdinValue, ok := gotRequest["stdin"].(string); !ok || stdinValue != "\\set aid random(1, 100000)\n" {
		t.Fatalf("unexpected stdin: %+v", gotRequest["stdin"])
	}
}

func TestMaterializePgbenchRunArgsAttachedFileWeight(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bench.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	args, stdinValue, err := materializePgbenchRunArgs([]string{"-fbench.sql@3", "-T", "30"}, dir, dir, strings.NewReader(""))
	if err != nil {
		t.Fatalf("materializePgbenchRunArgs: %v", err)
	}
	if got := strings.Join(args, "|"); got != "-f/dev/stdin@3|-T|30" {
		t.Fatalf("args = %q, want %q", got, "-f/dev/stdin@3|-T|30")
	}
	if stdinValue == nil || *stdinValue != "select 1;\n" {
		t.Fatalf("unexpected stdin: %+v", stdinValue)
	}
}

func TestMaterializePgbenchRunArgsRejectsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write file a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.sql"), []byte("select 2;\n"), 0o600); err != nil {
		t.Fatalf("write file b: %v", err)
	}

	_, _, err := materializePgbenchRunArgs([]string{"-f", "a.sql", "-f", "b.sql"}, dir, dir, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "Multiple pgbench file arguments") {
		t.Fatalf("expected multiple file error, got %v", err)
	}
}

func TestRunRunConflictingInstance(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cliRunOpts(), "psql", []string{"--instance", "dev"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "preceding prepare") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestRunRunMissingInstance(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cliRunOptsEmpty(), "psql", []string{"-c", "select 1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "Missing instance") {
		t.Fatalf("expected missing instance error, got %v", err)
	}
}

func TestRunRunHelp(t *testing.T) {
	var out bytes.Buffer
	err := runRun(&out, &bytes.Buffer{}, cli.RunOptions{}, "psql", []string{"--help"}, "", "")
	if err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected usage output")
	}
}

func TestRunRunUnknownKind(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{InstanceRef: "inst"}, "unknown", []string{}, "", "")
	if err == nil || !strings.Contains(err.Error(), "Unknown run kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}

func TestRunRunConflictingConnectionArgs(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{InstanceRef: "inst"}, "psql", []string{"--", "-h", "127.0.0.1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "Conflicting connection arguments") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestRunRunClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"message":"bad","details":"info"}`)
	}))
	defer server.Close()

	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
	}, "psql", []string{"--instance", "inst", "--", "-c", "select 1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "bad: info") {
		t.Fatalf("expected run error, got %v", err)
	}
}

func TestRunRunExitCodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":5}`+"\n")
	}))
	defer server.Close()

	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
	}, "psql", []string{"--instance", "inst", "--", "-c", "select 1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("expected command failed error, got %v", err)
	}
}

func TestRunRunBuildStepsError(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{InstanceRef: "inst"}, "psql", []string{"-f"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestRunRunParseArgsError(t *testing.T) {
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, cli.RunOptions{}, "psql", []string{"--instance"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "Missing value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestBuildPsqlRunStepsStdinReadError(t *testing.T) {
	_, err := buildPsqlRunSteps([]string{"-f", "-"}, "", "", errReader{})
	if err == nil {
		t.Fatalf("expected stdin read error")
	}
}

func cliRunOpts() cli.RunOptions {
	return cli.RunOptions{InstanceRef: "from-prepare"}
}

func cliRunOptsEmpty() cli.RunOptions {
	return cli.RunOptions{}
}
