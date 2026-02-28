package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/config"
)

type prepareArgs struct {
	Image          string
	PsqlArgs       []string
	Watch          bool
	WatchSpecified bool
}

type stdoutAndErr struct {
	stdout io.Writer
	stderr io.Writer
}

func parsePrepareArgs(args []string) (prepareArgs, bool, error) {
	opts := prepareArgs{Watch: true}
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return opts, false, err
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				opts.PsqlArgs = append(opts.PsqlArgs, args[i+1:]...)
			}
			return opts, false, nil
		}
		switch {
		case arg == "--help" || arg == "-h":
			return opts, true, nil
		case arg == "--watch":
			opts.Watch = true
			opts.WatchSpecified = true
		case arg == "--no-watch":
			opts.Watch = false
			opts.WatchSpecified = true
		case arg == "--image":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --image")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --image")
			}
			opts.Image = value
			i++
		case strings.HasPrefix(arg, "--image="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--image="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --image")
			}
			opts.Image = value
		default:
			opts.PsqlArgs = append(opts.PsqlArgs, args[i:]...)
			return opts, false, nil
		}
	}
	return opts, false, nil
}

func runPrepare(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string) error {
	result, handled, err := prepareResult(stdoutAndErr{stdout: stdout, stderr: stderr}, runOpts, cfg, workspaceRoot, cwd, args)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	fmt.Fprintf(stdout, "DSN=%s\n", result.DSN)
	return nil
}

func runPrepareLiquibase(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string) error {
	result, handled, err := prepareResultLiquibase(stdoutAndErr{stdout: stdout, stderr: stderr}, runOpts, cfg, workspaceRoot, cwd, args)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	fmt.Fprintf(stdout, "DSN=%s\n", result.DSN)
	return nil
}

func prepareResult(w stdoutAndErr, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string) (client.PrepareJobResult, bool, error) {
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	if showHelp {
		cli.PrintPrepareUsage(w.stdout)
		return client.PrepareJobResult{}, true, nil
	}

	imageID, source, err := resolvePrepareImage(parsed.Image, cfg)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	if imageID == "" {
		return client.PrepareJobResult{}, false, ExitErrorf(2, "Missing base image id (set --image or dbms.image)")
	}
	if runOpts.Verbose {
		fmt.Fprint(w.stderr, formatImageSource(imageID, source))
	}

	psqlArgs, stdin, err := normalizePsqlArgs(parsed.PsqlArgs, workspaceRoot, cwd, os.Stdin, buildPathConverter(runOpts))
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}

	runOpts.ImageID = imageID
	runOpts.PsqlArgs = psqlArgs
	runOpts.Stdin = stdin
	runOpts.PrepareKind = "psql"

	if !parsed.Watch {
		accepted, err := cli.SubmitPrepare(context.Background(), runOpts)
		if err != nil {
			return client.PrepareJobResult{}, false, err
		}
		printPrepareJobRefs(w.stdout, accepted)
		if runOpts.CompositeRun {
			printRunSkipped(w.stdout, "prepare_not_watched")
		}
		return client.PrepareJobResult{}, true, nil
	}

	result, err := cli.RunPrepare(context.Background(), runOpts)
	if err != nil {
		var detached *cli.PrepareDetachedError
		if errors.As(err, &detached) {
			accepted := client.PrepareJobAccepted{
				JobID:     detached.JobID,
				StatusURL: "/v1/prepare-jobs/" + detached.JobID,
				EventsURL: "/v1/prepare-jobs/" + detached.JobID + "/events",
			}
			printPrepareJobRefs(w.stdout, accepted)
			if runOpts.CompositeRun {
				printRunSkipped(w.stdout, "prepare_detached")
			}
			return client.PrepareJobResult{}, true, nil
		}
		return client.PrepareJobResult{}, false, err
	}
	return result, false, nil
}

