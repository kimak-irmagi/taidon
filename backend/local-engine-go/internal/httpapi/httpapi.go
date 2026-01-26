package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sqlrs/engine/internal/auth"
	"sqlrs/engine/internal/config"
	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/run"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/stream"
)

type Options struct {
	Version    string
	InstanceID string
	AuthToken  string
	Registry   *registry.Registry
	Prepare    *prepare.Manager
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

	mux.HandleFunc("/v1/config/schema", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.Config == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, opts.Config.Schema())
	})

	mux.HandleFunc("/v1/config", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if opts.Config == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch r.Method {
		case http.MethodGet:
			path := readQueryValue(r, "path")
			effective, err := parseBoolQuery(r, "effective")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid effective", err.Error(), http.StatusBadRequest)
				return
			}
			value, err := opts.Config.Get(path, effective)
			if err != nil {
				if errors.Is(err, config.ErrInvalidPath) {
					writeErrorResponse(w, "invalid_argument", "invalid path", err.Error(), http.StatusBadRequest)
					return
				}
				if errors.Is(err, config.ErrPathNotFound) {
					writeErrorResponse(w, "not_found", "path not found", err.Error(), http.StatusNotFound)
					return
				}
				writeErrorResponse(w, "internal_error", "cannot read config", err.Error(), http.StatusInternalServerError)
				return
			}
			if path == "" {
				writeJSON(w, value)
				return
			}
			writeJSON(w, config.Value{Path: path, Value: value})
		case http.MethodPatch:
			var req config.Value
			decoder := json.NewDecoder(r.Body)
			decoder.UseNumber()
			if err := decoder.Decode(&req); err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid json payload", err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Path) == "" {
				writeErrorResponse(w, "invalid_argument", "path is required", "", http.StatusBadRequest)
				return
			}
			value, err := opts.Config.Set(req.Path, req.Value)
			if err != nil {
				if errors.Is(err, config.ErrInvalidPath) || errors.Is(err, config.ErrInvalidValue) {
					writeErrorResponse(w, "invalid_argument", "invalid config value", err.Error(), http.StatusBadRequest)
					return
				}
				writeErrorResponse(w, "internal_error", "cannot update config", err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, config.Value{Path: req.Path, Value: value})
		case http.MethodDelete:
			path := readQueryValue(r, "path")
			if path == "" {
				writeErrorResponse(w, "invalid_argument", "path is required", "", http.StatusBadRequest)
				return
			}
			value, err := opts.Config.Remove(path)
			if err != nil {
				if errors.Is(err, config.ErrInvalidPath) {
					writeErrorResponse(w, "invalid_argument", "invalid path", err.Error(), http.StatusBadRequest)
					return
				}
				if errors.Is(err, config.ErrPathNotFound) {
					writeErrorResponse(w, "not_found", "path not found", err.Error(), http.StatusNotFound)
					return
				}
				writeErrorResponse(w, "internal_error", "cannot update config", err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, config.Value{Path: path, Value: value})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/prepare-jobs", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if opts.Prepare == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch r.Method {
		case http.MethodGet:
			entries := opts.Prepare.ListJobs(readQueryValue(r, "job"))
			_ = stream.WriteList(w, r, entries)
		case http.MethodPost:
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
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(accepted)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.Run == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req run.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorResponse(w, "invalid_argument", "invalid json payload", err.Error(), http.StatusBadRequest)
			return
		}
		result, err := opts.Run.Run(r.Context(), req)
		if err != nil {
			switch err.(type) {
			case run.ValidationError, *run.ValidationError:
				writeErrorResponse(w, "invalid_argument", err.Error(), "", http.StatusBadRequest)
				return
			case run.NotFoundError, *run.NotFoundError:
				writeErrorResponse(w, "not_found", err.Error(), "", http.StatusNotFound)
				return
			case run.ConflictError, *run.ConflictError:
				writeErrorResponse(w, "conflict", err.Error(), "", http.StatusConflict)
				return
			default:
				writeErrorResponse(w, "internal_error", "run failed", err.Error(), http.StatusInternalServerError)
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
	})

	mux.HandleFunc("/v1/prepare-jobs/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.RequireBearer(w, r, opts.AuthToken) {
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
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
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
		switch r.Method {
		case http.MethodGet:
			status, ok := opts.Prepare.Get(path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, status)
		case http.MethodDelete:
			force, err := parseBoolQuery(r, "force")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid force", err.Error(), http.StatusBadRequest)
				return
			}
			dryRun, err := parseBoolQuery(r, "dry_run")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid dry_run", err.Error(), http.StatusBadRequest)
				return
			}
			result, ok := opts.Prepare.Delete(path, deletion.DeleteOptions{
				Force:  force,
				DryRun: dryRun,
			})
			if !ok {
				writeErrorResponse(w, "not_found", "job not found", "", http.StatusNotFound)
				return
			}
			status := http.StatusOK
			if !dryRun && result.Outcome == deletion.OutcomeBlocked {
				status = http.StatusConflict
			}
			writeJSONStatus(w, result, status)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
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
		entries := opts.Prepare.ListTasks(readQueryValue(r, "job"))
		_ = stream.WriteList(w, r, entries)
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
		idPrefix, err := normalizeIDPrefix(readQueryValue(r, "id_prefix"))
		if err != nil {
			writeError(w, prepare.ErrorResponse{
				Code:    "invalid_argument",
				Message: "invalid id_prefix",
				Details: err.Error(),
			}, http.StatusBadRequest)
			return
		}
		filters := store.InstanceFilters{
			StateID:  readQueryValue(r, "state"),
			ImageID:  readQueryValue(r, "image"),
			IDPrefix: idPrefix,
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
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		idOrName := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
		if idOrName == "" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			if opts.Deletion == nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			force, err := parseBoolQuery(r, "force")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid force", err.Error(), http.StatusBadRequest)
				return
			}
			dryRun, err := parseBoolQuery(r, "dry_run")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid dry_run", err.Error(), http.StatusBadRequest)
				return
			}
			result, found, err := opts.Deletion.DeleteInstance(r.Context(), idOrName, deletion.DeleteOptions{
				Force:  force,
				DryRun: dryRun,
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !found {
				writeErrorResponse(w, "not_found", "instance not found", "", http.StatusNotFound)
				return
			}
			status := http.StatusOK
			if !dryRun && result.Outcome == deletion.OutcomeBlocked {
				status = http.StatusConflict
			}
			writeJSONStatus(w, result, status)
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
		idPrefix, err := normalizeIDPrefix(readQueryValue(r, "id_prefix"))
		if err != nil {
			writeError(w, prepare.ErrorResponse{
				Code:    "invalid_argument",
				Message: "invalid id_prefix",
				Details: err.Error(),
			}, http.StatusBadRequest)
			return
		}
		filters := store.StateFilters{
			Kind:     readQueryValue(r, "kind"),
			ImageID:  readQueryValue(r, "image"),
			IDPrefix: idPrefix,
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
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		stateID := strings.TrimPrefix(r.URL.Path, "/v1/states/")
		if stateID == "" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			if opts.Deletion == nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			recurse, err := parseBoolQuery(r, "recurse")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid recurse", err.Error(), http.StatusBadRequest)
				return
			}
			force, err := parseBoolQuery(r, "force")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid force", err.Error(), http.StatusBadRequest)
				return
			}
			dryRun, err := parseBoolQuery(r, "dry_run")
			if err != nil {
				writeErrorResponse(w, "invalid_argument", "invalid dry_run", err.Error(), http.StatusBadRequest)
				return
			}
			result, found, err := opts.Deletion.DeleteState(r.Context(), stateID, deletion.DeleteOptions{
				Recurse: recurse,
				Force:   force,
				DryRun:  dryRun,
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !found {
				writeErrorResponse(w, "not_found", "state not found", "", http.StatusNotFound)
				return
			}
			status := http.StatusOK
			if !dryRun && result.Outcome == deletion.OutcomeBlocked {
				status = http.StatusConflict
			}
			writeJSONStatus(w, result, status)
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

func writeJSONStatus(w http.ResponseWriter, payload any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, payload prepare.ErrorResponse, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErrorResponse(w http.ResponseWriter, code, message, details string, status int) {
	writeError(w, prepare.ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}, status)
}

func readQueryValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func parseBoolQuery(r *http.Request, key string) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func normalizeIDPrefix(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) < 8 {
		return "", fmt.Errorf("id_prefix must be at least 8 hex characters")
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return "", fmt.Errorf("id_prefix must be hex")
	}
	return strings.ToLower(value), nil
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
