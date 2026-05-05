package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
)

func newTestRunner(t *testing.T, configure func(*runnerDeps)) runner {
	t.Helper()

	deps := runnerDeps{
		stdout: io.Discard,
		stderr: io.Discard,
	}
	if configure != nil {
		configure(&deps)
	}
	return newRunner(deps)
}

func runWithParsedCommands(t *testing.T, opts cli.GlobalOptions, commands []cli.Command, configure func(*runnerDeps)) error {
	t.Helper()

	return newTestRunner(t, func(deps *runnerDeps) {
		deps.parseArgs = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return opts, commands, nil
		}
		if configure != nil {
			configure(deps)
		}
	}).run(nil)
}

func testCommandContext(cwd string, output string, verbose bool) commandContext {
	return commandContext{
		cwd:           cwd,
		workspaceRoot: cwd,
		output:        output,
		verbose:       verbose,
		cfgResult:     config.LoadedConfig{},
	}
}

func TestRunnerUsesParserAndReturnsHelpWithoutDispatch(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	getwdCalled := false

	err := newTestRunner(t, func(deps *runnerDeps) {
		deps.stdout = &stdout
		deps.stderr = &stderr
		deps.parseArgs = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{}, nil, cli.ErrHelp
		}
		deps.getwd = func() (string, error) {
			getwdCalled = true
			return "", nil
		}
		deps.runInit = func(io.Writer, string, string, []string, bool) error {
			t.Fatal("runInit should not be called on --help")
			return nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			t.Fatal("resolveCommandContext should not be called on --help")
			return commandContext{}, nil
		}
	}).run([]string{"--help"})
	if err != nil {
		t.Fatalf("runner.run: %v", err)
	}
	if getwdCalled {
		t.Fatal("getwd should not be called on --help")
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunnerSkipsCommandContextForInitAndDiff(t *testing.T) {
	t.Run("init", func(t *testing.T) {
		cwd := t.TempDir()
		resolveCalls := 0
		runInitCalls := 0

		err := runWithParsedCommands(t, cli.GlobalOptions{Workspace: "workspace", Verbose: true}, []cli.Command{{Name: "init", Args: []string{"--dry-run"}}}, func(deps *runnerDeps) {
			deps.getwd = func() (string, error) {
				return cwd, nil
			}
			deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
				resolveCalls++
				return commandContext{}, nil
			}
			deps.runInit = func(stdout io.Writer, gotCwd, globalWorkspace string, args []string, verbose bool) error {
				runInitCalls++
				if stdout != deps.stdout {
					t.Fatal("runInit received unexpected stdout writer")
				}
				if gotCwd != cwd {
					t.Fatalf("cwd = %q, want %q", gotCwd, cwd)
				}
				if globalWorkspace != "workspace" {
					t.Fatalf("workspace = %q, want %q", globalWorkspace, "workspace")
				}
				if !verbose {
					t.Fatal("verbose flag was not forwarded")
				}
				if got := strings.Join(args, " "); got != "--dry-run" {
					t.Fatalf("args = %q, want %q", got, "--dry-run")
				}
				return nil
			}
		})
		if err != nil {
			t.Fatalf("runner.run: %v", err)
		}
		if resolveCalls != 0 {
			t.Fatalf("resolveCommandContext calls = %d, want 0", resolveCalls)
		}
		if runInitCalls != 1 {
			t.Fatalf("runInit calls = %d, want 1", runInitCalls)
		}
	})

	t.Run("diff", func(t *testing.T) {
		cwd := t.TempDir()
		resolveCalls := 0
		runDiffCalls := 0

		err := runWithParsedCommands(t, cli.GlobalOptions{Output: "json", Verbose: true}, []cli.Command{{Name: "diff", Args: []string{"--from-path", "a", "--to-path", "b", "--", "plan:psql"}}}, func(deps *runnerDeps) {
			deps.getwd = func() (string, error) {
				return cwd, nil
			}
			deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
				resolveCalls++
				return commandContext{}, nil
			}
			deps.runDiff = func(stdout, stderr io.Writer, gotCwd string, args []string, output string, verbose bool) error {
				runDiffCalls++
				if stdout != deps.stdout || stderr != deps.stderr {
					t.Fatal("runDiff received unexpected writers")
				}
				if gotCwd != cwd {
					t.Fatalf("cwd = %q, want %q", gotCwd, cwd)
				}
				if output != "json" {
					t.Fatalf("output = %q, want %q", output, "json")
				}
				if !verbose {
					t.Fatal("verbose flag was not forwarded")
				}
				if len(args) == 0 {
					t.Fatal("diff args should be forwarded")
				}
				return nil
			}
		})
		if err != nil {
			t.Fatalf("runner.run: %v", err)
		}
		if resolveCalls != 0 {
			t.Fatalf("resolveCommandContext calls = %d, want 0", resolveCalls)
		}
		if runDiffCalls != 1 {
			t.Fatalf("runDiff calls = %d, want 1", runDiffCalls)
		}
	})
}

