package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"sqlrs/engine/internal/auth"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/stream"
)

type Options struct {
	Version    string
	InstanceID string
	AuthToken  string
	Registry   *registry.Registry
}

type healthResponse struct {
	Ok         bool   `json:"ok"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
	PID        int    `json:"pid"`
}

func NewHandler(opts Options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resp := healthResponse{
			Ok:         true,
			Version:    opts.Version,
			InstanceID: opts.InstanceID,
			PID:        os.Getpid(),
		}
		writeJSON(w, resp)
	})

	mux.HandleFunc("/v1/names", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := store.NameFilters{
			InstanceID: readQueryValue(r, "instance"),
			StateID:    readQueryValue(r, "state"),
			ImageID:    readQueryValue(r, "image"),
		}
		entries, err := opts.Registry.ListNames(r.Context(), filters)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = stream.WriteList(w, r, entries)
	})

	mux.HandleFunc("/v1/names/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
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
		entry, ok, err := opts.Registry.GetName(r.Context(), name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, entry)
	})

	mux.HandleFunc("/v1/instances", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := store.InstanceFilters{
			StateID: readQueryValue(r, "state"),
			ImageID: readQueryValue(r, "image"),
		}
		entries, err := opts.Registry.ListInstances(r.Context(), filters)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = stream.WriteList(w, r, entries)
	})

	mux.HandleFunc("/v1/instances/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		idOrName := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
		if idOrName == "" {
			http.NotFound(w, r)
			return
		}
		entry, ok, resolvedByName, err := opts.Registry.GetInstance(r.Context(), idOrName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
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
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		filters := store.StateFilters{
			Kind:    readQueryValue(r, "kind"),
			ImageID: readQueryValue(r, "image"),
		}
		entries, err := opts.Registry.ListStates(r.Context(), filters)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = stream.WriteList(w, r, entries)
	})

	mux.HandleFunc("/v1/states/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
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
		entry, ok, err := opts.Registry.GetState(r.Context(), stateID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, entry)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func readQueryValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}
