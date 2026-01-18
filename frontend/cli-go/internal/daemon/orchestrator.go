package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/util"
)

type ConnectOptions struct {
	Endpoint       string
	Autostart      bool
	DaemonPath     string
	RunDir         string
	StateDir       string
	StartupTimeout time.Duration
	ClientTimeout  time.Duration
	Verbose        bool
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
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return ConnectResult{}, err
	}
	logFileClosed := false
	closeLogFile := func() {
		if logFileClosed {
			return
		}
		logFileClosed = true
		_ = logFile.Close()
	}

	cmd, err := buildDaemonCommand(opts.DaemonPath, opts.RunDir, enginePath)
	if err != nil {
		closeLogFile()
		return ConnectResult{}, err
	}
	cliOutput := newGatedWriter(os.Stderr)
	logWriter := io.MultiWriter(logFile, cliOutput)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	logVerbose(opts.Verbose, "starting engine: %s", opts.DaemonPath)
	if err := cmd.Start(); err != nil {
		closeLogFile()
		return ConnectResult{}, err
	}

	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
		closeLogFile()
	}()

	logVerbose(opts.Verbose, "waiting for engine to become healthy")
	deadline := time.Now().Add(opts.StartupTimeout)
	for {
		select {
		case err := <-exitCh:
			cliOutput.Disable()
			return ConnectResult{}, formatEngineExit(err)
		default:
		}
		if state, ok := loadHealthyState(ctx, enginePath, opts.ClientTimeout); ok {
			cliOutput.Disable()
			logVerbose(opts.Verbose, "engine healthy at %s", state.Endpoint)
			return ConnectResult{Endpoint: state.Endpoint, AuthToken: state.AuthToken, State: state}, nil
		}
		if time.Now().After(deadline) {
			cliOutput.Disable()
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
	state, err := ReadEngineState(enginePath)
	if err != nil {
		return EngineState{}, false
	}
	health, healthErr := checkHealth(ctx, state.Endpoint, timeout)
	pidRunning := processExists(state.PID)
	if IsEngineStateStale(state, health, healthErr, pidRunning) {
		return EngineState{}, false
	}
	return state, true
}

func checkHealth(ctx context.Context, endpoint string, timeout time.Duration) (client.HealthResponse, error) {
	client := client.New(endpoint, client.Options{Timeout: timeout})
	return client.Health(ctx)
}

type gatedWriter struct {
	enabled atomic.Bool
	w       io.Writer
}

func newGatedWriter(w io.Writer) *gatedWriter {
	writer := &gatedWriter{w: w}
	writer.enabled.Store(true)
	return writer
}

func (g *gatedWriter) Disable() {
	g.enabled.Store(false)
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	if !g.enabled.Load() {
		return len(p), nil
	}
	return g.w.Write(p)
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