func TestRunnerBuildsCommandContextOnceForContextualCommands(t *testing.T) {
	cwd := t.TempDir()
	resolveCalls := 0
	runCalls := 0
	cleanupCalls := 0

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare:psql", Args: []string{"--image", "img"}},
		{Name: "run:psql", Args: []string{"--", "-c", "select 1"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(gotCwd string, opts cli.GlobalOptions) (commandContext, error) {
			resolveCalls++
			if gotCwd != cwd {
				t.Fatalf("cwd = %q, want %q", gotCwd, cwd)
			}
			return testCommandContext(gotCwd, "human", false), nil
		}
		deps.prepareResult = func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error) {
			return client.PrepareJobResult{InstanceID: "inst"}, false, nil
		}
		deps.runRun = func(io.Writer, io.Writer, cli.RunOptions, string, []string, string, string) error {
			runCalls++
			return nil
		}
		deps.cleanupPreparedInstance = func(context.Context, io.Writer, cli.RunOptions, string, bool) {
			cleanupCalls++
		}
	})
	if err != nil {
		t.Fatalf("runner.run: %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCommandContext calls = %d, want 1", resolveCalls)
	}
	if runCalls != 1 {
		t.Fatalf("runRun calls = %d, want 1", runCalls)
	}
	if cleanupCalls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", cleanupCalls)
	}
}

func TestRunnerRejectsCompositePrepareRefBeforeRunDispatch(t *testing.T) {
	cwd := t.TempDir()
	runCalled := false

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare", Args: []string{"--ref", "HEAD", "examples/chinook"}},
		{Name: "run:psql", Args: []string{"--", "-c", "select 1"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", false), nil
		}
		deps.runRun = func(io.Writer, io.Writer, cli.RunOptions, string, []string, string, string) error {
			runCalled = true
			return nil
		}
		deps.runPrepare = func(io.Writer, io.Writer, cli.PrepareOptions, config.LoadedConfig, string, string, []string) error {
			t.Fatal("runPrepare should not be called after composite --ref rejection")
			return nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "prepare --ref does not support composite run yet") {
		t.Fatalf("expected composite --ref error, got %v", err)
	}
	if runCalled {
		t.Fatal("runRun should not be called after composite --ref rejection")
	}
}

func TestRunnerRejectsCompositePrepareRefEqualsBeforeRunDispatch(t *testing.T) {
	cwd := t.TempDir()
	runCalled := false

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare", Args: []string{"--ref=HEAD", "examples/chinook"}},
		{Name: "run:psql", Args: []string{"--", "-c", "select 1"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", false), nil
		}
		deps.runRun = func(io.Writer, io.Writer, cli.RunOptions, string, []string, string, string) error {
			runCalled = true
			return nil
		}
		deps.runPrepare = func(io.Writer, io.Writer, cli.PrepareOptions, config.LoadedConfig, string, string, []string) error {
			t.Fatal("runPrepare should not be called after composite --ref rejection")
			return nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "prepare --ref does not support composite run yet") {
		t.Fatalf("expected composite --ref error, got %v", err)
	}
	if runCalled {
		t.Fatal("runRun should not be called after composite --ref rejection")
	}
}

func TestRunnerRejectsMalformedCompositeRunRefEqualsBeforePrepareDispatch(t *testing.T) {
	cwd := t.TempDir()
	prepareCalled := false

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare:psql", Args: []string{"--image", "img", "--", "-c", "select 1"}},
		{Name: "run", Args: []string{"--ref=HEAD", "--bad", "smoke"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", false), nil
		}
		deps.prepareResult = func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error) {
			prepareCalled = true
			return client.PrepareJobResult{}, false, nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "run --ref does not support composite prepare ... run yet") {
		t.Fatalf("expected composite run --ref error, got %v", err)
	}
	if prepareCalled {
		t.Fatal("prepareResult should not be called after composite run --ref rejection")
	}
}

func TestRunnerRoutesAliasAndDiscoverThroughInjectedHandlers(t *testing.T) {
	t.Run("alias", func(t *testing.T) {
		cwd := t.TempDir()
		aliasCalls := 0
		discoverCalls := 0

		err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{{Name: "alias", Args: []string{"list"}}}, func(deps *runnerDeps) {
			deps.getwd = func() (string, error) {
				return cwd, nil
			}
			deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
				return testCommandContext(cwd, "human", false), nil
			}
			deps.runAlias = func(io.Writer, commandContext, []string) error {
				aliasCalls++
				return nil
			}
			deps.runDiscover = func(io.Writer, io.Writer, commandContext, []string, string) error {
				discoverCalls++
				return nil
			}
		})
		if err != nil {
			t.Fatalf("runner.run: %v", err)
		}
		if aliasCalls != 1 {
			t.Fatalf("runAlias calls = %d, want 1", aliasCalls)
		}
		if discoverCalls != 0 {
			t.Fatalf("runDiscover calls = %d, want 0", discoverCalls)
		}
	})

	t.Run("discover", func(t *testing.T) {
		cwd := t.TempDir()
		aliasCalls := 0
		discoverCalls := 0

		err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{{Name: "discover", Args: []string{"--aliases"}}}, func(deps *runnerDeps) {
			deps.getwd = func() (string, error) {
				return cwd, nil
			}
			deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
				return testCommandContext(cwd, "json", false), nil
			}
			deps.runAlias = func(io.Writer, commandContext, []string) error {
				aliasCalls++
				return nil
			}
			deps.runDiscover = func(io.Writer, io.Writer, commandContext, []string, string) error {
				discoverCalls++
				return nil
			}
		})
		if err != nil {
			t.Fatalf("runner.run: %v", err)
		}
		if discoverCalls != 1 {
			t.Fatalf("runDiscover calls = %d, want 1", discoverCalls)
		}
		if aliasCalls != 0 {
			t.Fatalf("runAlias calls = %d, want 0", aliasCalls)
		}
	})
}

