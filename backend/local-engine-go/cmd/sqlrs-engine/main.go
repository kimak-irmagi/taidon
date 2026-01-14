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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type EngineState struct {
	Endpoint   string `json:"endpoint"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"startedAt"`
	AuthToken  string `json:"authToken"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
}

type HealthResponse struct {
	Ok         bool   `json:"ok"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
	PID        int    `json:"pid"`
}

type NameEntry struct {
	Name             string  `json:"name"`
	InstanceID       *string `json:"instance_id,omitempty"`
	ImageID          string  `json:"image_id"`
	StateID          string  `json:"state_id"`
	StateFingerprint string  `json:"state_fingerprint,omitempty"`
	Status           string  `json:"status"`
	LastUsedAt       *string `json:"last_used_at,omitempty"`
}

type InstanceEntry struct {
	InstanceID string  `json:"instance_id"`
	ImageID    string  `json:"image_id"`
	StateID    string  `json:"state_id"`
	Name       *string `json:"name,omitempty"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	Status     string  `json:"status"`
}

type StateEntry struct {
	StateID             string `json:"state_id"`
	ImageID             string `json:"image_id"`
	PrepareKind         string `json:"prepare_kind"`
	PrepareArgs         string `json:"prepare_args_normalized"`
	CreatedAt           string `json:"created_at"`
	SizeBytes           *int64 `json:"size_bytes,omitempty"`
	RefCount            int    `json:"refcount"`
}

type NameFilters struct {
	InstanceID string
	StateID    string
	ImageID    string
}

type InstanceFilters struct {
	StateID string
	ImageID string
}

type StateFilters struct {
	Kind    string
	ImageID string
}

type registry struct {
	mu        sync.RWMutex
	names     map[string]NameEntry
	instances map[string]InstanceEntry
	states    map[string]StateEntry
}

func newRegistry() *registry {
	return &registry{
		names:     map[string]NameEntry{},
		instances: map[string]InstanceEntry{},
		states:    map[string]StateEntry{},
	}
}

func (r *registry) ListNames(filters NameFilters) []NameEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]NameEntry, 0, len(r.names))
	for _, entry := range r.names {
		if filters.InstanceID != "" {
			if entry.InstanceID == nil || *entry.InstanceID != filters.InstanceID {
				continue
			}
		}
		if filters.StateID != "" && entry.StateID != filters.StateID {
			continue
		}
		if filters.ImageID != "" && entry.ImageID != filters.ImageID {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (r *registry) ListInstances(filters InstanceFilters) []InstanceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]InstanceEntry, 0, len(r.instances))
	for _, entry := range r.instances {
		if filters.StateID != "" && entry.StateID != filters.StateID {
			continue
		}
		if filters.ImageID != "" && entry.ImageID != filters.ImageID {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (r *registry) ListStates(filters StateFilters) []StateEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]StateEntry, 0, len(r.states))
	for _, entry := range r.states {
		if filters.Kind != "" && entry.PrepareKind != filters.Kind {
			continue
		}
		if filters.ImageID != "" && entry.ImageID != filters.ImageID {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (r *registry) GetName(name string) (NameEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.names[name]
	return entry, ok
}

func (r *registry) GetInstanceByID(instanceID string) (InstanceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.instances[instanceID]
	return entry, ok
}

func (r *registry) GetInstanceByName(name string) (InstanceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	nameEntry, ok := r.names[name]
	if !ok || nameEntry.InstanceID == nil {
		return InstanceEntry{}, false
	}
	entry, ok := r.instances[*nameEntry.InstanceID]
	return entry, ok
}

func (r *registry) GetState(stateID string) (StateEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.states[stateID]
	return entry, ok
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
	reg := newRegistry()

	mux := buildMux(*version, instanceID, authToken, reg)

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

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func buildMux(version, instanceID, authToken string, reg *registry) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resp := HealthResponse{
			Ok:         true,
			Version:    version,
			InstanceID: instanceID,
			PID:        os.Getpid(),
		}
		writeJSON(w, resp)
	})

	mux.HandleFunc("/v1/names", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := NameFilters{
			InstanceID: readQueryValue(r, "instance"),
			StateID:    readQueryValue(r, "state"),
			ImageID:    readQueryValue(r, "image"),
		}
		writeListResponse(w, r, reg.ListNames(filters))
	})

	mux.HandleFunc("/v1/names/", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/v1/names/")
		if name == "" {
			http.NotFound(w, r)
			return
		}
		entry, ok := reg.GetName(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, entry)
	})

	mux.HandleFunc("/v1/instances", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := InstanceFilters{
			StateID: readQueryValue(r, "state"),
			ImageID: readQueryValue(r, "image"),
		}
		writeListResponse(w, r, reg.ListInstances(filters))
	})

	mux.HandleFunc("/v1/instances/", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		instanceID := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
		if instanceID == "" {
			http.NotFound(w, r)
			return
		}
		var entry InstanceEntry
		var ok bool
		resolvedByName := false
		if isInstanceID(instanceID) {
			entry, ok = reg.GetInstanceByID(instanceID)
			if !ok {
				entry, ok = reg.GetInstanceByName(instanceID)
				resolvedByName = ok
			}
		} else {
			entry, ok = reg.GetInstanceByName(instanceID)
			resolvedByName = ok
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		if resolvedByName {
			w.Header().Set("Location", "/v1/instances/"+entry.InstanceID)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		writeJSON(w, entry)
	})

	mux.HandleFunc("/v1/states", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := StateFilters{
			Kind:    readQueryValue(r, "kind"),
			ImageID: readQueryValue(r, "image"),
		}
		writeListResponse(w, r, reg.ListStates(filters))
	})

	mux.HandleFunc("/v1/states/", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, authToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		stateID := strings.TrimPrefix(r.URL.Path, "/v1/states/")
		if stateID == "" {
			http.NotFound(w, r)
			return
		}
		entry, ok := reg.GetState(stateID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, entry)
	})

	return mux
}

func writeListResponse[T any](w http.ResponseWriter, r *http.Request, items []T) {
	if wantsNDJSON(r) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		for _, item := range items {
			_ = enc.Encode(item)
		}
		return
	}
	writeJSON(w, items)
}

func wantsNDJSON(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "application/x-ndjson")
}

func requireAuth(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "Bearer "+token {
		return true
	}
	w.WriteHeader(http.StatusUnauthorized)
	return false
}

func readQueryValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func isInstanceID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return false
	}
	return true
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
