package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/client"
)

func TestRunStatusRemoteIncludesCacheSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"version":"v1","instanceId":"inst","pid":1}`))
		case "/v1/cache/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"usage_bytes":2048,"configured_max_bytes":4096,"effective_max_bytes":3072,"reserve_bytes":512,"high_watermark":0.9,"low_watermark":0.8,"min_state_age":"10m","store_total_bytes":8192,"store_free_bytes":6144,"state_count":3,"reclaimable_bytes":1024,"blocked_count":1,"pressure_reasons":["usage_above_high_watermark"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if result.CacheSummary == nil {
		t.Fatalf("expected cache summary, got %+v", result)
	}
	if result.CacheSummary.UsageBytes != 2048 || result.CacheSummary.EffectiveMaxBytes != 3072 {
		t.Fatalf("unexpected cache summary: %+v", result.CacheSummary)
	}
	if result.CacheSummary.StateCount != 3 {
		t.Fatalf("expected stateCount=3, got %+v", result.CacheSummary)
	}
	if len(result.CacheSummary.PressureReasons) != 1 || result.CacheSummary.PressureReasons[0] != "usage_above_high_watermark" {
		t.Fatalf("unexpected pressure reasons: %+v", result.CacheSummary)
	}
	if result.CacheDetails != nil {
		t.Fatalf("expected no cache details without --cache, got %+v", result.CacheDetails)
	}
}

func TestRunStatusRemoteCacheSummaryFailureFallsBackToWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"version":"v1"}`))
		case "/v1/cache/status":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if result.CacheSummary != nil {
		t.Fatalf("expected cache summary to be omitted, got %+v", result.CacheSummary)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected cache warning, got %+v", result)
	}
	if !strings.Contains(strings.ToLower(result.Warnings[0]), "cache") {
		t.Fatalf("expected cache warning, got %+v", result.Warnings)
	}
}

func TestRunStatusRemoteCacheDetailsFailureIsFatal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"version":"v1"}`))
		case "/v1/cache/status":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunStatus(context.Background(), StatusOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		CacheDetails: true,
	})
	if err == nil {
		t.Fatalf("expected cache details error")
	}
}

func TestPrintStatusIncludesCacheSummary(t *testing.T) {
	result := StatusResult{
		OK:       true,
		Endpoint: "http://localhost:1",
		Profile:  "local",
		Mode:     "remote",
		Version:  "v1",
		CacheSummary: &StatusCacheSummary{
			UsageBytes:        2048,
			EffectiveMaxBytes: 3072,
			StoreFreeBytes:    6144,
			StateCount:        3,
			PressureReasons:   []string{"usage_above_high_watermark"},
		},
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	for _, want := range []string{
		"cache.usageBytes: 2048",
		"cache.effectiveMaxBytes: 3072",
		"cache.storeFreeBytes: 6144",
		"cache.stateCount: 3",
		"cache.pressureReasons: usage_above_high_watermark",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestPrintStatusIncludesCacheDetails(t *testing.T) {
	result := StatusResult{
		OK:       true,
		Endpoint: "http://localhost:1",
		Profile:  "local",
		Mode:     "remote",
		Version:  "v1",
		CacheSummary: &StatusCacheSummary{
			UsageBytes:        2048,
			EffectiveMaxBytes: 3072,
			StoreFreeBytes:    6144,
			StateCount:        3,
		},
		CacheDetails: &StatusCacheDetails{
			ReserveBytes:     512,
			HighWatermark:    0.9,
			LowWatermark:     0.8,
			MinStateAge:      "10m",
			StoreTotalBytes:  8192,
			ReclaimableBytes: 1024,
			BlockedCount:     1,
			LastEviction: &StatusCacheEvictionSummary{
				CompletedAt:      "2026-03-09T12:00:00Z",
				Trigger:          "post_snapshot",
				EvictedCount:     2,
				FreedBytes:       256,
				BlockedCount:     1,
				ReclaimableBytes: 1024,
				UsageBytesBefore: 2304,
				UsageBytesAfter:  2048,
				FreeBytesBefore:  5888,
				FreeBytesAfter:   6144,
			},
		},
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	for _, want := range []string{
		"cache.reserveBytes: 512",
		"cache.highWatermark: 0.9",
		"cache.lowWatermark: 0.8",
		"cache.minStateAge: 10m",
		"cache.storeTotalBytes: 8192",
		"cache.reclaimableBytes: 1024",
		"cache.blockedCount: 1",
		"cache.lastEviction.completedAt: 2026-03-09T12:00:00Z",
		"cache.lastEviction.trigger: post_snapshot",
		"cache.lastEviction.evictedCount: 2",
		"cache.lastEviction.freedBytes: 256",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestDetailedCacheStatusIncludesLastEviction(t *testing.T) {
	details := detailedCacheStatus(client.CacheStatus{
		ReserveBytes:     512,
		HighWatermark:    0.9,
		LowWatermark:     0.8,
		MinStateAge:      "10m",
		StoreTotalBytes:  8192,
		ReclaimableBytes: 1024,
		BlockedCount:     1,
		LastEviction: &client.CacheEvictionSummary{
			CompletedAt:      "2026-03-09T12:00:00Z",
			Trigger:          "post_snapshot",
			EvictedCount:     2,
			FreedBytes:       256,
			BlockedCount:     1,
			ReclaimableBytes: 1024,
			UsageBytesBefore: 2304,
			UsageBytesAfter:  2048,
			FreeBytesBefore:  5888,
			FreeBytesAfter:   6144,
		},
	})
	if details == nil || details.LastEviction == nil {
		t.Fatalf("expected detailed cache status with last eviction, got %+v", details)
	}
	if details.LastEviction.Trigger != "post_snapshot" || details.LastEviction.FreeBytesAfter != 6144 {
		t.Fatalf("unexpected last eviction details: %+v", details.LastEviction)
	}
}

func TestFormatCacheSummaryWarningNil(t *testing.T) {
	if got := formatCacheSummaryWarning(nil); got != "cache summary unavailable" {
		t.Fatalf("unexpected warning: %q", got)
	}
}
