package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/cli/runkind"
)

type runArgs struct {
	InstanceRef string
	Command     string
	Args        []string
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
			return parseRunCommand(opts, args[i+1:])
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
		case strings.HasPrefix(arg, "--instance="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--instance="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			opts.InstanceRef = value
		default:
			return parseRunCommand(opts, args[i:])
		}
	}
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

func runRun(stdout io.Writer, stderr io.Writer, runOpts cli.RunOptions, kind string, args []string, workspaceRoot string, cwd string) error {
	parsed, showHelp, err := parseRunArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintRunUsage(stdout)
		return nil
	}

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

	runOpts.Kind = kind
	runOpts.InstanceRef = instanceRef
	runOpts.Command = strings.TrimSpace(parsed.Command)
	runArgs := append([]string{}, parsed.Args...)
	switch kind {
	case runkind.KindPsql:
		steps, err := buildPsqlRunSteps(runArgs, workspaceRoot, cwd, os.Stdin)
		if err != nil {
			return err
		}
		runOpts.Steps = steps
		runOpts.Args = nil
		runOpts.Stdin = nil
	case runkind.KindPgbench:
		normalizedArgs, stdinValue, err := materializePgbenchRunArgs(runArgs, workspaceRoot, cwd, os.Stdin)
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
	var shared []string
	var steps []cli.RunStep
	stdinStep := -1

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "--command":
			if i+1 >= len(args) {
				return nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			cmd := args[i+1]
			i++
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", cmd)
			steps = append(steps, cli.RunStep{Args: stepArgs})
		case strings.HasPrefix(arg, "--command="):
			cmd := strings.TrimPrefix(arg, "--command=")
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", cmd)
			steps = append(steps, cli.RunStep{Args: stepArgs})
		case strings.HasPrefix(arg, "-c") && len(arg) > 2:
			cmd := arg[2:]
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", cmd)
			steps = append(steps, cli.RunStep{Args: stepArgs})
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			value := args[i+1]
			i++
			step, isStdin, err := buildFileStep(shared, value, workspaceRoot, cwd)
			if err != nil {
				return nil, err
			}
			if isStdin {
				if stdinStep != -1 {
					return nil, ExitErrorf(2, "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			step, isStdin, err := buildFileStep(shared, value, workspaceRoot, cwd)
			if err != nil {
				return nil, err
			}
			if isStdin {
				if stdinStep != -1 {
					return nil, ExitErrorf(2, "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			step, isStdin, err := buildFileStep(shared, value, workspaceRoot, cwd)
			if err != nil {
				return nil, err
			}
			if isStdin {
				if stdinStep != -1 {
					return nil, ExitErrorf(2, "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
		default:
			shared = append(shared, arg)
		}
	}

	if stdinStep != -1 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		text := string(data)
		steps[stdinStep].Stdin = &text
	}

	if len(steps) == 0 {
		return []cli.RunStep{{Args: shared}}, nil
	}
	return steps, nil
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
	normalized := make([]string, 0, len(args))
	var source *pgbenchFileSource

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			value := args[i+1]
			i++
			rewritten, nextSource, err := rewritePgbenchFileArg(value, workspaceRoot, cwd)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, ExitErrorf(2, "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, arg, rewritten)
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			rewritten, nextSource, err := rewritePgbenchFileArg(value, workspaceRoot, cwd)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, ExitErrorf(2, "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, "--file="+rewritten)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			rewritten, nextSource, err := rewritePgbenchFileArg(value, workspaceRoot, cwd)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, ExitErrorf(2, "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, "-f"+rewritten)
		default:
			normalized = append(normalized, arg)
		}
	}

	if source == nil {
		return normalized, nil, nil
	}
	text, err := readPgbenchFileSource(*source, stdin)
	if err != nil {
		return nil, nil, err
	}
	return normalized, &text, nil
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
	idx := strings.LastIndex(value, "@")
	if idx <= 0 || idx >= len(value)-1 {
		return value, ""
	}
	if _, err := strconv.ParseUint(value[idx+1:], 10, 32); err != nil {
		return value, ""
	}
	return value[:idx], value[idx:]
}
