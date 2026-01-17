package main

import (
	"context"
	"crypto/rand"
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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"sqlrs/engine/internal/conntrack"
	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/httpapi"
	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
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

var serveHTTP = func(server *http.Server, listener net.Listener) error {
	return server.Serve(listener)
}

var exitFn = os.Exit
var randReader = rand.Reader

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
	store, err := sqlite.Open(filepath.Join(stateDir, "state.db"))
	if err != nil {
		return 1, fmt.Errorf("open state db: %v", err)
	}
	defer store.Close()

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
	prepareMgr, err := prepare.NewManager(prepare.Options{
		Store:   store,
		Version: *version,
		Async:   true,
	})
	if err != nil {
		return 1, fmt.Errorf("prepare manager: %v", err)
	}

	deleteMgr, err := deletion.NewManager(deletion.Options{
		Store: store,
		Conn:  conntrack.Noop{},
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
	})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activity.Touch()
			mux.ServeHTTP(w, r)
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
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, path)
}

func removeEngineState(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove engine.json: %v", err)
	}
}
