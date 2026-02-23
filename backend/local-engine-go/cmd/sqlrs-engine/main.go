package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"sqlrs/engine/internal/config"
	"sqlrs/engine/internal/conntrack"
	"sqlrs/engine/internal/dbms"
	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/httpapi"
	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/prepare/queue"
	"sqlrs/engine/internal/registry"
	runpkg "sqlrs/engine/internal/run"
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/statefs"
	"sqlrs/engine/internal/store/sqlite"
)

type EngineState struct {
	Endpoint   string `json:"endpoint"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"startedAt"`
	AuthToken  string `json:"authToken"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
}

type activityTracker struct {
	last int64
}

func newActivityTracker() *activityTracker {
	return &activityTracker{last: time.Now().UnixNano()}
}

func (a *activityTracker) Touch() {
	atomic.StoreInt64(&a.last, time.Now().UnixNano())
}

func (a *activityTracker) IdleFor() time.Duration {
	last := atomic.LoadInt64(&a.last)
	return time.Since(time.Unix(0, last))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

type statusRecorderFlusher struct {
	*statusRecorder
	flusher http.Flusher
}

func (r *statusRecorderFlusher) Flush() {
	if r.flusher != nil {
		r.flusher.Flush()
	}
}

// resolveContainerRuntimeBinary returns the executable name or path for the container runtime
// (docker/podman). Prefers SQLRS_CONTAINER_RUNTIME env; otherwise tries "docker", then "podman".
// When using podman on macOS, ensures CONTAINER_HOST is set from the default connection so
// the runtime can reach the podman machine.
func resolveContainerRuntimeBinary() string {
	if name := strings.TrimSpace(os.Getenv("SQLRS_CONTAINER_RUNTIME")); name != "" {
		if filepath.IsAbs(name) {
			return name
		}
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
		return name
	}
	var binary string
	for _, name := range []string{"docker", "podman"} {
		if path, err := exec.LookPath(name); err == nil {
			binary = path
			break
		}
	}
	if binary == "" {
		return "docker"
	}
	if os.Getenv("CONTAINER_HOST") != "" {
		return binary
	}
	if strings.Contains(strings.ToLower(filepath.Base(binary)), "podman") {
		if uri := podmanDefaultConnectionURI(binary); uri != "" {
			os.Setenv("CONTAINER_HOST", uri)
		}
	}
	return binary
}

// podmanDefaultConnectionURI runs `podman system connection list` and returns the URI of the default connection.
// Used on macOS so child podman invocations can reach the podman machine.
func podmanDefaultConnectionURI(podmanPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, podmanPath, "system", "connection", "list", "--format", "{{.URI}}")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		uri := strings.TrimSpace(line)
		if uri != "" {
			return uri
		}
	}
	return ""
}

func snapshotBackendFromConfig(cfg config.Store) string {
	value, err := cfg.Get("snapshot.backend", true)
	if err != nil {
		return "auto"
	}
	backend, ok := value.(string)
	if !ok {
		return "auto"
	}
	switch backend {
	case "auto", "overlay", "btrfs", "copy":
		return backend
	default:
		return "auto"
	}
}

func logLevelFromConfig(cfg config.Store) string {
	if cfg == nil {
		return ""
	}
	value, err := cfg.Get("log.level", true)
	if err != nil {
		return ""
	}
	level, ok := value.(string)
	if !ok {
		return ""
	}
	return level
}

func buildSummary() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "unknown"
	}
	var revision, buildTime, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			buildTime = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" && buildTime == "" && modified == "" {
		if info.GoVersion != "" {
			return "go=" + info.GoVersion
		}
		return "unknown"
	}
	if modified == "" {
		modified = "false"
	}
	return fmt.Sprintf("rev=%s time=%s modified=%s", revision, buildTime, modified)
}

var serveHTTP = func(server *http.Server, listener net.Listener) error {
	return server.Serve(listener)
}

var exitFn = os.Exit
var randReader = rand.Reader
var writeFileFn = os.WriteFile
var renameFn = os.Rename
var idleTickerEvery = time.Second
var runMountCommandFn = runMountCommand
var openDBFn = func(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", path)
}
var newStoreFn = sqlite.New
var newQueueFn = queue.New
var newPrepareManagerFn = prepare.NewManager
var prepareRecoverFn = func(mgr *prepare.Manager) error {
	return mgr.Recover(context.Background())
}
var newDeletionManagerFn = deletion.NewManager
var newRunManagerFn = runpkg.NewManager
var newHandlerFn = httpapi.NewHandler
var serverShutdownFn = func(server *http.Server, ctx context.Context) error {
	return server.Shutdown(ctx)
}
var isCharDeviceFn = isCharDevice
var jsonMarshalIndent = json.MarshalIndent

func run(args []string) (int, error) {
	fs := flag.NewFlagSet("sqlrs-engine", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	listenAddr := fs.String("listen", "", "listen address (host:port)")
	runDir := fs.String("run-dir", "", "runtime directory (unused in MVP)")
	statePath := fs.String("write-engine-json", "", "path to engine.json")
	idleTimeout := fs.Duration("idle-timeout", 30*time.Second, "shutdown after this idle duration")
	version := fs.String("version", "dev", "engine version")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}

	log.SetFlags(log.LstdFlags | log.LUTC)

	if *listenAddr == "" {
		return 2, errors.New("missing --listen")
	}
	if *statePath == "" {
		return 2, errors.New("missing --write-engine-json")
	}
	if *runDir != "" {
		if err := os.MkdirAll(*runDir, 0o700); err != nil {
			return 1, fmt.Errorf("create run dir: %v", err)
		}
	}
	closeLog, err := setupLogging(*statePath)
	if err != nil {
		log.Printf("engine log setup failed: %v", err)
	} else {
		defer closeLog()
	}
	log.Printf("sqlrs-engine version=%s build=%s", *version, buildSummary())

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return 1, fmt.Errorf("listen: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	instanceID, err := randomHex(16)
	if err != nil {
		return 1, fmt.Errorf("instance id: %v", err)
	}
	authToken, err := randomHex(32)
	if err != nil {
		return 1, fmt.Errorf("auth token: %v", err)
	}

	stateDir := filepath.Dir(*statePath)
	stateStoreRoot := strings.TrimSpace(os.Getenv("SQLRS_STATE_STORE"))
	if stateStoreRoot == "" {
		stateStoreRoot = filepath.Join(stateDir, "state-store")
	}
	if err := ensureWSLMount(stateStoreRoot); err != nil {
		return 1, fmt.Errorf("wsl mount: %v", err)
	}
	if err := os.MkdirAll(stateStoreRoot, 0o700); err != nil {
		return 1, fmt.Errorf("create state store root: %v", err)
	}
	db, err := openDBFn(filepath.Join(stateStoreRoot, "state.db"))
	if err != nil {
		return 1, fmt.Errorf("open state db: %v", err)
	}
	defer db.Close()

	store, err := newStoreFn(db)
	if err != nil {
		return 1, fmt.Errorf("open state db: %v", err)
	}

	queueStore, err := newQueueFn(db)
	if err != nil {
		return 1, fmt.Errorf("open queue db: %v", err)
	}

	state := EngineState{
		Endpoint:   listener.Addr().String(),
		PID:        os.Getpid(),
		StartedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		AuthToken:  authToken,
		Version:    *version,
		InstanceID: instanceID,
	}
	if err := writeEngineState(*statePath, state); err != nil {
		return 1, fmt.Errorf("write engine.json: %v", err)
	}
	defer removeEngineState(*statePath)

	activity := newActivityTracker()
	activity.Touch()
	reg := registry.New(store)
	containerBinary := resolveContainerRuntimeBinary()
	rt := engineRuntime.NewDocker(engineRuntime.Options{Binary: containerBinary})
	configMgr, err := config.NewManager(config.Options{StateStoreRoot: stateStoreRoot})
	if err != nil {
		return 1, fmt.Errorf("config manager: %v", err)
	}
	stateFS := statefs.NewManager(statefs.Options{
		Backend:        snapshotBackendFromConfig(configMgr),
		StateStoreRoot: stateStoreRoot,
	})
	connector := dbms.NewPostgres(rt, dbms.WithLogLevel(func() string {
		return logLevelFromConfig(configMgr)
	}))
	prepareMgr, err := newPrepareManagerFn(prepare.Options{
		Store:          store,
		Queue:          queueStore,
		Runtime:        rt,
		StateFS:        stateFS,
		DBMS:           connector,
		StateStoreRoot: stateStoreRoot,
		Config:         configMgr,
		Version:        *version,
		Async:          true,
	})
	if err != nil {
		return 1, fmt.Errorf("prepare manager: %v", err)
	}
	if err := prepareRecoverFn(prepareMgr); err != nil {
		return 1, fmt.Errorf("prepare recovery: %v", err)
	}

	deleteMgr, err := newDeletionManagerFn(deletion.Options{
		Store:          store,
		Conn:           conntrack.Noop{},
		Runtime:        rt,
		StateFS:        stateFS,
		StateStoreRoot: stateStoreRoot,
	})
	if err != nil {
		return 1, fmt.Errorf("delete manager: %v", err)
	}

	runMgr, err := newRunManagerFn(runpkg.Options{
		Registry: reg,
		Runtime:  rt,
	})
	if err != nil {
		return 1, fmt.Errorf("run manager: %v", err)
	}

	mux := newHandlerFn(httpapi.Options{
		Version:    *version,
		InstanceID: instanceID,
		AuthToken:  authToken,
		Registry:   reg,
		Prepare:    prepareMgr,
		Deletion:   deleteMgr,
		Run:        runMgr,
		Config:     configMgr,
	})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activity.Touch()
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			writer := http.ResponseWriter(rec)
			if flusher, ok := w.(http.Flusher); ok {
				writer = &statusRecorderFlusher{statusRecorder: rec, flusher: flusher}
			}
			mux.ServeHTTP(writer, r)
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf("http request method=%s path=%s status=%d bytes=%d dur=%s remote=%s", r.Method, r.URL.RequestURI(), status, rec.bytes, time.Since(start).Truncate(time.Millisecond), r.RemoteAddr)
		}),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var shutdownOnce sync.Once
	shutdown := func(reason string) {
		shutdownOnce.Do(func() {
			log.Printf("shutting down: %s", reason)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := serverShutdownFn(server, shutdownCtx); err != nil {
				log.Printf("shutdown error: %v", err)
			}
		})
	}

	if *idleTimeout > 0 {
		go func() {
			ticker := time.NewTicker(idleTickerEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if activity.IdleFor() >= *idleTimeout {
						shutdown("idle timeout")
						return
					}
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		shutdown("signal")
	}()

	log.Printf("sqlrs-engine listening on %s", state.Endpoint)
	if err := serveHTTP(server, listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server error: %v", err)
		return 1, nil
	}
	return 0, nil
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		log.Printf("engine error: %v", err)
		fmt.Fprintln(os.Stderr, err.Error())
	}
	if code != 0 {
		exitFn(code)
	}
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := randReader.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func writeEngineState(path string, state EngineState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := jsonMarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := writeFileFn(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return renameFn(tmp, path)
}

func removeEngineState(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove engine.json: %v", err)
	}
}

func setupLogging(statePath string) (func(), error) {
	logDir := filepath.Join(filepath.Dir(statePath), "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, "engine.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	output := io.Writer(logFile)
	if isCharDeviceFn(os.Stderr) {
		output = io.MultiWriter(logFile, os.Stderr)
	}
	log.SetOutput(output)
	return func() {
		_ = logFile.Close()
	}, nil
}

func isCharDevice(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func ensureWSLMount(stateStoreRoot string) error {
	unit := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_UNIT"))
	fstype := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_FSTYPE"))
	if unit == "" && fstype == "" {
		return nil
	}
	if unit == "" {
		return fmt.Errorf("SQLRS_WSL_MOUNT_UNIT must be set")
	}
	if strings.TrimSpace(stateStoreRoot) == "" {
		return fmt.Errorf("SQLRS_STATE_STORE is required to mount WSL device")
	}
	if fstype == "" {
		fstype = "btrfs"
	}
	if err := os.MkdirAll(stateStoreRoot, 0o700); err != nil {
		return err
	}
	active, err := isSystemdUnitActive(unit)
	if err != nil {
		return fmt.Errorf("mount unit check failed: %w", err)
	}
	if !active {
		if _, err := runMountCommandFn("systemctl", "start", unit); err != nil {
			return fmt.Errorf("mount unit start failed: %w", err)
		}
		active, err = isSystemdUnitActive(unit)
		if err != nil {
			return fmt.Errorf("mount unit check failed: %w", err)
		}
		if !active {
			return fmt.Errorf("mount unit is not active")
		}
	}
	fsType, mounted, err := findmntFSType(stateStoreRoot)
	if err != nil {
		return err
	}
	if !mounted || fsType == "" {
		return fmt.Errorf("mount verification failed for %s", stateStoreRoot)
	}
	if fsType != fstype {
		return fmt.Errorf("mounted filesystem is %s, expected %s", fsType, fstype)
	}
	return nil
}

func findmntFSType(target string) (string, bool, error) {
	out, err := runMountCommandFn("findmnt", "-n", "-o", "FSTYPE", "-T", target)
	if err == nil {
		return strings.TrimSpace(out), true, nil
	}
	if isExitStatus(err, 1) {
		return "", false, nil
	}
	return "", false, err
}

func isExitStatus(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

func isSystemdUnitActive(unit string) (bool, error) {
	out, err := runMountCommandFn("systemctl", "is-active", unit)
	if err != nil {
		if isExitStatus(err, 3) || isExitStatus(err, 4) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "active", nil
}

func runMountCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
