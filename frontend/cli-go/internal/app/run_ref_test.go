package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

func TestParseRunArgsAcceptsRefFlagsAndDefaultsToWorktree(t *testing.T) {
	parsed, showHelp, err := parseRunArgs([]string{"--ref", "HEAD~1", "--instance", "dev", "--", "-c", "select 1"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if showHelp {
		t.Fatal("unexpected help")
	}
	if parsed.Ref != "HEAD~1" {
		t.Fatalf("Ref = %q, want %q", parsed.Ref, "HEAD~1")
	}
	if parsed.RefMode != "worktree" {
		t.Fatalf("RefMode = %q, want %q", parsed.RefMode, "worktree")
	}
	if parsed.RefKeepWorktree {
		t.Fatal("expected RefKeepWorktree=false")
	}
	if parsed.InstanceRef != "dev" {
		t.Fatalf("InstanceRef = %q, want %q", parsed.InstanceRef, "dev")
	}
	if parsed.Command != "" {
		t.Fatalf("Command = %q, want empty", parsed.Command)
	}
	if got := strings.Join(parsed.Args, " "); got != "-c select 1" {
		t.Fatalf("Args = %q, want %q", got, "-c select 1")
	}
}

func TestParseRunArgsRejectsRefModeWithoutRefAndRejectsBlobKeepWorktree(t *testing.T) {
	t.Run("ref mode requires ref", func(t *testing.T) {
		_, _, err := parseRunArgs([]string{"--ref-mode", "blob", "--instance", "dev"})
		if err == nil || !strings.Contains(err.Error(), "--ref-mode requires --ref") {
			t.Fatalf("expected --ref-mode requires --ref, got %v", err)
		}
	})

	t.Run("keep worktree requires ref", func(t *testing.T) {
		_, _, err := parseRunArgs([]string{"--ref-keep-worktree", "--instance", "dev"})
		if err == nil || !strings.Contains(err.Error(), "--ref-keep-worktree requires --ref") {
			t.Fatalf("expected --ref-keep-worktree requires --ref, got %v", err)
		}
	})

	t.Run("blob rejects keep worktree", func(t *testing.T) {
		_, _, err := parseRunArgs([]string{"--ref", "HEAD", "--ref-mode", "blob", "--ref-keep-worktree", "--instance", "dev"})
		if err == nil || !strings.Contains(err.Error(), "--ref-keep-worktree is only valid with --ref-mode worktree") {
			t.Fatalf("expected blob keep-worktree rejection, got %v", err)
		}
	})
}

func TestParseRunAliasArgsAcceptsRefFlagsAndPreservesStandaloneInstanceRequirement(t *testing.T) {
	t.Run("accepts ref flags", func(t *testing.T) {
		invocation, showHelp, err := parseRunAliasArgs([]string{"--ref", "HEAD~1", "--ref-mode", "blob", "smoke", "--instance", "dev"}, true)
		if err != nil {
			t.Fatalf("parseRunAliasArgs: %v", err)
		}
		if showHelp {
			t.Fatal("unexpected help")
		}
		if invocation.GitRef != "HEAD~1" {
			t.Fatalf("GitRef = %q, want %q", invocation.GitRef, "HEAD~1")
		}
		if invocation.RefMode != "blob" {
			t.Fatalf("RefMode = %q, want %q", invocation.RefMode, "blob")
		}
		if invocation.Ref != "smoke" {
			t.Fatalf("Ref = %q, want %q", invocation.Ref, "smoke")
		}
		if invocation.InstanceRef != "dev" {
			t.Fatalf("InstanceRef = %q, want %q", invocation.InstanceRef, "dev")
		}
	})

	t.Run("still requires instance when standalone", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"--ref", "HEAD~1", "smoke"}, true)
		if err == nil || !strings.Contains(err.Error(), "run alias requires --instance") {
			t.Fatalf("expected standalone instance requirement, got %v", err)
		}
	})
}

