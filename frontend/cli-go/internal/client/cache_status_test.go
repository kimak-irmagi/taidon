package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetCacheStatus(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/cache/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage_bytes":2048,"configured_max_bytes":4096,"effective_max_bytes":3072,"reserve_bytes":512,"high_watermark":0.9,"low_watermark":0.8,"min_state_age":"10m","store_total_bytes":8192,"store_free_bytes":6144,"state_count":3,"reclaimable_bytes":1024,"blocked_count":1,"pressure_reasons":["usage_above_high_watermark"],"last_eviction":{"completed_at":"2026-03-09T12:00:00Z","trigger":"post_snapshot","evicted_count":2,"freed_bytes":256,"blocked_count":1,"reclaimable_bytes":1024,"usage_bytes_before":2304,"usage_bytes_after":2048,"free_bytes_before":5888,"free_bytes_after":6144}}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "secret"})
	status, err := cli.GetCacheStatus(context.Background())
	if err != nil {
		t.Fatalf("GetCacheStatus: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if status.UsageBytes != 2048 || status.EffectiveMaxBytes != 3072 {
		t.Fatalf("unexpected cache status: %+v", status)
	}
	if status.ConfiguredMaxBytes == nil || *status.ConfiguredMaxBytes != 4096 {
		t.Fatalf("expected configured max bytes, got %+v", status)
	}
	if len(status.PressureReasons) != 1 || status.PressureReasons[0] != "usage_above_high_watermark" {
		t.Fatalf("unexpected pressure reasons: %+v", status)
	}
	if status.LastEviction == nil || status.LastEviction.Trigger != "post_snapshot" {
		t.Fatalf("expected last eviction details, got %+v", status)
	}
}

func TestGetCacheStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	if _, err := cli.GetCacheStatus(context.Background()); err == nil {
		t.Fatalf("expected cache status error")
	}
}

func TestListStatesIncludesCacheFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"state_id":"state-1","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1","created_at":"2026-01-01T00:00:00Z","size_bytes":42,"last_used_at":"2026-03-09T12:00:00Z","use_count":7,"min_retention_until":"2026-03-09T12:10:00Z","refcount":1}]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	states, err := cli.ListStates(context.Background(), ListFilters{})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one state, got %+v", states)
	}
	if states[0].LastUsedAt == nil || *states[0].LastUsedAt != "2026-03-09T12:00:00Z" {
		t.Fatalf("expected last_used_at, got %+v", states[0])
	}
	if states[0].UseCount == nil || *states[0].UseCount != 7 {
		t.Fatalf("expected use_count, got %+v", states[0])
	}
	if states[0].MinRetentionUntil == nil || *states[0].MinRetentionUntil != "2026-03-09T12:10:00Z" {
		t.Fatalf("expected min_retention_until, got %+v", states[0])
	}
}
