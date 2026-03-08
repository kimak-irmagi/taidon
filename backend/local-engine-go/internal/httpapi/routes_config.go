package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sqlrs/engine-local/internal/auth"
	"github.com/sqlrs/engine-local/internal/config"
)

// configRoutes keeps config HTTP behavior isolated per resource as described in
// docs/architecture/local-engine-cli-maintainability-refactor.md.
type configRoutes struct {
	opts Options
}

func (routes configRoutes) register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/config/schema", routes.handleSchema)
	mux.HandleFunc("/v1/config", routes.handleConfig)
}

func (routes configRoutes) handleSchema(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if routes.opts.Config == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = writeJSON(w, routes.opts.Config.Schema())
}

func (routes configRoutes) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if routes.opts.Config == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		path := readQueryValue(r, "path")
		effective, err := parseBoolQuery(r, "effective")
		if err != nil {
			_ = writeErrorResponse(w, "invalid_argument", "invalid effective", err.Error(), http.StatusBadRequest)
			return
		}
		value, err := routes.opts.Config.Get(path, effective)
		if err != nil {
			if errors.Is(err, config.ErrInvalidPath) {
				_ = writeErrorResponse(w, "invalid_argument", "invalid path", err.Error(), http.StatusBadRequest)
				return
			}
			if errors.Is(err, config.ErrPathNotFound) {
				_ = writeErrorResponse(w, "not_found", "path not found", err.Error(), http.StatusNotFound)
				return
			}
			_ = writeErrorResponse(w, "internal_error", "cannot read config", err.Error(), http.StatusInternalServerError)
			return
		}
		if path == "" {
			_ = writeJSON(w, value)
			return
		}
		_ = writeJSON(w, config.Value{Path: path, Value: value})
	case http.MethodPatch:
		var req config.Value
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&req); err != nil {
			_ = writeErrorResponse(w, "invalid_argument", "invalid json payload", err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Path) == "" {
			_ = writeErrorResponse(w, "invalid_argument", "path is required", "", http.StatusBadRequest)
			return
		}
		value, err := routes.opts.Config.Set(req.Path, req.Value)
		if err != nil {
			if errors.Is(err, config.ErrInvalidPath) || errors.Is(err, config.ErrInvalidValue) {
				_ = writeErrorResponse(w, "invalid_argument", "invalid config value", err.Error(), http.StatusBadRequest)
				return
			}
			_ = writeErrorResponse(w, "internal_error", "cannot update config", err.Error(), http.StatusInternalServerError)
			return
		}
		_ = writeJSON(w, config.Value{Path: req.Path, Value: value})
	case http.MethodDelete:
		path := readQueryValue(r, "path")
		if path == "" {
			_ = writeErrorResponse(w, "invalid_argument", "path is required", "", http.StatusBadRequest)
			return
		}
		value, err := routes.opts.Config.Remove(path)
		if err != nil {
			if errors.Is(err, config.ErrInvalidPath) {
				_ = writeErrorResponse(w, "invalid_argument", "invalid path", err.Error(), http.StatusBadRequest)
				return
			}
			if errors.Is(err, config.ErrPathNotFound) {
				_ = writeErrorResponse(w, "not_found", "path not found", err.Error(), http.StatusNotFound)
				return
			}
			_ = writeErrorResponse(w, "internal_error", "cannot update config", err.Error(), http.StatusInternalServerError)
			return
		}
		_ = writeJSON(w, config.Value{Path: path, Value: value})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
