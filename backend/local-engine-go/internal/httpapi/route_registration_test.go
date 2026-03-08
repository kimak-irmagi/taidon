package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandlerRegistersResourceRoutes(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	handler := NewHandler(opts)
	tests := []struct {
		name   string
		method string
		path   string
		auth   bool
		want   int
	}{
		{name: "health", method: http.MethodGet, path: "/v1/health", want: http.StatusOK},
		{name: "config schema", method: http.MethodGet, path: "/v1/config/schema", auth: true, want: http.StatusOK},
		{name: "prepare jobs", method: http.MethodGet, path: "/v1/prepare-jobs", auth: true, want: http.StatusOK},
		{name: "tasks", method: http.MethodGet, path: "/v1/tasks", auth: true, want: http.StatusOK},
		{name: "names", method: http.MethodGet, path: "/v1/names", auth: true, want: http.StatusOK},
		{name: "instances", method: http.MethodGet, path: "/v1/instances", auth: true, want: http.StatusOK},
		{name: "states", method: http.MethodGet, path: "/v1/states", auth: true, want: http.StatusOK},
		{name: "runs", method: http.MethodGet, path: "/v1/runs", auth: true, want: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.auth {
				req.Header.Set("Authorization", "Bearer secret")
			}
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d", resp.Code, tt.want)
			}
		})
	}
}
