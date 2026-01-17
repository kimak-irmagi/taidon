package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store"
)

type errorStore struct {
	listNamesErr     error
	getNameErr       error
	listInstancesErr error
	getInstanceErr   error
	listStatesErr    error
	getStateErr      error
}

func (e *errorStore) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	return nil, e.listNamesErr
}

func (e *errorStore) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	return store.NameEntry{}, false, e.getNameErr
}

func (e *errorStore) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	return nil, e.listInstancesErr
}

func (e *errorStore) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	return store.InstanceEntry{}, false, e.getInstanceErr
}

func (e *errorStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	return nil, e.listStatesErr
}

func (e *errorStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	return store.StateEntry{}, false, e.getStateErr
}

func (e *errorStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	return nil
}

func (e *errorStore) CreateInstance(ctx context.Context, entry store.InstanceCreate) error {
	return nil
}

func (e *errorStore) DeleteInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (e *errorStore) DeleteState(ctx context.Context, stateID string) error {
	return nil
}

func (e *errorStore) Close() error {
	return nil
}

func newErrorServer(t *testing.T, st store.Store) *httptest.Server {
	t.Helper()
	reg := registry.New(st)
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   reg,
	})
	return httptest.NewServer(handler)
}

func TestListNamesInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{listNamesErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/names", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetNameInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{getNameErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/names/dev", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestListInstancesInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{listInstancesErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetInstanceInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{getInstanceErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestListStatesInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{listStatesErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/states", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetStateInternalError(t *testing.T) {
	server := newErrorServer(t, &errorStore{getStateErr: errors.New("boom")})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/states/state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestPrepareJobsNilManager(t *testing.T) {
	server := newErrorServer(t, &errorStore{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestPrepareJobStatusNilManager(t *testing.T) {
	server := newErrorServer(t, &errorStore{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/job", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestDeleteInstanceNilDeletion(t *testing.T) {
	server := newErrorServer(t, &errorStore{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/instances/inst", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestDeleteStateNilDeletion(t *testing.T) {
	server := newErrorServer(t, &errorStore{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/states/state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