func TestParseRunAliasArgsRejectsInvalidRefFlagCombinations(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "ref mode without ref",
			args: []string{"--ref-mode", "blob", "smoke", "--instance", "dev"},
			want: "--ref-mode requires --ref",
		},
		{
			name: "keep worktree without ref",
			args: []string{"--ref-keep-worktree", "smoke", "--instance", "dev"},
			want: "--ref-keep-worktree requires --ref",
		},
		{
			name: "missing ref value",
			args: []string{"--ref", " ", "smoke", "--instance", "dev"},
			want: "Missing value for --ref",
		},
		{
			name: "missing ref mode value",
			args: []string{"--ref", "HEAD", "--ref-mode", "smoke", "--instance", "dev"},
			want: "--ref-mode \"smoke\" is not supported",
		},
		{
			name: "bad ref mode",
			args: []string{"--ref", "HEAD", "--ref-mode", "bad", "smoke", "--instance", "dev"},
			want: "--ref-mode \"bad\" is not supported",
		},
		{
			name: "blob keep worktree",
			args: []string{"--ref", "HEAD", "--ref-mode", "blob", "--ref-keep-worktree", "smoke", "--instance", "dev"},
			want: "--ref-keep-worktree is only valid with --ref-mode worktree",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseRunAliasArgs(tc.args, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRunnerRejectsCompositeRunRefBeforeDispatch(t *testing.T) {
	cwd := t.TempDir()
	runCalled := false

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare:psql", Args: []string{"--image", "img", "--", "-c", "select 1"}},
		{Name: "run", Args: []string{"--ref", "HEAD", "smoke"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", false), nil
		}
		deps.prepareResult = func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error) {
			t.Fatal("prepareResult should not be called after composite run --ref rejection")
			return client.PrepareJobResult{}, false, nil
		}
		deps.runRun = func(io.Writer, io.Writer, cli.RunOptions, string, []string, string, string) error {
			runCalled = true
			return nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "run --ref does not support composite prepare ... run yet") {
		t.Fatalf("expected composite run --ref error, got %v", err)
	}
	if runCalled {
		t.Fatal("runRun should not be called after composite run --ref rejection")
	}
}

func TestResolveRunAliasWithOptionalRefLoadsAliasFromSelectedFilesystem(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	cwd := filepath.Join(repo, "examples")

	for _, mode := range []string{"blob", "worktree"} {
		t.Run(mode, func(t *testing.T) {
			alias, aliasPath, ctx, err := resolveRunAliasWithOptionalRef(repo, cwd, "chinook/smoke", parentRef, mode, false)
			if err != nil {
				t.Fatalf("resolveRunAliasWithOptionalRef: %v", err)
			}
			if ctx == nil {
				t.Fatal("expected ref context")
			}
			t.Cleanup(func() {
				if err := ctx.Cleanup(); err != nil {
					t.Fatalf("Cleanup(%s): %v", mode, err)
				}
			})
			if alias.Kind != "psql" {
				t.Fatalf("Kind = %q, want %q", alias.Kind, "psql")
			}
			if got := strings.Join(alias.Args, "|"); got != "-f|./scripts/first.sql" {
				t.Fatalf("Args = %q, want %q", got, "-f|./scripts/first.sql")
			}
			if filepath.Base(aliasPath) != "smoke.run.s9s.yaml" {
				t.Fatalf("aliasPath = %q, want file smoke.run.s9s.yaml", aliasPath)
			}
		})
	}
}

func TestResolveRunAliasWithOptionalRefCleansRefContextOnResolveOrLoadFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	cwd := filepath.Join(repo, "examples")
	baseline := gitWorktreeCount(t, repo)

	t.Run("resolve failure", func(t *testing.T) {
		_, _, _, err := resolveRunAliasWithOptionalRef(repo, cwd, "current-only", parentRef, "worktree", false)
		if err == nil || !strings.Contains(err.Error(), "run alias file not found") {
			t.Fatalf("expected alias not found error, got %v", err)
		}
		if got := gitWorktreeCount(t, repo); got != baseline {
			t.Fatalf("worktree count = %d, want %d", got, baseline)
		}
	})

	t.Run("load failure", func(t *testing.T) {
		_, _, _, err := resolveRunAliasWithOptionalRef(repo, cwd, "broken", parentRef, "worktree", false)
		if err == nil || !strings.Contains(err.Error(), "read run alias") {
			t.Fatalf("expected run alias load error, got %v", err)
		}
		if got := gitWorktreeCount(t, repo); got != baseline {
			t.Fatalf("worktree count = %d, want %d", got, baseline)
		}
	})
}

func TestProjectedRunInvocationCWDPrefersProjectedRefBaseDir(t *testing.T) {
	liveCWD := filepath.Join(t.TempDir(), "examples")
	projectedCWD := filepath.Join(t.TempDir(), "projected", "examples")

	if got := projectedRunInvocationCWD(liveCWD, nil); got != liveCWD {
		t.Fatalf("projectedRunInvocationCWD(nil) = %q, want %q", got, liveCWD)
	}

	if got := projectedRunInvocationCWD(liveCWD, &refctx.Context{}); got != liveCWD {
		t.Fatalf("projectedRunInvocationCWD(blank) = %q, want %q", got, liveCWD)
	}

	if got := projectedRunInvocationCWD(liveCWD, &refctx.Context{BaseDir: projectedCWD}); got != projectedCWD {
		t.Fatalf("projectedRunInvocationCWD(ref) = %q, want %q", got, projectedCWD)
	}
}

func TestRunRefRawPsqlBindsFileInputsFromProjectedCwd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))

	for _, mode := range []string{"blob", "worktree"} {
		t.Run(mode, func(t *testing.T) {
			var gotRequest client.RunRequest
			server := newRunCaptureServer(t, func(req client.RunRequest, _ map[string]any) {
				gotRequest = req
			})
			defer server.Close()

			err := Run([]string{
				"--mode", "remote",
				"--endpoint", server.URL,
				"--workspace", repo,
				"run:psql",
				"--ref", parentRef,
				"--ref-mode", mode,
				"--instance", "dev",
				"--",
				"-f", "query.sql",
			})
			if err != nil {
				t.Fatalf("Run(run:psql --ref): %v", err)
			}
			if gotRequest.Kind != "psql" {
				t.Fatalf("Kind = %q, want %q", gotRequest.Kind, "psql")
			}
			if gotRequest.InstanceRef != "dev" {
				t.Fatalf("InstanceRef = %q, want %q", gotRequest.InstanceRef, "dev")
			}
			if len(gotRequest.Steps) != 1 {
				t.Fatalf("Steps len = %d, want 1", len(gotRequest.Steps))
			}
			if got := strings.Join(gotRequest.Steps[0].Args, " "); got != "-f -" {
				t.Fatalf("step args = %q, want %q", got, "-f -")
			}
			if gotRequest.Steps[0].Stdin == nil || strings.ReplaceAll(*gotRequest.Steps[0].Stdin, "\r\n", "\n") != "select 1;\n" {
				t.Fatalf("unexpected step stdin: %+v", gotRequest.Steps[0].Stdin)
			}
		})
	}
}

