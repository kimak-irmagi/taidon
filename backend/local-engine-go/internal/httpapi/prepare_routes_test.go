package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrepareRoutesRegisterExpectedHandlers(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	mux := http.NewServeMux()
	prepareRoutes{opts: opts}.register(mux)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "list jobs", method: http.MethodGet, path: "/v1/prepare-jobs", want: http.StatusOK},
		{name: "invalid create payload", method: http.MethodPost, path: "/v1/prepare-jobs", body: "{", want: http.StatusBadRequest},
		{name: "list tasks", method: http.MethodGet, path: "/v1/tasks", want: http.StatusOK},
		{name: "missing job status", method: http.MethodGet, path: "/v1/prepare-jobs/missing", want: http.StatusNotFound},
		{name: "missing job delete", method: http.MethodDelete, path: "/v1/prepare-jobs/missing", want: http.StatusNotFound},
		{name: "missing job cancel", method: http.MethodPost, path: "/v1/prepare-jobs/missing/cancel", want: http.StatusNotFound},
		{name: "missing job events", method: http.MethodGet, path: "/v1/prepare-jobs/missing/events", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer secret")
			resp := httptest.NewRecorder()

			mux.ServeHTTP(resp, req)

			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d", resp.Code, tt.want)
			}
		})
	}
}
