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
	"os/signal"
	"path/filepath"
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
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/snapshot"
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

var serveHTTP = func(server *http.Server, listener net.Listener) error {
	return server.Serve(listener)
}

var exitFn = os.Exit
var randReader = rand.Reader
var writeFileFn = os.WriteFile
var renameFn = os.Rename
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
var isCharDeviceFn = isCharDevice

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
	stateStoreRoot := filepath.Join(stateDir, "state-store")
	if err := os.MkdirAll(stateStoreRoot, 0o700); err != nil {
		return 1, fmt.Errorf("create state store root: %v", err)
	}
	db, err := openDBFn(filepath.Join(stateDir, "state.db"))
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
	rt := engineRuntime.NewDocker(engineRuntime.Options{})
	snap := snapshot.NewManager(snapshot.Options{PreferOverlay: true})
	connector := dbms.NewPostgres(rt)
	configMgr, err := config.NewManager(config.Options{StateStoreRoot: stateStoreRoot})
	if err != nil {
		return 1, fmt.Errorf("config manager: %v", err)
	}
	prepareMgr, err := newPrepareManagerFn(prepare.Options{
		Store:          store,
		Queue:          queueStore,
		Runtime:        rt,
		Snapshot:       snap,
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
		StateStoreRoot: stateStoreRoot,
	})
	if err != nil {
		return 1, fmt.Errorf("delete manager: %v", err)
	}

	mux := httpapi.NewHandler(httpapi.Options{
		Version:    *version,
		InstanceID: instanceID,
		AuthToken:  authToken,
		Registry:   reg,
		Prepare:    prepareMgr,
		Deletion:   deleteMgr,
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
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Printf("shutdown error: %v", err)
			}
		})
	}

	if *idleTimeout > 0 {
		go func() {
			ticker := time.NewTicker(time.Second)
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
	data, err := json.MarshalIndent(state, "", "  ")
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
