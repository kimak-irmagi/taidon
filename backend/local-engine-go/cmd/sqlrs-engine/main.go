package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

	"sqlrs/engine/internal/httpapi"
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

func main() {
	listenAddr := flag.String("listen", "", "listen address (host:port)")
	runDir := flag.String("run-dir", "", "runtime directory (unused in MVP)")
	statePath := flag.String("write-engine-json", "", "path to engine.json")
	idleTimeout := flag.Duration("idle-timeout", 30*time.Second, "shutdown after this idle duration")
	version := flag.String("version", "dev", "engine version")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.LUTC)

	if *listenAddr == "" {
		fmt.Fprintln(os.Stderr, "missing --listen")
		os.Exit(2)
	}
	if *statePath == "" {
		fmt.Fprintln(os.Stderr, "missing --write-engine-json")
		os.Exit(2)
	}
	if *runDir != "" {
		if err := os.MkdirAll(*runDir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "create run dir: %v\n", err)
			os.Exit(1)
		}
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	instanceID, err := randomHex(16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "instance id: %v\n", err)
		os.Exit(1)
	}
	authToken, err := randomHex(32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth token: %v\n", err)
		os.Exit(1)
	}

	stateDir := filepath.Dir(*statePath)
	store, err := sqlite.Open(filepath.Join(stateDir, "state.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open state db: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "write engine.json: %v\n", err)
		os.Exit(1)
	}
	defer removeEngineState(*statePath)

	tracker := newActivityTracker()
	tracker.Touch()
	reg := registry.New(store)

	mux := httpapi.NewHandler(httpapi.Options{
		Version:    *version,
		InstanceID: instanceID,
		AuthToken:  authToken,
		Registry:   reg,
	})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracker.Touch()
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
					if tracker.IdleFor() >= *idleTimeout {
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
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
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