func prepareResultLiquibase(w stdoutAndErr, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string) (client.PrepareJobResult, bool, error) {
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	if showHelp {
		cli.PrintPrepareUsage(w.stdout)
		return client.PrepareJobResult{}, true, nil
	}

	if len(parsed.PsqlArgs) == 0 {
		return client.PrepareJobResult{}, false, ExitErrorf(2, "liquibase command is required")
	}

	imageID, source, err := resolvePrepareImage(parsed.Image, cfg)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	if imageID == "" {
		return client.PrepareJobResult{}, false, ExitErrorf(2, "Missing base image id (set --image or dbms.image)")
	}
	if runOpts.Verbose {
		fmt.Fprint(w.stderr, formatImageSource(imageID, source))
	}

	liquibaseExec, err := resolveLiquibaseExec(cfg)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	liquibaseExecMode, err := resolveLiquibaseExecMode(cfg)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	converter := buildPathConverter(runOpts)
	if shouldUseLiquibaseWindowsMode(liquibaseExec, liquibaseExecMode) {
		converter = nil
	}
	liquibaseArgs, err := normalizeLiquibaseArgs(parsed.PsqlArgs, workspaceRoot, cwd, converter)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}
	if shouldUseLiquibaseWindowsMode(liquibaseExec, liquibaseExecMode) {
		liquibaseArgs = relativizeLiquibaseArgs(liquibaseArgs, workspaceRoot, cwd)
	}
	liquibaseEnv := resolveLiquibaseEnv()
	workDir, err := normalizeWorkDir(cwd, converter)
	if err != nil {
		return client.PrepareJobResult{}, false, err
	}

	runOpts.ImageID = imageID
	runOpts.LiquibaseArgs = liquibaseArgs
	runOpts.LiquibaseExec = liquibaseExec
	runOpts.LiquibaseExecMode = liquibaseExecMode
	runOpts.LiquibaseEnv = liquibaseEnv
	runOpts.WorkDir = workDir
	runOpts.PrepareKind = "lb"

	if !parsed.Watch {
		accepted, err := cli.SubmitPrepare(context.Background(), runOpts)
		if err != nil {
			return client.PrepareJobResult{}, false, err
		}
		printPrepareJobRefs(w.stdout, accepted)
		if runOpts.CompositeRun {
			printRunSkipped(w.stdout, "prepare_not_watched")
		}
		return client.PrepareJobResult{}, true, nil
	}

	result, err := cli.RunPrepare(context.Background(), runOpts)
	if err != nil {
		var detached *cli.PrepareDetachedError
		if errors.As(err, &detached) {
			accepted := client.PrepareJobAccepted{
				JobID:     detached.JobID,
				StatusURL: "/v1/prepare-jobs/" + detached.JobID,
				EventsURL: "/v1/prepare-jobs/" + detached.JobID + "/events",
			}
			printPrepareJobRefs(w.stdout, accepted)
			if runOpts.CompositeRun {
				printRunSkipped(w.stdout, "prepare_detached")
			}
			return client.PrepareJobResult{}, true, nil
		}
		return client.PrepareJobResult{}, false, err
	}
	return result, false, nil
}

func printPrepareJobRefs(w io.Writer, accepted client.PrepareJobAccepted) {
	fmt.Fprintf(w, "JOB_ID=%s\n", accepted.JobID)
	fmt.Fprintf(w, "STATUS_URL=%s\n", accepted.StatusURL)
	fmt.Fprintf(w, "EVENTS_URL=%s\n", accepted.EventsURL)
}

func printRunSkipped(w io.Writer, reason string) {
	fmt.Fprintf(w, "RUN_SKIPPED=%s\n", reason)
}

func resolvePrepareImage(cliValue string, cfg config.LoadedConfig) (string, string, error) {
	if strings.TrimSpace(cliValue) != "" {
		return strings.TrimSpace(cliValue), "command line", nil
	}
	if cfg.ProjectConfigPath != "" {
		if value, ok, err := config.LookupDBMSImage(cfg.ProjectConfigPath); err != nil {
			return "", "", err
		} else if ok {
			return value, "workspace config", nil
		}
	}
	globalPath := filepath.Join(cfg.Paths.ConfigDir, "config.yaml")
	if fileExists(globalPath) {
		if value, ok, err := config.LookupDBMSImage(globalPath); err != nil {
			return "", "", err
		} else if ok {
			return value, "global config", nil
		}
	}
	return "", "", nil
}

func resolveLiquibaseExec(cfg config.LoadedConfig) (string, error) {
	if cfg.ProjectConfigPath != "" {
		if value, ok, err := config.LookupLiquibaseExec(cfg.ProjectConfigPath); err != nil {
			return "", err
		} else if ok {
			return sanitizeLiquibaseExec(value), nil
		}
	}
	globalPath := filepath.Join(cfg.Paths.ConfigDir, "config.yaml")
	if fileExists(globalPath) {
		if value, ok, err := config.LookupLiquibaseExec(globalPath); err != nil {
			return "", err
		} else if ok {
			return sanitizeLiquibaseExec(value), nil
		}
	}
	return "", nil
}

