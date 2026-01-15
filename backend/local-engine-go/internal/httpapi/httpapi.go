package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"sqlrs/engine/internal/auth"
	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/stream"
)

type Options struct {
	Version    string
	InstanceID string
	AuthToken  string
	Registry   *registry.Registry
	Prepare    *prepare.Manager
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

	mux.HandleFunc("/v1/prepare-jobs", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.Prepare == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req prepare.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, prepare.ErrorResponse{
				Code:    "invalid_argument",
				Message: "invalid json payload",
				Details: err.Error(),
			}, http.StatusBadRequest)
			return
		}
		accepted, err := opts.Prepare.Submit(r.Context(), req)
		if err != nil {
			resp := prepare.ToErrorResponse(err)
			status := http.StatusInternalServerError
			if _, ok := err.(prepare.ValidationError); ok {
				status = http.StatusBadRequest
			}
			if _, ok := err.(*prepare.ValidationError); ok {
				status = http.StatusBadRequest
			}
			writeError(w, *resp, status)
			return
		}
		w.Header().Set("Location", accepted.StatusURL)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(accepted)
	})

	mux.HandleFunc("/v1/prepare-jobs/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.Prepare == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/v1/prepare-jobs/")
		if path == "" {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(path, "/events") {
			jobID := strings.TrimSuffix(path, "/events")
			if jobID == "" || strings.Contains(jobID, "/") {
				http.NotFound(w, r)
				return
			}
			streamPrepareEvents(w, r, opts.Prepare, jobID)
			return
		}
		if strings.Contains(path, "/") {
			http.NotFound(w, r)
			return
		}
		status, ok := opts.Prepare.Get(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, status)
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

func writeError(w http.ResponseWriter, payload prepare.ErrorResponse, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readQueryValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func streamPrepareEvents(w http.ResponseWriter, r *http.Request, mgr *prepare.Manager, jobID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	index := 0
	for {
		events, ok, done := mgr.EventsSince(jobID, index)
		if !ok {
			http.NotFound(w, r)
			return
		}
		for _, event := range events {
			_ = enc.Encode(event)
			flusher.Flush()
			index++
		}
		if done {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}
