package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/util"
)

type initOptions struct {
	Workspace   string
	Force       bool
	EnginePath  string
	SharedCache bool
	DryRun      bool
}

func runInit(w io.Writer, cwd, globalWorkspace string, args []string) error {
	opts, showHelp, err := parseInitFlags(args, globalWorkspace)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintInitUsage(w)
		return nil
	}

	target, err := resolveWorkspacePath(opts.Workspace, cwd)
	if err != nil {
		return ExitErrorf(4, "Cannot create .sqlrs directory: %v", err)
	}

	localMarker := filepath.Join(target, ".sqlrs")
	localExists := dirExists(localMarker)

	parentMarker := findParentWorkspace(target)
	if parentMarker && !localExists && !opts.Force {
		return ExitErrorf(2, "Refusing to create nested workspace")
	}

	configPath := filepath.Join(localMarker, "config.yaml")

	if localExists {
		if fileExists(configPath) {
			if err := validateConfig(configPath); err != nil {
				return ExitErrorf(3, "Workspace config is corrupted: %v", err)
			}
		}
		if opts.DryRun {
			fmt.Fprintf(w, "Workspace already initialized at %s (dry-run)\n", target)
		} else {
			fmt.Fprintf(w, "Workspace already initialized at %s\n", target)
		}
		return nil
	}

	if opts.EnginePath != "" {
		opts.EnginePath = normalizeEnginePath(opts.EnginePath, cwd, target)
	}

	if opts.DryRun {
		fmt.Fprintf(w, "Would create %s\n", localMarker)
		fmt.Fprintf(w, "Would write %s\n", configPath)
		return nil
	}

	if err := os.MkdirAll(localMarker, 0o700); err != nil {
		return ExitErrorf(4, "Cannot create .sqlrs directory: %v", err)
	}

	configData, err := buildWorkspaceConfig(opts)
	if err != nil {
		return ExitErrorf(1, "Internal error: %v", err)
	}
	if err := util.AtomicWriteFile(configPath, configData, 0o600); err != nil {
		return ExitErrorf(4, "Cannot write config.yaml: %v", err)
	}

	fmt.Fprintf(w, "Initialized workspace at %s\n", target)
	return nil
}

func parseInitFlags(args []string, globalWorkspace string) (initOptions, bool, error) {
	var opts initOptions

	fs := flag.NewFlagSet("sqlrs init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	workspace := fs.String("workspace", "", "workspace root")
	force := fs.Bool("force", false, "allow nested workspace")
	engine := fs.String("engine", "", "engine binary path")
	sharedCache := fs.Bool("shared-cache", false, "use shared cache")
	dryRun := fs.Bool("dry-run", false, "dry run")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return opts, false, ExitErrorf(64, "Invalid arguments: %v", err)
	}

	if *help || *helpShort {
		return opts, true, nil
	}

	if fs.NArg() > 0 {
		return opts, false, ExitErrorf(64, "Invalid arguments")
	}

	opts.Workspace = strings.TrimSpace(*workspace)
	if opts.Workspace == "" {
		opts.Workspace = strings.TrimSpace(globalWorkspace)
	}
	opts.Force = *force
	opts.EnginePath = strings.TrimSpace(*engine)
	opts.SharedCache = *sharedCache
	opts.DryRun = *dryRun
	return opts, false, nil
}

func resolveWorkspacePath(workspace, cwd string) (string, error) {
	target := strings.TrimSpace(workspace)
	if target == "" {
		target = cwd
	}
	if target == "" {
		return "", errors.New("workspace path is empty")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	abs := filepath.Clean(target)

	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace is not a directory: %s", abs)
	}
	return abs, nil
}

func findParentWorkspace(target string) bool {
	parent := filepath.Dir(target)
	for parent != "" && parent != filepath.Dir(parent) {
		if dirExists(filepath.Join(parent, ".sqlrs")) {
			return true
		}
		parent = filepath.Dir(parent)
	}
	if parent != "" && parent != target {
		if dirExists(filepath.Join(parent, ".sqlrs")) {
			return true
		}
	}
	return false
}

func buildWorkspaceConfig(opts initOptions) ([]byte, error) {
	cfg := config.DefaultConfigMap()
	if opts.EnginePath != "" {
		setNested(cfg, []string{"orchestrator", "daemonPath"}, opts.EnginePath)
	}
	if opts.SharedCache {
		setNested(cfg, []string{"cache", "shared"}, true)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data, nil
}

func normalizeEnginePath(enginePath, cwd, workspace string) string {
	path := strings.TrimSpace(enginePath)
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	cwdAbs := resolveExistingPath(cwd)
	if isWithin(workspace, cwdAbs) {
		configDir := filepath.Join(workspace, ".sqlrs")
		absEngine := filepath.Clean(filepath.Join(cwdAbs, path))
		if rel, err := filepath.Rel(configDir, absEngine); err == nil {
			return rel
		}
		return absEngine
	}

	return filepath.Clean(filepath.Join(cwdAbs, path))
}

func resolveExistingPath(value string) string {
	path := value
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func validateConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	return nil
}

func setNested(root map[string]any, keys []string, value any) {
	current := root
	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			return
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
