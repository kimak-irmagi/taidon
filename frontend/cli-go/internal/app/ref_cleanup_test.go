package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

func TestBindPreparePsqlInputsCleansWorktreeOnError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, _ := initPrepareRefTestRepo(t)
	cwd := filepath.Join(repo, "examples")
	before := countGitWorktrees(t, repo)

	_, err := bindPreparePsqlInputs(
		cli.PrepareOptions{},
		repo,
		cwd,
		prepareArgs{Ref: "HEAD", RefMode: "worktree", PsqlArgs: []string{"-f", "../../outside.sql"}},
		nil,
		strings.NewReader(""),
	)
	if err == nil {
		t.Fatal("expected bindPreparePsqlInputs error")
	}

	after := countGitWorktrees(t, repo)
	if after != before {
		t.Fatalf("worktree count = %d, want %d after cleanup", after, before)
	}
}

func TestBindPrepareLiquibaseInputsCleansWorktreeOnError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, _ := initPrepareRefTestRepo(t)
	cwd := filepath.Join(repo, "examples")
	before := countGitWorktrees(t, repo)

	_, err := bindPrepareLiquibaseInputs(
		cli.PrepareOptions{},
		repo,
		cwd,
		prepareArgs{Ref: "HEAD", RefMode: "worktree", PsqlArgs: []string{"update", "--changelog-file", "../../outside.xml"}},
		nil,
		"",
		"",
		false,
	)
	if err == nil {
		t.Fatal("expected bindPrepareLiquibaseInputs error")
	}

	after := countGitWorktrees(t, repo)
	if after != before {
		t.Fatalf("worktree count = %d, want %d after cleanup", after, before)
	}
}

func TestPrepareResultParsedPropagatesCleanupError(t *testing.T) {
	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{
			PsqlArgs: []string{"-c", "select 1"},
			cleanup: func() error {
				return errors.New("cleanup failed")
			},
		}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(context.Context, cli.PrepareOptions) (client.PrepareJobResult, error) {
		return client.PrepareJobResult{DSN: "dsn"}, nil
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	_, handled, err := prepareResultParsed(
		stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard},
		cli.PrepareOptions{},
		config.LoadedConfig{},
		"",
		"",
		prepareArgs{Image: "image", PsqlArgs: []string{"-c", "select 1"}, Watch: true},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if handled {
		t.Fatalf("expected handled=false on cleanup error")
	}
}

func TestPrepareResultParsedDisablesControlPromptForRef(t *testing.T) {
	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{PsqlArgs: []string{"-c", "select 1"}}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(_ context.Context, opts cli.PrepareOptions) (client.PrepareJobResult, error) {
		if !opts.DisableControlPrompt {
			t.Fatalf("expected DisableControlPrompt for ref-backed prepare")
		}
		return client.PrepareJobResult{DSN: "dsn"}, nil
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	_, handled, err := prepareResultParsed(
		stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard},
		cli.PrepareOptions{},
		config.LoadedConfig{},
		"",
		"",
		prepareArgs{Image: "image", PsqlArgs: []string{"-c", "select 1"}, Watch: true},
		&refctx.Context{},
	)
	if err != nil {
		t.Fatalf("prepareResultParsed: %v", err)
	}
	if handled {
		t.Fatalf("expected handled=false")
	}
}

func TestRunPlanKindParsedWithPathModePropagatesCleanupError(t *testing.T) {
	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{
			PsqlArgs: []string{"-c", "select 1"},
			cleanup: func() error {
				return errors.New("cleanup failed")
			},
		}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevRunPlan := runPlanFn
	runPlanFn = func(context.Context, cli.PrepareOptions) (cli.PlanResult, error) {
		return cli.PlanResult{
			PrepareKind:           "psql",
			ImageID:               "image",
			PrepareArgsNormalized: "-c select 1",
			Tasks:                 []client.PlanTask{{TaskID: "plan", Type: "plan", PlannerKind: "psql"}},
		}, nil
	}
	t.Cleanup(func() { runPlanFn = prevRunPlan })

	err := runPlanKindParsedWithPathMode(
		&bytes.Buffer{},
		io.Discard,
		cli.PrepareOptions{},
		config.LoadedConfig{},
		"",
		"",
		prepareArgs{Image: "image", PsqlArgs: []string{"-c", "select 1"}},
		nil,
		"json",
		"psql",
		true,
	)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected cleanup error, got %v", err)
	}
}

func countGitWorktrees(t *testing.T, repo string) int {
	t.Helper()

	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree list: %v\n%s", err, out)
	}

	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	return count
}
