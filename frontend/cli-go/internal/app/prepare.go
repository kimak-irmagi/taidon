package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

type prepareArgs struct {
	Image    string
	PsqlArgs []string
}

func parsePrepareArgs(args []string) (prepareArgs, bool, error) {
	var opts prepareArgs
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
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintPrepareUsage(stdout)
		return nil
	}

	imageID, source, err := resolvePrepareImage(parsed.Image, cfg)
	if err != nil {
		return err
	}
	if imageID == "" {
		return ExitErrorf(2, "Missing base image id (set --image or dbms.image)")
	}
	if runOpts.Verbose {
		fmt.Fprint(stderr, formatImageSource(imageID, source))
	}

	psqlArgs, stdin, err := normalizePsqlArgs(parsed.PsqlArgs, workspaceRoot, cwd, os.Stdin)
	if err != nil {
		return err
	}

	runOpts.ImageID = imageID
	runOpts.PsqlArgs = psqlArgs
	runOpts.Stdin = stdin
	runOpts.PrepareKind = "psql"

	result, err := cli.RunPrepare(context.Background(), runOpts)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "DSN=%s\n", result.DSN)
	return nil
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

func formatImageSource(imageID, source string) string {
	if imageID == "" || source == "" {
		return ""
	}
	return fmt.Sprintf("dbms.image=%s (source: %s)\n", imageID, source)
}

func normalizePsqlArgs(args []string, workspaceRoot string, cwd string, stdin io.Reader) ([]string, *string, error) {
	normalized := make([]string, 0, len(args))
	usesStdin := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, nil, ExitErrorf(2, "Missing value for %s", arg)
			}
			path, useStdin, err := normalizeFilePath(args[i+1], workspaceRoot, cwd)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, arg, path)
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			path, useStdin, err := normalizeFilePath(value, workspaceRoot, cwd)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, "--file="+path)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			path, useStdin, err := normalizeFilePath(value, workspaceRoot, cwd)
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

func normalizeFilePath(path string, workspaceRoot string, cwd string) (string, bool, error) {
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
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, absPath)
	}
	absPath = filepath.Clean(absPath)
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}
	if root != "" && !isWithin(root, absPath) {
		return "", false, ExitErrorf(2, "File path must be within workspace root: %s", absPath)
	}
	return absPath, false, nil
}
