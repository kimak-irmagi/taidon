package httpapi

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/sqlrs/engine-local/internal/config"
	"github.com/sqlrs/engine-local/internal/deletion"
	"github.com/sqlrs/engine-local/internal/prepare"
	"github.com/sqlrs/engine-local/internal/registry"
	"github.com/sqlrs/engine-local/internal/run"
)

type Options struct {
	Version    string
	InstanceID string
	AuthToken  string
	Registry   *registry.Registry
	Prepare    *prepare.PrepareService
	Deletion   *deletion.Manager
	Run        *run.Manager
	Config     config.Store
}

type healthResponse struct {
	Ok         bool   `json:"ok"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
	PID        int    `json:"pid"`
}

func NewHandler(opts Options) http.Handler {
	mux := http.NewServeMux()
	registerHealthRoutes(mux, opts)
	cacheRoutes{opts: opts}.register(mux)
	configRoutes{opts: opts}.register(mux)
	prepareRoutes{opts: opts}.register(mux)
	runRoutes{opts: opts}.register(mux)
	registryRoutes{opts: opts}.register(mux)
	return mux
}

func registerHealthRoutes(mux *http.ServeMux, opts Options) {
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		_ = writeJSON(w, healthResponse{
			Ok:         true,
			Version:    opts.Version,
			InstanceID: opts.InstanceID,
			PID:        os.Getpid(),
		})
	})
}

func streamPrepareEvents(w http.ResponseWriter, r *http.Request, mgr *prepare.PrepareService, jobID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	index := 0
	for {
		events, ok, done, err := mgr.EventsSince(jobID, index)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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
		if len(events) == 0 {
			if err := mgr.WaitForEvent(r.Context(), jobID, index); err != nil {
				return
			}
		}
	}
}
