package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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
	defer logFile.Close()

	cmd, err := buildDaemonCommand(opts.DaemonPath, opts.RunDir, enginePath)
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
	for {
		select {
		case err := <-exitCh:
			if tailer != nil {
				tailer.Stop()
			}
			return ConnectResult{}, formatEngineExit(err)
		default:
		}
		if state, ok := loadHealthyState(ctx, enginePath, opts.ClientTimeout); ok {
			if tailer != nil {
				tailer.Stop()
			}
			logVerbose(opts.Verbose, "engine healthy at %s", state.Endpoint)
			return ConnectResult{Endpoint: state.Endpoint, AuthToken: state.AuthToken, State: state}, nil
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
