package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEngineAuth(t *testing.T) {
	reg := newRegistry()
	server := httptest.NewServer(buildMux("test", "instance", "secret", reg))
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for health, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/names", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("names request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for names without auth, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, server.URL+"/v1/names", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("names request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for names with bad auth, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, server.URL+"/v1/names", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("names request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for names with auth, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestNamesNDJSON(t *testing.T) {
	reg := newRegistry()
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	setName(reg, NameEntry{
		Name:       "dev",
		InstanceID: strPtr(instanceID),
		ImageID:    "image-1",
		StateID:    "state-1",
		Status:     "active",
	})
	setName(reg, NameEntry{
		Name:       "qa",
		InstanceID: strPtr(instanceID),
		ImageID:    "image-1",
		StateID:    "state-2",
		Status:     "active",
	})

	server := httptest.NewServer(buildMux("test", "instance", "secret", reg))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/names", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Accept", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("names request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("expected ndjson content type, got %q", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines, got %d", len(lines))
	}
	for _, line := range lines {
		var entry NameEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid ndjson line: %v", err)
		}
	}
}

func TestInstancesAliasRedirect(t *testing.T) {
	reg := newRegistry()
	instanceID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	setInstance(reg, InstanceEntry{
		InstanceID: instanceID,
		ImageID:    "image-1",
		StateID:    "state-1",
		CreatedAt:  "2025-01-01T00:00:00Z",
		Status:     "active",
	})
	setName(reg, NameEntry{
		Name:       "dev",
		InstanceID: strPtr(instanceID),
		ImageID:    "image-1",
		StateID:    "state-1",
		Status:     "active",
	})

	server := httptest.NewServer(buildMux("test", "instance", "secret", reg))
	defer server.Close()

	redirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/instances/dev", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := redirectClient.Do(req)
	if err != nil {
		t.Fatalf("instances request: %v", err)
	}
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/v1/instances/"+instanceID {
		t.Fatalf("unexpected location: %q", loc)
	}
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, server.URL+"/v1/instances/"+instanceID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("instances request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entry InstanceEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("decode instance: %v", err)
	}
	if entry.InstanceID != instanceID {
		t.Fatalf("expected instance id %q, got %q", instanceID, entry.InstanceID)
	}
}

func TestNamesFilterByInstance(t *testing.T) {
	reg := newRegistry()
	id1 := "cccccccccccccccccccccccccccccccc"
	id2 := "dddddddddddddddddddddddddddddddd"
	setName(reg, NameEntry{
		Name:       "dev",
		InstanceID: strPtr(id1),
		ImageID:    "image-1",
		StateID:    "state-1",
		Status:     "active",
	})
	setName(reg, NameEntry{
		Name:       "qa",
		InstanceID: strPtr(id2),
		ImageID:    "image-1",
		StateID:    "state-1",
		Status:     "active",
	})

	server := httptest.NewServer(buildMux("test", "instance", "secret", reg))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/names?instance="+id1, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("names request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entries []NameEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode names: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "dev" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func setName(reg *registry, entry NameEntry) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.names[entry.Name] = entry
}

func setInstance(reg *registry, entry InstanceEntry) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.instances[entry.InstanceID] = entry
}

func strPtr(value string) *string {
	return &value
}
