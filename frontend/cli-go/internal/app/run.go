package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/cli/runkind"
	"github.com/sqlrs/cli/internal/inputset"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
	"github.com/sqlrs/cli/internal/refctx"
)

type runArgs struct {
	InstanceRef     string
	Ref             string
	RefMode         string
	RefKeepWorktree bool
	Command         string
	Args            []string
}

const pgbenchStdinPath = "/dev/stdin"

func parseRunArgs(args []string) (runArgs, bool, error) {
	var opts runArgs
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return opts, false, err
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return finalizeRunCommand(parseRunCommand(opts, args[i+1:]))
		case arg == "--help" || arg == "-h":
			return opts, true, nil
		case arg == "--instance":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			opts.InstanceRef = value
			i++
		case arg == "--ref":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			opts.Ref = value
			i++
		case arg == "--ref-mode":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref-mode")
			}
			opts.RefMode = strings.TrimSpace(args[i+1])
			i++
		case arg == "--ref-keep-worktree":
			opts.RefKeepWorktree = true
		case strings.HasPrefix(arg, "--instance="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--instance="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			opts.InstanceRef = value
		case strings.HasPrefix(arg, "--ref="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--ref="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			opts.Ref = value
		case strings.HasPrefix(arg, "--ref-mode="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--ref-mode="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --ref-mode")
			}
			opts.RefMode = value
		default:
			return finalizeRunCommand(parseRunCommand(opts, args[i:]))
		}
	}
	mode, err := normalizeRefMode(opts.Ref, opts.RefMode, opts.RefKeepWorktree)
	if err != nil {
		return opts, false, err
	}
	opts.RefMode = mode
	return opts, false, nil
}

func parseRunCommand(opts runArgs, args []string) (runArgs, bool, error) {
	if len(args) == 0 {
		return opts, false, nil
	}
	if strings.HasPrefix(args[0], "-") {
		opts.Args = append([]string{}, args...)
		return opts, false, nil
	}
	opts.Command = args[0]
	opts.Args = append([]string{}, args[1:]...)
	return opts, false, nil
}

func finalizeRunCommand(opts runArgs, showHelp bool, err error) (runArgs, bool, error) {
	if err != nil || showHelp {
		return opts, showHelp, err
	}
	mode, modeErr := normalizeRefMode(opts.Ref, opts.RefMode, opts.RefKeepWorktree)
	if modeErr != nil {
		return opts, false, modeErr
	}
	opts.RefMode = mode
	return opts, false, nil
}

func runRun(stdout io.Writer, stderr io.Writer, runOpts cli.RunOptions, kind string, args []string, workspaceRoot string, cwd string) error {
	parsed, showHelp, err := parseRunArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintRunUsage(stdout)
		return nil
	}
	return runRunParsed(stdout, stderr, runOpts, kind, parsed, workspaceRoot, cwd, nil)
}

// runRunParsed executes one standalone run stage after applying the optional
// git-ref binding flow described in docs/architecture/run-ref-flow.md.
func runRunParsed(stdout io.Writer, stderr io.Writer, runOpts cli.RunOptions, kind string, parsed runArgs, workspaceRoot string, cwd string, ref *refctx.Context) (err error) {
	if parsed.InstanceRef != "" && runOpts.InstanceRef != "" {
		return fmt.Errorf("instance is already selected by a preceding prepare")
	}
	instanceRef := parsed.InstanceRef
	if instanceRef == "" {
		instanceRef = runOpts.InstanceRef
	}
	if strings.TrimSpace(instanceRef) == "" {
		return ExitErrorf(2, "Missing instance (use --instance or run after prepare)")
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if !runkind.IsKnown(kind) {
		return ExitErrorf(2, "Unknown run kind: %s", kind)
	}
	if runkind.HasConnectionArgs(kind, parsed.Args) {
		return ExitErrorf(2, "Conflicting connection arguments for run:%s", kind)
	}

	actualRef, cleanup, err := resolveRunBindingContext(workspaceRoot, cwd, parsed, ref)
	if err != nil {
		return err
	}
	defer func() {
		err = finishPrepareCleanup(err, cleanup)
	}()
	bindCWD := cwd
	if ref == nil && actualRef != nil && strings.TrimSpace(actualRef.BaseDir) != "" {
		bindCWD = actualRef.BaseDir
	}

	runOpts.Kind = kind
	runOpts.InstanceRef = instanceRef
	runOpts.Command = strings.TrimSpace(parsed.Command)
	runArgs := append([]string{}, parsed.Args...)
	switch kind {
	case runkind.KindPsql:
		steps, err := buildPsqlRunStepsForContext(runArgs, workspaceRoot, bindCWD, actualRef, os.Stdin)
		if err != nil {
			return err
		}
		runOpts.Steps = steps
		runOpts.Args = nil
		runOpts.Stdin = nil
	case runkind.KindPgbench:
		normalizedArgs, stdinValue, err := materializePgbenchRunArgsForContext(runArgs, workspaceRoot, bindCWD, actualRef, os.Stdin)
		if err != nil {
			return err
		}
		runOpts.Args = normalizedArgs
		runOpts.Stdin = stdinValue
		runOpts.Steps = nil
	default:
		runOpts.Args = runArgs
		runOpts.Stdin = nil
		runOpts.Steps = nil
	}

	result, err := cli.RunRun(context.Background(), runOpts, stdout, stderr)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return ExitErrorf(result.ExitCode, "command failed")
	}
	return nil
}