func TestRunRefRawPgbenchBindsFileInputsFromProjectedCwd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))

	for _, mode := range []string{"blob", "worktree"} {
		t.Run(mode, func(t *testing.T) {
			var gotRequest client.RunRequest
			server := newRunCaptureServer(t, func(req client.RunRequest, _ map[string]any) {
				gotRequest = req
			})
			defer server.Close()

			err := Run([]string{
				"--mode", "remote",
				"--endpoint", server.URL,
				"--workspace", repo,
				"run:pgbench",
				"--ref", parentRef,
				"--ref-mode", mode,
				"--instance", "perf",
				"--",
				"-f", "bench.sql",
				"-T", "30",
			})
			if err != nil {
				t.Fatalf("Run(run:pgbench --ref): %v", err)
			}
			if gotRequest.Kind != "pgbench" {
				t.Fatalf("Kind = %q, want %q", gotRequest.Kind, "pgbench")
			}
			if gotRequest.InstanceRef != "perf" {
				t.Fatalf("InstanceRef = %q, want %q", gotRequest.InstanceRef, "perf")
			}
			if got := strings.Join(gotRequest.Args, "|"); got != "-f|/dev/stdin|-T|30" {
				t.Fatalf("Args = %q, want %q", got, "-f|/dev/stdin|-T|30")
			}
			if gotRequest.Stdin == nil || strings.ReplaceAll(*gotRequest.Stdin, "\r\n", "\n") != "\\set aid 1\n" {
				t.Fatalf("unexpected stdin: %+v", gotRequest.Stdin)
			}
		})
	}
}

