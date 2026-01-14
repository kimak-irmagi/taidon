package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

type Dirs struct {
	ConfigDir string
	StateDir  string
	CacheDir  string
}

func Resolve() (Dirs, error) {
	switch runtime.GOOS {
	case "windows":
		return resolveWindows()
	case "darwin":
		return resolveDarwin()
	default:
		return resolveXDG()
	}
}

func FindProjectConfig(startDir string) (string, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		startDir = cwd
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ".sqlrs", "config.yaml")
		if fileExists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", nil
}

func resolveXDG() (Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Dirs{}, err
	}

	configHome := getenvOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	stateHome := getenvOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	cacheHome := getenvOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	return Dirs{
		ConfigDir: filepath.Join(configHome, "sqlrs"),
		StateDir:  filepath.Join(stateHome, "sqlrs"),
		CacheDir:  filepath.Join(cacheHome, "sqlrs"),
	}, nil
}

func resolveDarwin() (Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Dirs{}, err
	}

	base := filepath.Join(home, "Library", "Application Support", "sqlrs")
	return Dirs{
		ConfigDir: filepath.Join(base, "config"),
		StateDir:  filepath.Join(base, "state"),
		CacheDir:  filepath.Join(base, "cache"),
	}, nil
}

func resolveWindows() (Dirs, error) {
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")

	if appData == "" || localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Dirs{}, err
		}
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
	}

	return Dirs{
		ConfigDir: filepath.Join(appData, "sqlrs"),
		StateDir:  filepath.Join(localAppData, "sqlrs"),
		CacheDir:  filepath.Join(localAppData, "sqlrs"),
	}, nil
}

func getenvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
