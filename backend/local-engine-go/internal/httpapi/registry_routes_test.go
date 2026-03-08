package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sqlrs/engine-local/internal/deletion"
)

func TestRegistryRoutesRegisterExpectedHandlers(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	mux := http.NewServeMux()
	registryRoutes{opts: opts}.register(mux)

	t.Run("list names", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/names", nil)
		req.Header.Set("Authorization", "Bearer secret")
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
	})

	t.Run("instance alias redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/instances/dev", nil)
		req.Header.Set("Authorization", "Bearer secret")
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, req)

		if resp.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusTemporaryRedirect)
		}
		if got := resp.Header().Get("Location"); got != "/v1/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
			t.Fatalf("Location = %q, want instance redirect", got)
		}
	})

	t.Run("state detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/states/state-1", nil)
		req.Header.Set("Authorization", "Bearer secret")
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
	})

	t.Run("instance delete dry run", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa?dry_run=true", nil)
		req.Header.Set("Authorization", "Bearer secret")
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
		var result deletion.DeleteResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode delete result: %v", err)
		}
		if !result.DryRun {
			t.Fatalf("expected dry-run delete result, got %+v", result)
		}
	})

	t.Run("reject invalid instance prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/instances?id_prefix=xyz", nil)
		req.Header.Set("Authorization", "Bearer secret")
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})
}