func TestRunRefAliasRebasesFileBearingArgsRelativeToAliasFileAtRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))

	for _, mode := range []string{"blob", "worktree"} {
		t.Run(mode, func(t *testing.T) {
			var gotRequest client.RunRequest
			server := newRunCaptureServer(t, func(req client.RunRequest, _ map[string]any) {
				gotRequest = req
			})
			defer server.Close()

			err := Run([]string{
				"--mode", "remote",
				"--endpoint", server.URL,
				"--workspace", repo,
				"run",
				"--ref", parentRef,
				"--ref-mode", mode,
				"chinook/smoke",
				"--instance", "dev",
			})
			if err != nil {
				t.Fatalf("Run(run alias --ref): %v", err)
			}
			if len(gotRequest.Steps) != 1 {
				t.Fatalf("Steps len = %d, want 1", len(gotRequest.Steps))
			}
			if got := strings.Join(gotRequest.Steps[0].Args, " "); got != "-f -" {
				t.Fatalf("step args = %q, want %q", got, "-f -")
			}
			if gotRequest.Steps[0].Stdin == nil || strings.ReplaceAll(*gotRequest.Steps[0].Stdin, "\r\n", "\n") != "select 'first';\n" {
				t.Fatalf("unexpected step stdin: %+v", gotRequest.Steps[0].Stdin)
			}
		})
	}
}

func TestRunRefFailsWhenProjectedCwdIsMissingAtSelectedRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples", "later"))

	for _, mode := range []string{"blob", "worktree"} {
		t.Run(mode, func(t *testing.T) {
			err := Run([]string{
				"--mode", "remote",
				"--endpoint", "http://example.invalid",
				"--workspace", repo,
				"run:psql",
				"--ref", parentRef,
				"--ref-mode", mode,
				"--instance", "dev",
				"--",
				"-f", "query.sql",
			})
			if err == nil || !strings.Contains(err.Error(), "projected cwd missing at ref") {
				t.Fatalf("expected projected cwd error, got %v", err)
			}
		})
	}
}

func TestRunRefFailsWhenAliasOrRawFileInputIsMissingAtSelectedRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))

	t.Run("missing alias at ref", func(t *testing.T) {
		err := Run([]string{
			"--mode", "remote",
			"--endpoint", "http://example.invalid",
			"--workspace", repo,
			"run",
			"--ref", parentRef,
			"current-only",
			"--instance", "dev",
		})
		if err == nil || !strings.Contains(err.Error(), "run alias file not found") {
			t.Fatalf("expected missing alias error, got %v", err)
		}
	})

	t.Run("missing raw file at ref", func(t *testing.T) {
		err := Run([]string{
			"--mode", "remote",
			"--endpoint", "http://example.invalid",
			"--workspace", repo,
			"run:psql",
			"--ref", parentRef,
			"--instance", "dev",
			"--",
			"-f", "current-only.sql",
		})
		if err == nil || !strings.Contains(err.Error(), "current-only.sql") {
			t.Fatalf("expected missing raw file error, got %v", err)
		}
	})
}

func TestRunRefUsesExistingRunTransportWithMaterializedRunOptions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))

	var gotRequest client.RunRequest
	var gotRaw map[string]any
	server := newRunCaptureServer(t, func(req client.RunRequest, raw map[string]any) {
		gotRequest = req
		gotRaw = raw
	})
	defer server.Close()

	err := Run([]string{
		"--mode", "remote",
		"--endpoint", server.URL,
		"--workspace", repo,
		"run:psql",
		"--ref", parentRef,
		"--instance", "dev",
		"--",
		"-c", "select 1",
	})
	if err != nil {
		t.Fatalf("Run(run:psql --ref -c): %v", err)
	}

	if gotRequest.InstanceRef != "dev" || gotRequest.Kind != "psql" {
		t.Fatalf("unexpected request identity: %+v", gotRequest)
	}
	if len(gotRequest.Steps) != 1 || strings.Join(gotRequest.Steps[0].Args, " ") != "-c select 1" {
		t.Fatalf("unexpected steps: %+v", gotRequest.Steps)
	}
	if _, ok := gotRaw["ref"]; ok {
		t.Fatalf("unexpected ref field in transport payload: %+v", gotRaw)
	}
	if _, ok := gotRaw["ref_mode"]; ok {
		t.Fatalf("unexpected ref_mode field in transport payload: %+v", gotRaw)
	}
	if _, ok := gotRaw["ref_keep_worktree"]; ok {
		t.Fatalf("unexpected ref_keep_worktree field in transport payload: %+v", gotRaw)
	}
}

