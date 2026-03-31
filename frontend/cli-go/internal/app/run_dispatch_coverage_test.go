package app

import (
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
)

func TestRunCombinedCommandDispatchErrorsCoverage(t *testing.T) {
	t.Run("diff", func(t *testing.T) {
		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{}, []cli.Command{
				{Name: "diff"},
				{Name: "status"},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "diff cannot be combined with other commands") {
			t.Fatalf("expected diff combine error, got %v", err)
		}
	})

	t.Run("alias", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		workspace := writeAliasWorkspace(t, temp, "http://127.0.0.1:1")
		withWorkingDir(t, workspace)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{}, []cli.Command{
				{Name: "alias", Args: []string{"--help"}},
				{Name: "status"},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "alias cannot be combined with other commands") {
			t.Fatalf("expected alias combine error, got %v", err)
		}
	})

	t.Run("discover", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		workspace := writeAliasWorkspace(t, temp, "http://127.0.0.1:1")
		withWorkingDir(t, workspace)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{}, []cli.Command{
				{Name: "discover"},
				{Name: "status"},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "discover cannot be combined with other commands") {
			t.Fatalf("expected discover combine error, got %v", err)
		}
	})

	t.Run("plan", func(t *testing.T) {
		temp := t.TempDir()
		setTestDirs(t, temp)
		workspace := writeAliasWorkspace(t, temp, "http://127.0.0.1:1")
		withWorkingDir(t, workspace)

		prev := parseArgsFn
		parseArgsFn = func([]string) (cli.GlobalOptions, []cli.Command, error) {
			return cli.GlobalOptions{}, []cli.Command{
				{Name: "plan", Args: []string{"examples/chinook"}},
				{Name: "status"},
			}, nil
		}
		t.Cleanup(func() { parseArgsFn = prev })

		if err := Run(nil); err == nil || !strings.Contains(err.Error(), "plan cannot be combined with other commands") {
			t.Fatalf("expected plan combine error, got %v", err)
		}
	})
}
