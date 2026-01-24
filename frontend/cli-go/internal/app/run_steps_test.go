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

	"sqlrs/cli/internal/cli"
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