func TestRunRefCleansDetachedWorktreeOnSuccessAndOnBindingFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initRunRefTestRepo(t)
	setTestDirs(t, t.TempDir())
	withWorkingDir(t, filepath.Join(repo, "examples"))
	baseline := gitWorktreeCount(t, repo)

	t.Run("success", func(t *testing.T) {
		server := newRunCaptureServer(t, func(client.RunRequest, map[string]any) {})
		defer server.Close()

		err := Run([]string{
			"--mode", "remote",
			"--endpoint", server.URL,
			"--workspace", repo,
			"run:psql",
			"--ref", parentRef,
			"--ref-mode", "worktree",
			"--instance", "dev",
			"--",
			"-f", "query.sql",
		})
		if err != nil {
			t.Fatalf("Run(success worktree): %v", err)
		}
		if got := gitWorktreeCount(t, repo); got != baseline {
			t.Fatalf("worktree count = %d, want %d", got, baseline)
		}
	})

	t.Run("binding failure", func(t *testing.T) {
		err := Run([]string{
			"--mode", "remote",
			"--endpoint", "http://example.invalid",
			"--workspace", repo,
			"run:psql",
			"--ref", parentRef,
			"--ref-mode", "worktree",
			"--instance", "dev",
			"--",
			"-f", "current-only.sql",
		})
		if err == nil || !strings.Contains(err.Error(), "current-only.sql") {
			t.Fatalf("expected binding failure, got %v", err)
		}
		if got := gitWorktreeCount(t, repo); got != baseline {
			t.Fatalf("worktree count = %d, want %d", got, baseline)
		}
	})
}

func newRunCaptureServer(t *testing.T, onRequest func(client.RunRequest, map[string]any)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var req client.RunRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("unmarshal raw request: %v", err)
		}
		onRequest(req, raw)

		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"exit\",\"ts\":\"2026-05-02T00:00:00Z\",\"exit_code\":0}\n")
	}))
}

func gitWorktreeCount(t *testing.T, repo string) int {
	t.Helper()

	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git worktree list: %v", err)
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	return count
}

func initRunRefTestRepo(t *testing.T) (string, string) {
	t.Helper()

	emptyTemplate := t.TempDir()
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	initCmd := exec.Command("git", "-C", repo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init skipped (need writable temp; run tests outside sandbox): %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")

	writeTestFile(t, repo, filepath.Join("examples", "query.sql"), "select 1;\n")
	writeTestFile(t, repo, filepath.Join("examples", "bench.sql"), "\\set aid 1\n")
	writeTestFile(t, repo, filepath.Join("examples", "chinook", "scripts", "first.sql"), "select 'first';\n")
	writeRunAliasFile(t, filepath.Join(repo, "examples"), filepath.Join("chinook", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - ./scripts/first.sql\n")
	writeRunAliasFile(t, filepath.Join(repo, "examples"), "broken.run.s9s.yaml", "kind: [\n")
	runGit("add", "examples")
	runGit("commit", "-m", "first")

	writeTestFile(t, repo, filepath.Join("examples", "query.sql"), "select 2;\n")
	writeTestFile(t, repo, filepath.Join("examples", "bench.sql"), "\\set aid 2\n")
	writeTestFile(t, repo, filepath.Join("examples", "chinook", "scripts", "second.sql"), "select 'second';\n")
	writeRunAliasFile(t, filepath.Join(repo, "examples"), filepath.Join("chinook", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -f\n  - ./scripts/second.sql\n")
	writeRunAliasFile(t, filepath.Join(repo, "examples"), "current-only.run.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - ./current-only.sql\n")
	writeTestFile(t, repo, filepath.Join("examples", "current-only.sql"), "select 'current';\n")
	writeTestFile(t, repo, filepath.Join("examples", "later", "query.sql"), "select 'later';\n")
	runGit("add", "examples")
	runGit("commit", "-m", "second")

	parentRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD^").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD^: %v", err)
	}
	return repo, strings.TrimSpace(string(parentRef))
}
