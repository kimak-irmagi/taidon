package httpapi

import (
	"log"
	"net/http"
	"strings"

	"github.com/sqlrs/engine-local/internal/auth"
	"github.com/sqlrs/engine-local/internal/deletion"
	"github.com/sqlrs/engine-local/internal/store"
)

type registryRoutes struct {
	opts Options
}

func (routes registryRoutes) register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/names", routes.handleNames)
	mux.HandleFunc("/v1/names/", routes.handleName)
	mux.HandleFunc("/v1/instances", routes.handleInstances)
	mux.HandleFunc("/v1/instances/", routes.handleInstance)
	mux.HandleFunc("/v1/states", routes.handleStates)
	mux.HandleFunc("/v1/states/", routes.handleState)
}

func (routes registryRoutes) handleNames(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	filters := store.NameFilters{
		InstanceID: readQueryValue(r, "instance"),
		StateID:    readQueryValue(r, "state"),
		ImageID:    readQueryValue(r, "image"),
	}
	entries, err := routes.opts.Registry.ListNames(r.Context(), filters)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = writeListResponse(w, r, entries)
}

func (routes registryRoutes) handleName(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/v1/names/")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	entry, ok, err := routes.opts.Registry.GetName(r.Context(), name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = writeJSON(w, entry)
}

func (routes registryRoutes) handleInstances(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	idPrefix, err := normalizeIDPrefix(readQueryValue(r, "id_prefix"))
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid id_prefix", err.Error(), http.StatusBadRequest)
		return
	}
	filters := store.InstanceFilters{
		StateID:  readQueryValue(r, "state"),
		ImageID:  readQueryValue(r, "image"),
		IDPrefix: idPrefix,
	}
	entries, err := routes.opts.Registry.ListInstances(r.Context(), filters)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = writeListResponse(w, r, entries)
}

func (routes registryRoutes) handleInstance(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
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
		routes.deleteInstance(w, r, idOrName)
		return
	}
	entry, ok, resolvedByName, err := routes.opts.Registry.GetInstance(r.Context(), idOrName)
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
	_ = writeJSON(w, entry)
}

func (routes registryRoutes) handleStates(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
		return
	}
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	idPrefix, err := normalizeIDPrefix(readQueryValue(r, "id_prefix"))
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid id_prefix", err.Error(), http.StatusBadRequest)
		return
	}
	filters := store.StateFilters{
		Kind:     readQueryValue(r, "kind"),
		ImageID:  readQueryValue(r, "image"),
		IDPrefix: idPrefix,
	}
	entries, err := routes.opts.Registry.ListStates(r.Context(), filters)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = writeListResponse(w, r, entries)
}

func (routes registryRoutes) handleState(w http.ResponseWriter, r *http.Request) {
	if !auth.RequireBearer(w, r, routes.opts.AuthToken) {
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
		routes.deleteState(w, r, stateID)
		return
	}
	entry, ok, err := routes.opts.Registry.GetState(r.Context(), stateID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = writeJSON(w, entry)
}

func (routes registryRoutes) deleteInstance(w http.ResponseWriter, r *http.Request, idOrName string) {
	if routes.opts.Deletion == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	force, err := parseBoolQuery(r, "force")
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid force", err.Error(), http.StatusBadRequest)
		return
	}
	dryRun, err := parseBoolQuery(r, "dry_run")
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid dry_run", err.Error(), http.StatusBadRequest)
		return
	}
	result, found, err := routes.opts.Deletion.DeleteInstance(r.Context(), idOrName, deletion.DeleteOptions{Force: force, DryRun: dryRun})
	if err != nil {
		log.Printf("delete instance failed id=%s error=%v", idOrName, err)
		_ = writeErrorResponse(w, "internal_error", "delete instance failed", err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		_ = writeErrorResponse(w, "not_found", "instance not found", "", http.StatusNotFound)
		return
	}
	status := http.StatusOK
	if !dryRun && result.Outcome == deletion.OutcomeBlocked {
		status = http.StatusConflict
	}
	_ = writeJSONStatus(w, result, status)
}

func (routes registryRoutes) deleteState(w http.ResponseWriter, r *http.Request, stateID string) {
	if routes.opts.Deletion == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	recurse, err := parseBoolQuery(r, "recurse")
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid recurse", err.Error(), http.StatusBadRequest)
		return
	}
	force, err := parseBoolQuery(r, "force")
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid force", err.Error(), http.StatusBadRequest)
		return
	}
	dryRun, err := parseBoolQuery(r, "dry_run")
	if err != nil {
		_ = writeErrorResponse(w, "invalid_argument", "invalid dry_run", err.Error(), http.StatusBadRequest)
		return
	}
	result, found, err := routes.opts.Deletion.DeleteState(r.Context(), stateID, deletion.DeleteOptions{Recurse: recurse, Force: force, DryRun: dryRun})
	if err != nil {
		log.Printf("delete state failed id=%s error=%v", stateID, err)
		_ = writeErrorResponse(w, "internal_error", "delete state failed", err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		_ = writeErrorResponse(w, "not_found", "state not found", "", http.StatusNotFound)
		return
	}
	status := http.StatusOK
	if !dryRun && result.Outcome == deletion.OutcomeBlocked {
		status = http.StatusConflict
	}
	_ = writeJSONStatus(w, result, status)
}