func TestRunnerCleansPreparedInstanceThroughInjectedCleanup(t *testing.T) {
	cwd := t.TempDir()
	runCalls := 0
	cleanupCalls := 0
	var cleanedInstanceID string
	var cleanedRunOpts cli.RunOptions
	var cleanedVerbose bool

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare:psql", Args: []string{"--image", "img"}},
		{Name: "run:psql", Args: []string{"--", "-c", "select 1"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", true), nil
		}
		deps.prepareResult = func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error) {
			return client.PrepareJobResult{InstanceID: "inst-123"}, false, nil
		}
		deps.runRun = func(_ io.Writer, _ io.Writer, runOpts cli.RunOptions, kind string, args []string, workspaceRoot string, gotCwd string) error {
			runCalls++
			if runOpts.InstanceRef != "inst-123" {
				t.Fatalf("instance ref = %q, want %q", runOpts.InstanceRef, "inst-123")
			}
			if kind != "psql" {
				t.Fatalf("kind = %q, want %q", kind, "psql")
			}
			if gotCwd != cwd || workspaceRoot != cwd {
				t.Fatalf("unexpected run paths: workspaceRoot=%q cwd=%q", workspaceRoot, gotCwd)
			}
			if len(args) == 0 {
				t.Fatal("run args should be forwarded")
			}
			return nil
		}
		deps.cleanupPreparedInstance = func(_ context.Context, _ io.Writer, runOpts cli.RunOptions, instanceID string, verbose bool) {
			cleanupCalls++
			cleanedInstanceID = instanceID
			cleanedRunOpts = runOpts
			cleanedVerbose = verbose
		}
	})
	if err != nil {
		t.Fatalf("runner.run: %v", err)
	}
	if runCalls != 1 {
		t.Fatalf("runRun calls = %d, want 1", runCalls)
	}
	if cleanupCalls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", cleanupCalls)
	}
	if cleanedInstanceID != "inst-123" {
		t.Fatalf("cleanup instanceID = %q, want %q", cleanedInstanceID, "inst-123")
	}
	if cleanedRunOpts.InstanceRef != "inst-123" {
		t.Fatalf("cleanup run opts instance ref = %q, want %q", cleanedRunOpts.InstanceRef, "inst-123")
	}
	if !cleanedVerbose {
		t.Fatal("cleanup should receive verbose=true from command context")
	}
}

func TestRunUsesDefaultRunnerDependencies(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		_ = w.Close()
		os.Stdout = oldStdout
	})

	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "Usage:") {
		t.Fatalf("stdout = %q, want usage", string(data))
	}
}