func resolveLiquibaseExecMode(cfg config.LoadedConfig) (string, error) {
	if cfg.ProjectConfigPath != "" {
		if value, ok, err := config.LookupLiquibaseExecMode(cfg.ProjectConfigPath); err != nil {
			return "", err
		} else if ok {
			return value, nil
		}
	}
	globalPath := filepath.Join(cfg.Paths.ConfigDir, "config.yaml")
	if fileExists(globalPath) {
		if value, ok, err := config.LookupLiquibaseExecMode(globalPath); err != nil {
			return "", err
		} else if ok {
			return value, nil
		}
	}
	return "", nil
}

func resolveLiquibaseEnv() map[string]string {
	javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME"))
	if javaHome == "" {
		return nil
	}
	javaHome = strings.Trim(javaHome, "\"")
	javaHome = strings.TrimRight(javaHome, "\\/")
	if javaHome == "" {
		return nil
	}
	return map[string]string{"JAVA_HOME": javaHome}
}

func shouldUseLiquibaseWindowsMode(execPath string, execMode string) bool {
	mode := strings.TrimSpace(strings.ToLower(execMode))
	switch mode {
	case "windows-bat":
		return true
	case "native":
		return false
	}
	path := strings.ToLower(strings.TrimSpace(execPath))
	return strings.HasSuffix(path, ".bat") || strings.HasSuffix(path, ".cmd")
}

func sanitizeLiquibaseExec(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, `\"`) {
		value = strings.ReplaceAll(value, `\"`, `"`)
	}
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return strings.TrimSpace(value)
}

func formatImageSource(imageID, source string) string {
	if imageID == "" || source == "" {
		return ""
	}
	return fmt.Sprintf("dbms.image=%s (source: %s)\n", imageID, source)
}

func normalizePsqlArgs(args []string, workspaceRoot string, cwd string, stdin io.Reader, convert func(string) (string, error)) ([]string, *string, error) {
	normalized := make([]string, 0, len(args))
	usesStdin := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			path, useStdin, err := normalizeFilePath(args[i+1], workspaceRoot, cwd, convert)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, arg, path)
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			path, useStdin, err := normalizeFilePath(value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, "--file="+path)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			path, useStdin, err := normalizeFilePath(value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, "-f"+path)
		default:
			normalized = append(normalized, arg)
		}
	}

	if !usesStdin {
		return normalized, nil, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)
	return normalized, &text, nil
}

func normalizeFilePath(path string, workspaceRoot string, cwd string, convert func(string) (string, error)) (string, bool, error) {
	if path == "-" {
		return path, true, nil
	}
	if strings.TrimSpace(path) == "" {
		return "", false, ExitErrorf(2, "File path is empty")
	}
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		root = cwd
	}
	root = filepath.Clean(root)
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, absPath)
	}
	absPath = filepath.Clean(absPath)
	if rootResolved, rootErr := filepath.EvalSymlinks(root); rootErr == nil {
		if pathResolved, pathErr := filepath.EvalSymlinks(absPath); pathErr == nil {
			root = rootResolved
			absPath = pathResolved
		}
	}
	if root != "" && !isWithin(root, absPath) {
		return "", false, ExitErrorf(2, "File path must be within workspace root: %s", absPath)
	}
	if convert != nil {
		converted, err := convert(absPath)
		if err != nil {
			return "", false, err
		}
		return converted, false, nil
	}
	return absPath, false, nil
}

func buildPathConverter(opts cli.PrepareOptions) func(string) (string, error) {
	if opts.WSLDistro == "" {
		return nil
	}
	if runtime.GOOS != "windows" {
		return nil
	}
	return windowsToWSLPath
}