func buildPsqlRunSteps(args []string, workspaceRoot string, cwd string, stdin io.Reader) ([]cli.RunStep, error) {
	return buildPsqlRunStepsWithResolver(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), stdin, inputset.OSFileSystem{})
}

func buildPsqlRunStepsForContext(args []string, workspaceRoot string, cwd string, ctx *refctx.Context, stdin io.Reader) ([]cli.RunStep, error) {
	if ctx == nil {
		return buildPsqlRunSteps(args, workspaceRoot, cwd, stdin)
	}
	root := strings.TrimSpace(ctx.WorkspaceRoot)
	if root == "" {
		root = strings.TrimSpace(workspaceRoot)
	}
	baseDir := strings.TrimSpace(cwd)
	if baseDir == "" {
		baseDir = ctx.BaseDir
	}
	return buildPsqlRunStepsWithResolver(args, inputset.NewWorkspaceResolver(root, baseDir, nil), stdin, ctx.FileSystem)
}

func buildPsqlRunStepsWithResolver(args []string, resolver inputset.Resolver, stdin io.Reader, fs inputset.FileSystem) ([]cli.RunStep, error) {
	steps, err := inputpsql.BuildRunSteps(
		args,
		resolver,
		stdin,
		fs,
	)
	if err != nil {
		return nil, wrapInputsetError(err)
	}
	out := make([]cli.RunStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, cli.RunStep{Args: step.Args, Stdin: step.Stdin})
	}
	return out, nil
}

func buildFileStep(shared []string, value string, workspaceRoot string, cwd string) (cli.RunStep, bool, error) {
	if strings.TrimSpace(value) == "" {
		return cli.RunStep{}, false, ExitErrorf(2, "Missing value for --file")
	}
	if value == "-" {
		stepArgs := append([]string{}, shared...)
		stepArgs = append(stepArgs, "-f", "-")
		return cli.RunStep{Args: stepArgs}, true, nil
	}
	path, _, err := normalizeFilePath(value, workspaceRoot, cwd, nil)
	if err != nil {
		return cli.RunStep{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cli.RunStep{}, false, err
	}
	text := string(data)
	stepArgs := append([]string{}, shared...)
	stepArgs = append(stepArgs, "-f", "-")
	return cli.RunStep{Args: stepArgs, Stdin: &text}, false, nil
}

type pgbenchFileSource struct {
	Path      string
	UsesStdin bool
}

func materializePgbenchRunArgs(args []string, workspaceRoot string, cwd string, stdin io.Reader) ([]string, *string, error) {
	return materializePgbenchRunArgsWithResolver(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), stdin, inputset.OSFileSystem{})
}

func materializePgbenchRunArgsForContext(args []string, workspaceRoot string, cwd string, ctx *refctx.Context, stdin io.Reader) ([]string, *string, error) {
	if ctx == nil {
		return materializePgbenchRunArgs(args, workspaceRoot, cwd, stdin)
	}
	root := strings.TrimSpace(ctx.WorkspaceRoot)
	if root == "" {
		root = strings.TrimSpace(workspaceRoot)
	}
	baseDir := strings.TrimSpace(cwd)
	if baseDir == "" {
		baseDir = ctx.BaseDir
	}
	return materializePgbenchRunArgsWithResolver(args, inputset.NewWorkspaceResolver(root, baseDir, nil), stdin, ctx.FileSystem)
}

func materializePgbenchRunArgsWithResolver(args []string, resolver inputset.Resolver, stdin io.Reader, fs inputset.FileSystem) ([]string, *string, error) {
	normalized, stdinValue, err := inputpgbench.MaterializeArgs(
		args,
		resolver,
		stdin,
		fs,
	)
	if err != nil {
		return nil, nil, wrapInputsetError(err)
	}
	return normalized, stdinValue, nil
}

func rewritePgbenchFileArg(value string, workspaceRoot string, cwd string) (string, *pgbenchFileSource, error) {
	path, weightSuffix := splitPgbenchFileArgValue(value)
	if strings.TrimSpace(path) == "" {
		return "", nil, ExitErrorf(2, "Missing value for --file")
	}
	if path == "-" || path == pgbenchStdinPath {
		return pgbenchStdinPath + weightSuffix, &pgbenchFileSource{UsesStdin: true}, nil
	}
	normalizedPath, _, err := normalizeFilePath(path, workspaceRoot, cwd, nil)
	if err != nil {
		return "", nil, err
	}
	return pgbenchStdinPath + weightSuffix, &pgbenchFileSource{Path: normalizedPath}, nil
}

func readPgbenchFileSource(source pgbenchFileSource, stdin io.Reader) (string, error) {
	if source.UsesStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile(source.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func splitPgbenchFileArgValue(value string) (string, string) {
	return inputset.SplitPgbenchFileArgValue(value)
}
