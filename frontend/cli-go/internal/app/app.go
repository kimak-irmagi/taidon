package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/paths"
	"github.com/sqlrs/cli/internal/wsl"
)

const defaultTimeout = 30 * time.Second
const defaultStartupTimeout = 5 * time.Second
const defaultIdleTimeout = 120 * time.Second

var spinnerInitialDelay = 500 * time.Millisecond
var spinnerTickInterval = 150 * time.Millisecond

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func formatCleanupResult(result client.DeleteResult) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(result.Outcome) != "" {
		parts = append(parts, "outcome="+result.Outcome)
	}
	if strings.TrimSpace(result.Root.Blocked) != "" {
		parts = append(parts, "blocked="+result.Root.Blocked)
	}
	if result.Root.Connections != nil {
		parts = append(parts, fmt.Sprintf("connections=%d", *result.Root.Connections))
	}
	if len(parts) == 0 {
		return "blocked"
	}
	return strings.Join(parts, ", ")
}

func startCleanupSpinner(instanceID string, verbose bool) func() {
	label := fmt.Sprintf("Deleting instance %s", instanceID)
	out := os.Stdout
	if verbose || !isTerminalWriterFn(out) {
		fmt.Fprintln(out, label)
		return func() {}
	}

	clearLen := len(label) + 2
	done := make(chan struct{})
	finished := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(finished)
		timer := time.NewTimer(spinnerInitialDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-done:
			return
		}
		spinner := []string{"-", "\\", "|", "/"}
		idx := 0
		ticker := time.NewTicker(spinnerTickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				clearLineOut(out, clearLen)
				return
			case <-ticker.C:
				clearLineOut(out, clearLen)
				fmt.Fprintf(out, "%s %s", label, spinner[idx])
				idx = (idx + 1) % len(spinner)
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			<-finished
		})
	}
}

func clearLineOut(out io.Writer, width int) {
	if out == nil {
		return
	}
	if width <= 0 {
		width = 1
	}
	fmt.Fprint(out, "\r")
	fmt.Fprint(out, strings.Repeat(" ", width))
	fmt.Fprint(out, "\r")
}

func resolveWSLSettings(cfg config.Config, dirs paths.Dirs, daemonPath string) (string, string, string, string, string, string, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Engine.WSL.Mode))
	if mode == "" {
		return daemonPath, "", "", "", "", "", "", nil
	}
	if mode != "auto" && mode != "required" {
		return daemonPath, "", "", "", "", "", "", nil
	}

	stateDir := strings.TrimSpace(cfg.Engine.WSL.StateDir)
	distro := strings.TrimSpace(cfg.Engine.WSL.Distro)
	mountUnit := strings.TrimSpace(cfg.Engine.WSL.Mount.Unit)
	mountFSType := strings.TrimSpace(cfg.Engine.WSL.Mount.FSType)
	if distro == "" {
		distros, err := listWSLDistrosFn()
		if err != nil {
			if mode == "required" {
				return "", "", "", "", "", "", "", fmt.Errorf("WSL unavailable: %v", err)
			}
			return daemonPath, "", "", "", "", "", "", nil
		}
		distro, err = wsl.SelectDistro(distros, "")
		if err != nil {
			if mode == "required" {
				return "", "", "", "", "", "", "", fmt.Errorf("WSL distro resolution failed: %v", err)
			}
			return daemonPath, "", "", "", "", "", "", nil
		}
	}
	if distro == "" || stateDir == "" {
		if mode == "required" {
			return "", "", "", "", "", "", "", fmt.Errorf("WSL configuration is missing distro or stateDir")
		}
		return daemonPath, "", "", "", "", "", "", nil
	}
	if mountUnit == "" && mode == "required" {
		return "", "", "", "", "", "", "", fmt.Errorf("WSL configuration is missing mount unit (run sqlrs init local --snapshot btrfs)")
	}
	if mountFSType == "" && mountUnit != "" {
		mountFSType = "btrfs"
	}

	engineBinary := daemonPath
	if cfg.Engine.WSL.EnginePath != "" {
		engineBinary = cfg.Engine.WSL.EnginePath
	}
	wslDaemonPath, err := windowsToWSLPath(engineBinary)
	if err != nil {
		if mode == "required" {
			return "", "", "", "", "", "", "", err
		}
		return daemonPath, "", "", "", "", "", "", nil
	}

	statePath := filepath.Join(dirs.StateDir, "engine.json")
	wslStatePath, err := windowsToWSLPath(statePath)
	if err != nil {
		if mode == "required" {
			return "", "", "", "", "", "", "", err
		}
		return daemonPath, "", "", "", "", "", "", nil
	}

	runDir := path.Join(stateDir, "run")
	return wslDaemonPath, runDir, wslStatePath, stateDir, distro, mountUnit, mountFSType, nil
}

func resolveAuthToken(auth config.AuthConfig) string {
	if env := strings.TrimSpace(auth.TokenEnv); env != "" {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(auth.Token)
}

func windowsToWSLPath(value string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(cleaned, "/") {
		return cleaned, nil
	}
	drive := ""
	rest := ""
	if len(cleaned) >= 2 && cleaned[1] == ':' {
		letter := cleaned[0]
		if (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') {
			drive = strings.ToLower(cleaned[:1])
			rest = cleaned[2:]
		}
	}
	if drive == "" {
		vol := filepath.VolumeName(cleaned)
		if vol == "" {
			return "", fmt.Errorf("path is not absolute: %s", cleaned)
		}
		drive = strings.TrimSuffix(strings.ToLower(vol), ":")
		rest = cleaned[len(vol):]
	}
	rest = strings.TrimLeft(rest, `\\/`)
	rest = strings.ReplaceAll(rest, "\\", "/")
	if rest == "" {
		return fmt.Sprintf("/mnt/%s", drive), nil
	}
	return fmt.Sprintf("/mnt/%s/%s", drive, rest), nil
}
