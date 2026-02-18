package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/util"
)

var isWindows = runtime.GOOS == "windows"

type ConnectOptions struct {
	Endpoint        string
	Autostart       bool
	DaemonPath      string
	RunDir          string
	StateDir        string
	EngineRunDir    string
	EngineStatePath string
	WSLDistro       string
	EngineStoreDir  string
	WSLVHDXPath     string
	WSLMountUnit    string
	WSLMountFSType  string
	IdleTimeout     time.Duration
	StartupTimeout  time.Duration
	ClientTimeout   time.Duration
	Verbose         bool
}

type ConnectResult struct {
	Endpoint  string
	AuthToken string
	State     EngineState
}

func ConnectOrStart(ctx context.Context, opts ConnectOptions) (ConnectResult, error) {
	endpoint := opts.Endpoint
	if endpoint != "" && endpoint != "auto" {
		return ConnectResult{Endpoint: endpoint}, nil
	}

	if err := ensureWSLStoreMount(ctx, opts); err != nil {
		return ConnectResult{}, err
	}

	enginePath := filepath.Join(opts.StateDir, "engine.json")
	logVerbose(opts.Verbose, "checking engine.json at %s", enginePath)
	if state, ok := loadHealthyState(ctx, enginePath, opts.ClientTimeout); ok {
		logVerbose(opts.Verbose, "engine healthy at %s", state.Endpoint)
		return ConnectResult{Endpoint: state.Endpoint, AuthToken: state.AuthToken, State: state}, nil
	}

	if !opts.Autostart {
		logVerbose(opts.Verbose, "engine not running and autostart disabled")
		return ConnectResult{}, fmt.Errorf("local engine is not running")
	}
	if opts.DaemonPath == "" {
		return ConnectResult{}, fmt.Errorf("local daemon path is not configured")
	}

	if opts.RunDir == "" {
		return ConnectResult{}, fmt.Errorf("runDir is not configured")
	}

	if err := util.EnsureDir(opts.RunDir); err != nil {
		return ConnectResult{}, err
	}

	lockPath := filepath.Join(opts.RunDir, "daemon.lock")
	lock, err := AcquireLock(lockPath, opts.StartupTimeout)
	if err != nil {
		return ConnectResult{}, err
	}
	defer lock.Release()

	logVerbose(opts.Verbose, "acquired engine lock")
	if state, ok := loadHealthyState(ctx, enginePath, opts.ClientTimeout); ok {
		logVerbose(opts.Verbose, "engine became healthy while waiting for lock")
		return ConnectResult{Endpoint: state.Endpoint, AuthToken: state.AuthToken, State: state}, nil
	}

	logsDir := filepath.Join(opts.StateDir, "logs")
	if err := util.EnsureDir(logsDir); err != nil {
		return ConnectResult{}, err
	}

	logPath := filepath.Join(logsDir, "engine.log")
	isWSL := opts.WSLDistro != ""
	if isWSL {
		logPath = filepath.Join(logsDir, "engine-wsl.log")
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return ConnectResult{}, err
	}
	defer logFile.Close()

	daemonRunDir := opts.EngineRunDir
	if daemonRunDir == "" {
		daemonRunDir = opts.RunDir
	}
	daemonStatePath := opts.EngineStatePath
	if daemonStatePath == "" {
		daemonStatePath = enginePath
	}
	cmd, err := buildDaemonCommand(
		opts.DaemonPath,
		daemonRunDir,
		daemonStatePath,
		opts.WSLDistro,
		opts.EngineStoreDir,
		opts.WSLMountUnit,
		opts.WSLMountFSType,
		opts.IdleTimeout,
		logPath,
	)
	if err != nil {
		return ConnectResult{}, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	tailer, err := startLogTail(logPath, os.Stderr)
	if err != nil {
		tailer = nil
	}

	logVerbose(opts.Verbose, "starting engine: %s", opts.DaemonPath)
	if err := cmd.Start(); err != nil {
		if tailer != nil {
			tailer.Stop()
		}
		return ConnectResult{}, err
	}

	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
	}()

	logVerbose(opts.Verbose, "waiting for engine to become healthy")
	deadline := time.Now().Add(opts.StartupTimeout)
	lastReason := ""
	lastReasonAt := time.Time{}
	for {
		select {
		case err := <-exitCh:
			if tailer != nil {
				tailer.Stop()
			}
			return ConnectResult{}, formatEngineExit(err)
		default:
		}
		if state, ok, reason := loadHealthyStateWithReason(ctx, enginePath, opts.ClientTimeout); ok {
			if tailer != nil {
				tailer.Stop()
			}
			logVerbose(opts.Verbose, "engine healthy at %s", state.Endpoint)
			return ConnectResult{Endpoint: state.Endpoint, AuthToken: state.AuthToken, State: state}, nil
		} else if opts.Verbose && reason != "" {
			now := time.Now()
			if reason != lastReason || now.Sub(lastReasonAt) > time.Second {
				logVerbose(opts.Verbose, "engine not healthy yet: %s", reason)
				lastReason = reason
				lastReasonAt = now
			}
		}
		if time.Now().After(deadline) {
			if tailer != nil {
				tailer.Stop()
			}
			return ConnectResult{}, fmt.Errorf("engine did not become healthy within startup timeout")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func logVerbose(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func loadHealthyState(ctx context.Context, enginePath string, timeout time.Duration) (EngineState, bool) {
	state, ok, _ := loadHealthyStateWithReason(ctx, enginePath, timeout)
	return state, ok
}

func checkHealth(ctx context.Context, endpoint string, timeout time.Duration) (client.HealthResponse, error) {
	client := client.New(endpoint, client.Options{Timeout: timeout})
	return client.Health(ctx)
}

func loadHealthyStateWithReason(ctx context.Context, enginePath string, timeout time.Duration) (EngineState, bool, string) {
	state, err := ReadEngineState(enginePath)
	if err != nil {
		return EngineState{}, false, fmt.Sprintf("engine.json read failed: %v", err)
	}
	health, healthErr := checkHealth(ctx, state.Endpoint, timeout)
	if healthErr != nil {
		return EngineState{}, false, fmt.Sprintf("health check failed: %v", healthErr)
	}
	pidRunning := processExists(state.PID)
	if isWindows {
		pidRunning = true
	}
	if state.InstanceID != "" && health.InstanceID != "" && state.InstanceID != health.InstanceID {
		return EngineState{}, false, fmt.Sprintf("instanceId mismatch (state=%s health=%s)", state.InstanceID, health.InstanceID)
	}
	if state.PID > 0 && !pidRunning {
		return EngineState{}, false, fmt.Sprintf("engine pid not running (pid=%d)", state.PID)
	}
	if IsEngineStateStale(state, health, healthErr, pidRunning) {
		return EngineState{}, false, "engine state is stale"
	}
	return state, true, ""
}

func formatEngineExit(err error) error {
	if err == nil {
		return fmt.Errorf("engine exited before becoming healthy (exit code 0)")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("engine exited before becoming healthy (exit code %d)", exitErr.ExitCode())
	}
	return fmt.Errorf("engine exited before becoming healthy: %v", err)
}

var runWSLCommandFn = runWSLCommand
var runHostCommandFn = runHostCommand

func ensureWSLStoreMount(ctx context.Context, opts ConnectOptions) error {
	if strings.TrimSpace(opts.WSLDistro) == "" {
		return nil
	}
	unit := strings.TrimSpace(opts.WSLMountUnit)
	fstype := strings.TrimSpace(opts.WSLMountFSType)
	storeDir := strings.TrimSpace(opts.EngineStoreDir)
	if unit == "" || storeDir == "" {
		return nil
	}
	if fstype == "" {
		fstype = "btrfs"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logVerbose(opts.Verbose, "ensuring WSL store mount (%s) via %s", fstype, unit)

	if err := ensureWSLMountUnitActive(ctx, opts.WSLDistro, unit); err != nil {
		if runtime.GOOS == "windows" && strings.TrimSpace(opts.WSLVHDXPath) != "" {
			if attachErr := attachVHDXToWSL(ctx, opts.WSLVHDXPath, opts.Verbose); attachErr != nil {
				return appendWSLMountLogs(ctx, opts.WSLDistro, unit, attachErr)
			}
			if err := ensureWSLMountUnitActive(ctx, opts.WSLDistro, unit); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	out, err := runWSLCommandInInitNamespace(ctx, opts.WSLDistro, "findmnt", "-n", "-o", "FSTYPE", "-T", storeDir)
	if err != nil {
		return err
	}
	fs := strings.TrimSpace(out)
	if fs == "" {
		return fmt.Errorf("WSL store mount is not available (empty fstype)")
	}
	if fs != fstype {
		if runtime.GOOS == "windows" && strings.TrimSpace(opts.WSLVHDXPath) != "" {
			logVerbose(opts.Verbose, "WSL store mount is %s, retrying attach/start", fs)
			if attachErr := attachVHDXToWSL(ctx, opts.WSLVHDXPath, opts.Verbose); attachErr != nil {
				return appendWSLMountLogs(ctx, opts.WSLDistro, unit, attachErr)
			}
			if err := ensureWSLMountUnitActive(ctx, opts.WSLDistro, unit); err != nil {
				return err
			}
			out, err = runWSLCommandInInitNamespace(ctx, opts.WSLDistro, "findmnt", "-n", "-o", "FSTYPE", "-T", storeDir)
			if err != nil {
				return err
			}
			fs = strings.TrimSpace(out)
			if fs == "" {
				return fmt.Errorf("WSL store mount is not available (empty fstype)")
			}
			if fs != fstype {
				return appendWSLMountLogs(ctx, opts.WSLDistro, unit, fmt.Errorf("WSL store mount is %s, expected %s", fs, fstype))
			}
			return nil
		}
		return appendWSLMountLogs(ctx, opts.WSLDistro, unit, fmt.Errorf("WSL store mount is %s, expected %s", fs, fstype))
	}
	return nil
}

func runWSLCommand(ctx context.Context, distro string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmdArgs := []string{"-d", distro, "-u", "root", "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "wsl.exe", cmdArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runHostCommand(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runWSLCommandInInitNamespace(ctx context.Context, distro string, args ...string) (string, error) {
	cmdArgs := append([]string{"nsenter", "-t", "1", "-m", "--"}, args...)
	out, err := runWSLCommandFn(ctx, distro, cmdArgs...)
	if err == nil {
		return out, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "command not found") {
		return runWSLCommandFn(ctx, distro, args...)
	}
	return out, err
}

func ensureWSLMountUnitActive(ctx context.Context, distro, unit string) error {
	out, err := runWSLCommandFn(ctx, distro, "systemctl", "is-active", unit)
	if err == nil && strings.TrimSpace(out) == "active" {
		return nil
	}
	_, startErr := runWSLCommandFn(ctx, distro, "systemctl", "start", "--no-block", unit)
	if startErr != nil {
		return appendWSLMountLogs(ctx, distro, unit, startErr)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		out, err = runWSLCommandFn(ctx, distro, "systemctl", "is-active", unit)
		if err == nil && strings.TrimSpace(out) == "active" {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return appendWSLMountLogs(ctx, distro, unit, fmt.Errorf("WSL mount unit is not active"))
}

func appendWSLMountLogs(ctx context.Context, distro, unit string, err error) error {
	tail, tailErr := runWSLCommandFn(ctx, distro, "journalctl", "-u", unit, "-n", "20", "--no-pager")
	if tailErr == nil && strings.TrimSpace(tail) != "" {
		return fmt.Errorf("%v\n%s", err, strings.TrimSpace(tail))
	}
	return err
}

func attachVHDXToWSL(ctx context.Context, vhdxPath string, verbose bool) error {
	if strings.TrimSpace(vhdxPath) == "" {
		return fmt.Errorf("VHDX path is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logVerbose(verbose, "attaching VHDX: %s", vhdxPath)
	out, err := runHostCommandFn(ctx, "wsl.exe", "--mount", vhdxPath, "--vhd", "--bare")
	if err != nil {
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("attach VHDX failed: %v (%s)", err, strings.TrimSpace(out))
		}
		return fmt.Errorf("attach VHDX failed: %v", err)
	}
	return nil
}

func isExitStatus(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

type logTail struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

func (t *logTail) Stop() {
	if t == nil {
		return
	}
	t.once.Do(func() {
		close(t.stop)
		<-t.done
	})
}

func startLogTail(path string, out io.Writer) (*logTail, error) {
	if out == nil {
		return nil, fmt.Errorf("log tail output is nil")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, err
	}

	tail := &logTail{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	go func() {
		defer close(tail.done)
		defer file.Close()

		buf := make([]byte, 4096)
		for {
			select {
			case <-tail.stop:
				return
			default:
			}

			n, err := file.Read(buf)
			if n > 0 {
				_, _ = out.Write(buf[:n])
			}
			if errors.Is(err, io.EOF) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if err != nil {
				return
			}
		}
	}()

	return tail, nil
}