func normalizeLiquibaseArgs(args []string, workspaceRoot string, cwd string, convert func(string) (string, error)) ([]string, error) {
	normalized := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file" || arg == "--searchPath" || arg == "--search-path":
			if i+1 >= len(args) {
				return nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			value := args[i+1]
			flag := arg
			if flag == "--search-path" {
				flag = "--searchPath"
			}
			rewritten, err := rewriteLiquibasePathArg(flag, value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, flag, rewritten)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value := strings.TrimPrefix(arg, "--changelog-file=")
			if strings.TrimSpace(value) == "" {
				return nil, ExitErrorf(2, "Missing value for --changelog-file")
			}
			rewritten, err := rewriteLiquibasePathArg("--changelog-file", value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--changelog-file="+rewritten)
		case strings.HasPrefix(arg, "--defaults-file="):
			value := strings.TrimPrefix(arg, "--defaults-file=")
			if strings.TrimSpace(value) == "" {
				return nil, ExitErrorf(2, "Missing value for --defaults-file")
			}
			rewritten, err := rewriteLiquibasePathArg("--defaults-file", value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--defaults-file="+rewritten)
		case strings.HasPrefix(arg, "--searchPath="):
			value := strings.TrimPrefix(arg, "--searchPath=")
			if strings.TrimSpace(value) == "" {
				return nil, ExitErrorf(2, "Missing value for --searchPath")
			}
			rewritten, err := rewriteLiquibasePathArg("--searchPath", value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--searchPath="+rewritten)
		case strings.HasPrefix(arg, "--search-path="):
			value := strings.TrimPrefix(arg, "--search-path=")
			if strings.TrimSpace(value) == "" {
				return nil, ExitErrorf(2, "Missing value for --search-path")
			}
			rewritten, err := rewriteLiquibasePathArg("--searchPath", value, workspaceRoot, cwd, convert)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--searchPath="+rewritten)
		default:
			normalized = append(normalized, arg)
		}
	}

	return normalized, nil
}

func normalizeWorkDir(cwd string, convert func(string) (string, error)) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", nil
	}
	if convert != nil {
		converted, err := convert(cwd)
		if err != nil {
			return "", err
		}
		return converted, nil
	}
	return cwd, nil
}

func rewriteLiquibasePathArg(flag string, value string, workspaceRoot string, cwd string, convert func(string) (string, error)) (string, error) {
	if flag == "--searchPath" || flag == "--search-path" {
		if strings.TrimSpace(value) == "" {
			return "", ExitErrorf(2, "searchPath is empty")
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				return "", ExitErrorf(2, "searchPath is empty")
			}
			if looksLikeLiquibaseRemoteRef(item) {
				out = append(out, item)
				continue
			}
			normalized, _, err := normalizeFilePath(item, workspaceRoot, cwd, convert)
			if err != nil {
				return "", err
			}
			out = append(out, normalized)
		}
		return strings.Join(out, ","), nil
	}

	if strings.TrimSpace(value) == "" {
		return "", ExitErrorf(2, "Path is empty")
	}
	if looksLikeLiquibaseRemoteRef(value) {
		return value, nil
	}
	normalized, _, err := normalizeFilePath(value, workspaceRoot, cwd, convert)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func relativizeLiquibaseArgs(args []string, workspaceRoot string, cwd string) []string {
	base := strings.TrimSpace(workspaceRoot)
	if base == "" {
		base = cwd
	}
	if strings.TrimSpace(base) == "" {
		return args
	}
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file":
			if i+1 >= len(args) {
				normalized = append(normalized, arg)
				continue
			}
			value := args[i+1]
			rel := toRelativeIfWithin(base, value)
			normalized = append(normalized, arg, rel)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value := strings.TrimPrefix(arg, "--changelog-file=")
			rel := toRelativeIfWithin(base, value)
			normalized = append(normalized, "--changelog-file="+rel)
		case strings.HasPrefix(arg, "--defaults-file="):
			value := strings.TrimPrefix(arg, "--defaults-file=")
			rel := toRelativeIfWithin(base, value)
			normalized = append(normalized, "--defaults-file="+rel)
		default:
			normalized = append(normalized, arg)
		}
	}
	return normalized
}

func toRelativeIfWithin(base string, value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	if !filepath.IsAbs(value) {
		return value
	}
	rel, err := filepath.Rel(base, value)
	if err != nil {
		return value
	}
	if rel == "." {
		return "."
	}
	if strings.HasPrefix(rel, "..") {
		return value
	}
	return rel
}

func looksLikeLiquibaseRemoteRef(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "://") || strings.HasPrefix(lower, "classpath:")
}
