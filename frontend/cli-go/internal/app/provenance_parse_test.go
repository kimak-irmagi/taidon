package app

import (
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
)

func TestParsePrepareArgsProvenancePath(t *testing.T) {
	opts, showHelp, err := parsePrepareArgs([]string{"--provenance-path", "artifacts/plan.json", "--image", "img", "--", "-f", "prepare.sql"})
	if err != nil || showHelp {
		t.Fatalf("parsePrepareArgs: err=%v help=%v", err, showHelp)
	}
	if opts.ProvenancePath != "artifacts/plan.json" {
		t.Fatalf("unexpected provenance path: %+v", opts)
	}
}

func TestParsePlanAliasArgsProvenancePath(t *testing.T) {
	opts, showHelp, err := parsePlanAliasArgs([]string{"--provenance-path", "artifacts/plan.json", "chinook"})
	if err != nil || showHelp {
		t.Fatalf("parsePlanAliasArgs: err=%v help=%v", err, showHelp)
	}
	if opts.ProvenancePath != "artifacts/plan.json" {
		t.Fatalf("unexpected provenance path: %+v", opts)
	}
}

func TestRunnerRejectsCompositePrepareProvenance(t *testing.T) {
	cwd := t.TempDir()
	err := runWithParsedCommands(t, cli.GlobalOptions{}, []cli.Command{
		{Name: "prepare:psql", Args: []string{"--provenance-path", "artifacts/prepare.json", "--image", "img", "--", "-c", "select 1"}},
		{Name: "run:psql", Args: []string{"--", "-c", "select 1"}},
	}, func(deps *runnerDeps) {
		deps.getwd = func() (string, error) {
			return cwd, nil
		}
		deps.resolveCommandContext = func(string, cli.GlobalOptions) (commandContext, error) {
			return testCommandContext(cwd, "human", false), nil
		}
		deps.prepareResult = func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error) {
			t.Fatal("prepareResult should not run for composite provenance")
			return client.PrepareJobResult{}, false, nil
		}
	})
	if err == nil || !strings.Contains(err.Error(), "provenance is not supported with composite prepare ... run") {
		t.Fatalf("expected composite provenance error, got %v", err)
	}
}
