package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/util"
)

type initOptions struct {
	Workspace   string
	Force       bool
	Update      bool
	EnginePath  string
	SharedCache bool
	DryRun      bool
	WSL         bool
	WSLDistro   string
	WSLRequire  bool
	WSLNoStart  bool
	WSLReinit   bool
	StoreSizeGB int
	Verbose     bool
}

func runInit(w io.Writer, cwd, globalWorkspace string, args []string, verbose bool) error {
	opts, showHelp, err := parseInitFlags(args, globalWorkspace)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintInitUsage(w)
		return nil
	}
	opts.Verbose = verbose

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
	hasUpdateFlags := opts.EnginePath != "" || opts.SharedCache || opts.WSL
	configExists := fileExists(configPath)
	configValid := false

	if localExists {
		if configExists {
			if err := validateConfig(configPath); err != nil {
				if !opts.Update {
					return ExitErrorf(3, "Workspace config is corrupted: %v", err)
				}
			} else {
				configValid = true
			}
		}
		if !opts.Update {
			if opts.DryRun {
				fmt.Fprintf(w, "Workspace already initialized at %s (dry-run)\n", target)
			} else {
				fmt.Fprintf(w, "Workspace already initialized at %s\n", target)
			}
			return nil
		}
		if configExists && configValid && !hasUpdateFlags {
			if opts.DryRun {
				fmt.Fprintf(w, "Workspace already initialized at %s (dry-run)\n", target)
			} else {
				fmt.Fprintf(w, "Workspace already initialized at %s\n", target)
			}
			return nil
		}
	}

	if opts.EnginePath != "" {
		opts.EnginePath = normalizeEnginePath(opts.EnginePath, cwd, target)
	}

	var wslResult *wslInitResult
	if opts.WSL {
		result, err := initWSLFn(wslInitOptions{
			Enable:      opts.WSL,
			Distro:      opts.WSLDistro,
			Require:     opts.WSLRequire,
			NoStart:     opts.WSLNoStart,
			Workspace:   target,
			Verbose:     opts.Verbose,
			StoreSizeGB: opts.StoreSizeGB,
			Reinit:      opts.WSLReinit,
		})
		if err != nil {
			return ExitErrorf(1, "WSL init failed: %v", err)
		}
		if result.Warning != "" {
			fmt.Fprintln(os.Stderr, strings.TrimSpace(result.Warning))
		}
		if opts.Update && !result.UseWSL {
			return ExitErrorf(1, "WSL init failed: %s", strings.TrimSpace(result.Warning))
		}
		wslResult = &result
	}

	if opts.DryRun {
		if !localExists {
			fmt.Fprintf(w, "Would create %s\n", localMarker)
		}
		fmt.Fprintf(w, "Would write %s\n", configPath)
		return nil
	}

	if !localExists {
		if err := os.MkdirAll(localMarker, 0o700); err != nil {
			return ExitErrorf(4, "Cannot create .sqlrs directory: %v", err)
		}
	}

	baseConfig := map[string]any(nil)
	if opts.Update && configExists && configValid {
		loaded, err := readConfigMap(configPath)
		if err != nil {
			return ExitErrorf(4, "Cannot read config.yaml: %v", err)
		}
		baseConfig = loaded
	}
	configData, err := buildWorkspaceConfig(opts, wslResult, baseConfig)
	if err != nil {
		return ExitErrorf(1, "Internal error: %v", err)
	}
	if err := util.AtomicWriteFile(configPath, configData, 0o600); err != nil {
		return ExitErrorf(4, "Cannot write config.yaml: %v", err)
	}

	if localExists {
		fmt.Fprintf(w, "Updated workspace at %s\n", target)
	} else {
		fmt.Fprintf(w, "Initialized workspace at %s\n", target)
	}
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
	update := fs.Bool("update", false, "update existing workspace config")
	wsl := fs.Bool("wsl", false, "setup WSL2 integration")
	wslDistro := fs.String("distro", "", "WSL distro name")
	wslRequire := fs.Bool("require", false, "require WSL2+btrfs (no fallback)")
	wslNoStart := fs.Bool("no-start", false, "skip WSL engine auto-start")
	wslReinit := fs.Bool("reinit", false, "recreate WSL btrfs store")
	storeSize := fs.String("store-size", "", "btrfs store size (example: 100GB)")
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
	opts.Update = *update
	opts.EnginePath = strings.TrimSpace(*engine)
	opts.SharedCache = *sharedCache
	opts.DryRun = *dryRun
	opts.WSL = *wsl
	opts.WSLDistro = strings.TrimSpace(*wslDistro)
	opts.WSLRequire = *wslRequire
	opts.WSLNoStart = *wslNoStart
	opts.WSLReinit = *wslReinit
	if size := strings.TrimSpace(*storeSize); size != "" {
		value, err := parseStoreSizeGB(size)
		if err != nil {
			return opts, false, ExitErrorf(64, "Invalid arguments: %v", err)
		}
		opts.StoreSizeGB = value
	}
	if (opts.StoreSizeGB > 0 || opts.WSLReinit) && !opts.WSL {
		return opts, false, ExitErrorf(64, "Invalid arguments: --store-size/--reinit require --wsl")
	}
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

