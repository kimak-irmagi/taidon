package app

import (
	"io"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/config"
)

func TestRunnerRoutesCacheThroughInjectedHandler(t *testing.T) {
	cwd := t.TempDir()
	cacheCalls := 0

	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{{Name: "cache", Args: []string{"explain", "prepare", "chinook"}}}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "json", false), nil
		}
		deps.runCache = func(stdout io.Writer, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string) error {
			cacheCalls++
			if output != "json" {
				t.Fatalf("output = %q, want %q", output, "json")
			}
			if workspaceRoot != cwd {
				t.Fatalf("expected matching workspace/cwd in test context, got %q %q", workspaceRoot, cwd)
			}
			if len(args) != 3 || args[0] != "explain" {
				t.Fatalf("unexpected cache args: %+v", args)
			}
			return nil
		}
	})
	if err != nil {
		t.Fatalf("runner.run: %v", err)
	}
	if cacheCalls != 1 {
		t.Fatalf("runCache calls = %d, want 1", cacheCalls)
	}
}
