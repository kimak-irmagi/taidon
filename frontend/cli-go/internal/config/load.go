package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"sqlrs/cli/internal/paths"
)

func Load(opts LoadOptions) (LoadedConfig, error) {
	var err error
	result := LoadedConfig{}

	if opts.WorkingDir == "" {
		opts.WorkingDir, err = os.Getwd()
		if err != nil {
			return result, err
		}
	}

	dirs := opts.Dirs
	if dirs == nil {
		resolved, err := paths.Resolve()
		if err != nil {
			return result, err
		}
		dirs = &resolved
	}

	merged := DefaultConfigMap()

	globalPath := filepath.Join(dirs.ConfigDir, "config.yaml")
	if fileExists(globalPath) {
		data, err := readConfigMap(globalPath)
		if err != nil {
			return result, fmt.Errorf("read config: %w", err)
		}
		mergeMap(merged, data)
	}

	projectPath, err := paths.FindProjectConfig(opts.WorkingDir)
	if err != nil {
		return result, err
	}
	if projectPath != "" {
		data, err := readConfigMap(projectPath)
		if err != nil {
			return result, fmt.Errorf("read project config: %w", err)
		}
		mergeMap(merged, data)
	}

	vars := map[string]string{
		"ConfigDir": dirs.ConfigDir,
		"StateDir":  dirs.StateDir,
		"CacheDir":  dirs.CacheDir,
	}

	if envRoot := os.Getenv("SQLRSROOT"); envRoot != "" {
		vars["SQLRSROOT"] = envRoot
	} else {
		vars["SQLRSROOT"] = dirs.StateDir
	}

	ExpandMap(merged, vars)

	raw, err := yaml.Marshal(merged)
	if err != nil {
		return result, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return result, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	if cfg.Orchestrator.RunDir != "" {
		cfg.Orchestrator.RunDir = filepath.Clean(cfg.Orchestrator.RunDir)
	}
	if cfg.Orchestrator.DaemonPath != "" && !filepath.IsAbs(cfg.Orchestrator.DaemonPath) {
		base := dirs.ConfigDir
		if projectPath != "" {
			base = filepath.Dir(projectPath)
		}
		cfg.Orchestrator.DaemonPath = filepath.Clean(filepath.Join(base, cfg.Orchestrator.DaemonPath))
	}

	result.Config = cfg
	result.Paths = *dirs
	result.ProjectConfigPath = projectPath
	return result, nil
}

func DefaultConfigMap() map[string]any {
	return map[string]any{
		"defaultProfile": "local",
		"client": map[string]any{
			"timeout": "30s",
			"retries": 1,
			"output":  "human",
		},
		"orchestrator": map[string]any{
			"startupTimeout": "5s",
			"idleTimeout":    "120s",
			"runDir":         "${StateDir}/run",
		},
		"profiles": map[string]any{
			"local": map[string]any{
				"mode":      "local",
				"endpoint":  "auto",
				"autostart": true,
				"auth": map[string]any{
					"mode": "fileToken",
				},
			},
		},
	}
}

func readConfigMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized, ok := normalizeMap(raw).(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	return normalized, nil
}

func normalizeMap(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = normalizeMap(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			out[keyStr] = normalizeMap(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeMap(item)
		}
		return out
	default:
		return value
	}
}

func mergeMap(dst, src map[string]any) {
	for key, srcVal := range src {
		if dstMap, ok := dst[key].(map[string]any); ok {
			if srcMap, ok := srcVal.(map[string]any); ok {
				mergeMap(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