func buildWorkspaceConfig(opts initOptions, wslResult *wslInitResult, base map[string]any) ([]byte, error) {
	var cfg map[string]any
	if base != nil {
		cfg = base
	} else {
		cfg = config.DefaultConfigMap()
	}
	if opts.EnginePath != "" {
		setNested(cfg, []string{"orchestrator", "daemonPath"}, opts.EnginePath)
	}
	if opts.SharedCache {
		setNested(cfg, []string{"cache", "shared"}, true)
	}
	if opts.WSL {
		mode := "auto"
		if opts.WSLRequire {
			mode = "required"
		}
		setNested(cfg, []string{"engine", "wsl", "mode"}, mode)
		distro := opts.WSLDistro
		if wslResult != nil && wslResult.Distro != "" {
			distro = wslResult.Distro
		}
		if distro != "" {
			setNested(cfg, []string{"engine", "wsl", "distro"}, distro)
		}
		if wslResult != nil {
			if wslResult.StateDir != "" {
				setNested(cfg, []string{"engine", "wsl", "stateDir"}, wslResult.StateDir)
			}
			if wslResult.EnginePath != "" {
				setNested(cfg, []string{"engine", "wsl", "enginePath"}, wslResult.EnginePath)
			}
			if wslResult.MountDevice != "" {
				setNested(cfg, []string{"engine", "wsl", "mount", "device"}, wslResult.MountDevice)
			}
			if wslResult.MountFSType != "" {
				setNested(cfg, []string{"engine", "wsl", "mount", "fstype"}, wslResult.MountFSType)
			}
			if wslResult.MountDeviceUUID != "" {
				setNested(cfg, []string{"engine", "wsl", "mount", "deviceUUID"}, wslResult.MountDeviceUUID)
			}
			if wslResult.MountUnit != "" {
				setNested(cfg, []string{"engine", "wsl", "mount", "unit"}, wslResult.MountUnit)
			}
			if wslResult.StorePath != "" {
				setNested(cfg, []string{"engine", "storePath"}, wslResult.StorePath)
			}
		}
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

func readConfigMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return map[string]any{}, nil
	}
	return raw, nil
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

func parseStoreSizeGB(value string) (int, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, fmt.Errorf("store size is empty")
	}
	upper := strings.ToUpper(raw)
	if !strings.HasSuffix(upper, "GB") {
		return 0, fmt.Errorf("store size must use GB suffix")
	}
	num := strings.TrimSpace(raw[:len(raw)-2])
	if num == "" {
		return 0, fmt.Errorf("store size is empty")
	}
	size, err := strconv.Atoi(num)
	if err != nil || size <= 0 {
		return 0, fmt.Errorf("store size must be a positive integer")
	}
	return size, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
