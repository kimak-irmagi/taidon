package httpapi

import (
	"net/http"

	"github.com/sqlrs/engine-local/internal/auth"
)

type cacheRoutes struct {
	opts Options
}

func (routes cacheRoutes) register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/cache/status", routes.handleStatus)
}

func (routes cacheRoutes) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if routes.opts.Prepare == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	status, err := routes.opts.Prepare.CacheStatus(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = writeJSON(w, status)
}
