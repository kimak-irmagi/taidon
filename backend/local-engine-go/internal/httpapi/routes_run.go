package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sqlrs/engine-local/internal/auth"
	"github.com/sqlrs/engine-local/internal/run"
)

type runRoutes struct {
	opts Options
}

func (routes runRoutes) register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/runs", routes.handleRuns)
}

func (routes runRoutes) handleRuns(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if routes.opts.Run == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var req run.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid json payload", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := routes.opts.Run.Run(r.Context(), req)
	if err != nil {
		switch err.(type) {
		case run.ValidationError, *run.ValidationError:
			_ = writeErrorResponse(w, "invalid_argument", err.Error(), "", http.StatusBadRequest)
			return
		case run.NotFoundError, *run.NotFoundError:
			_ = writeErrorResponse(w, "not_found", err.Error(), "", http.StatusNotFound)
			return
		case run.ConflictError, *run.ConflictError:
			_ = writeErrorResponse(w, "conflict", err.Error(), "", http.StatusConflict)
			return
		default:
			_ = writeErrorResponse(w, "internal_error", "run failed", err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	for _, evt := range result.Events {
		_ = enc.Encode(evt)
	}
	_ = enc.Encode(run.Event{
		Type:       "start",
		Ts:         time.Now().UTC().Format(time.RFC3339Nano),
		InstanceID: result.InstanceID,
	})
	if strings.TrimSpace(result.Stdout) != "" {
		_ = enc.Encode(run.Event{
			Type: "stdout",
			Ts:   time.Now().UTC().Format(time.RFC3339Nano),
			Data: result.Stdout,
		})
	}
	if strings.TrimSpace(result.Stderr) != "" {
		_ = enc.Encode(run.Event{
			Type: "stderr",
			Ts:   time.Now().UTC().Format(time.RFC3339Nano),
			Data: result.Stderr,
		})
	}
	exitCode := result.ExitCode
	_ = enc.Encode(run.Event{
		Type:     "exit",
		Ts:       time.Now().UTC().Format(time.RFC3339Nano),
		ExitCode: &exitCode,
	})
}
