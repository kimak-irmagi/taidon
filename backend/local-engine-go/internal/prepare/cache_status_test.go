package prepare

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/engine-local/internal/store"
)

func TestCacheStatusReturnsDiagnostics(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-1",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				SizeBytes:   int64Ptr(1024),
				PrepareKind: "psql",
			},
			{
				StateID:     "state-2",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				SizeBytes:   int64Ptr(256),
				PrepareKind: "psql",
				RefCount:    1,
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
		config: &fakeConfigStore{values: map[string]any{
			"cache.capacity.maxBytes":      int64(3000),
			"cache.capacity.reserveBytes":  int64(0),
			"cache.capacity.highWatermark": 0.60,
			"cache.capacity.lowWatermark":  0.50,
			"cache.capacity.minStateAge":   "10m",
		}},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 5000, 1500, nil },
		func(string) (int64, error) { return 1900, nil },
	)
	mgr.recordCacheEviction(CacheEvictionSummary{
		CompletedAt:      "2026-03-09T12:00:00Z",
		Trigger:          "metadata_commit",
		EvictedCount:     1,
		FreedBytes:       1024,
		BlockedCount:     1,
		ReclaimableBytes: 1024,
		UsageBytesBefore: 2924,
		UsageBytesAfter:  1900,
		FreeBytesBefore:  476,
		FreeBytesAfter:   1500,
	})

	status, err := mgr.CacheStatus(context.Background())
	if err != nil {
		t.Fatalf("CacheStatus: %v", err)
	}
	if status.UsageBytes != 1900 || status.EffectiveMaxBytes != 3000 {
		t.Fatalf("unexpected status limits: %+v", status)
	}
	if status.ConfiguredMaxBytes == nil || *status.ConfiguredMaxBytes != 3000 {
		t.Fatalf("expected configured max bytes, got %+v", status.ConfiguredMaxBytes)
	}
	if status.StateCount != 2 || status.BlockedCount != 1 || status.ReclaimableBytes != 1024 {
		t.Fatalf("unexpected state summary: %+v", status)
	}
	if len(status.PressureReasons) != 1 || status.PressureReasons[0] != "usage_above_high_watermark" {
		t.Fatalf("unexpected pressure reasons: %+v", status.PressureReasons)
	}
	if status.LastEviction == nil || status.LastEviction.Trigger != "metadata_commit" || status.LastEviction.EvictedCount != 1 {
		t.Fatalf("unexpected last eviction: %+v", status.LastEviction)
	}
}

func TestCacheStatusStoreCoupledModeUsesEmptyPressureSlice(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
		config: &fakeConfigStore{values: map[string]any{
			"cache.capacity.maxBytes":      int64(0),
			"cache.capacity.reserveBytes":  int64(0),
			"cache.capacity.highWatermark": 0.90,
			"cache.capacity.lowWatermark":  0.80,
			"cache.capacity.minStateAge":   "0s",
		}},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 900, nil },
		func(string) (int64, error) { return 10, nil },
	)

	status, err := mgr.CacheStatus(context.Background())
	if err != nil {
		t.Fatalf("CacheStatus: %v", err)
	}
	if status.ConfiguredMaxBytes != nil {
		t.Fatalf("expected nil configured max bytes, got %+v", status.ConfiguredMaxBytes)
	}
	if status.LastEviction != nil {
		t.Fatalf("expected no last eviction, got %+v", status.LastEviction)
	}
	if status.PressureReasons == nil || len(status.PressureReasons) != 0 {
		t.Fatalf("expected empty pressure reasons slice, got %+v", status.PressureReasons)
	}
}

func TestCacheStatusSkipsStatesWithoutSizeInReclaimableAccounting(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-no-size",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				PrepareKind: "psql",
				SizeBytes:   nil,
			},
			{
				StateID:     "state-blocked",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				PrepareKind: "psql",
				SizeBytes:   int64Ptr(256),
				RefCount:    1,
			},
			{
				StateID:     "state-reclaimable",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				PrepareKind: "psql",
				SizeBytes:   int64Ptr(1024),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
		config: &fakeConfigStore{values: map[string]any{
			"cache.capacity.maxBytes":      int64(4096),
			"cache.capacity.reserveBytes":  int64(0),
			"cache.capacity.highWatermark": 0.90,
			"cache.capacity.lowWatermark":  0.80,
			"cache.capacity.minStateAge":   "0s",
		}},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(path string) (int64, error) {
			if strings.Contains(path, "state-no-size") {
				return 0, nil
			}
			return 1280, nil
		},
	)

	status, err := mgr.CacheStatus(context.Background())
	if err != nil {
		t.Fatalf("CacheStatus: %v", err)
	}
	if status.StateCount != 3 {
		t.Fatalf("expected 3 states, got %d", status.StateCount)
	}
	if status.BlockedCount != 1 {
		t.Fatalf("expected blocked_count=1, got %d", status.BlockedCount)
	}
	if status.ReclaimableBytes != 1024 {
		t.Fatalf("expected reclaimable_bytes=1024, got %d", status.ReclaimableBytes)
	}
}

func TestCacheStatusReturnsFilesystemStatsError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 0, 0, errors.New("stats boom") },
		func(string) (int64, error) { return 0, nil },
	)

	_, err := mgr.CacheStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stats boom") {
		t.Fatalf("expected filesystem stats error, got %v", err)
	}
}

func TestCacheStatusReturnsCacheUsageError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(string) (int64, error) { return 0, errors.New("usage boom") },
	)

	_, err := mgr.CacheStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "usage boom") {
		t.Fatalf("expected cache usage error, got %v", err)
	}
}

func TestCacheStatusReturnsLoadSettingsError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
		config: &fakeConfigStore{values: map[string]any{
			"cache.capacity.maxBytes":      int64(4096),
			"cache.capacity.reserveBytes":  int64(0),
			"cache.capacity.highWatermark": "bad",
			"cache.capacity.lowWatermark":  0.80,
			"cache.capacity.minStateAge":   "0s",
		}},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(string) (int64, error) { return 1024, nil },
	)

	_, err := mgr.CacheStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cache.capacity.highWatermark") {
		t.Fatalf("expected load settings error, got %v", err)
	}
}

func TestCacheStatusReturnsListStatesError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{listStatesErr: errors.New("list boom")}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(string) (int64, error) { return 1024, nil },
	)

	_, err := mgr.CacheStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list boom") {
		t.Fatalf("expected list states error, got %v", err)
	}
}

func TestCacheStatusIgnoresEvictionCandidateMeasurementError(t *testing.T) {
	oldStoreUsage := storeUsageFn
	storeUsageFn = func(path string) (int64, error) {
		return 0, errors.New("measure boom")
	}
	t.Cleanup(func() { storeUsageFn = oldStoreUsage })

	mgr := newManagerWithDeps(t, &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-no-size",
				ImageID:     "image-1",
				CreatedAt:   "2025-12-01T00:00:00Z",
				PrepareKind: "psql",
				SizeBytes:   nil,
			},
		},
	}, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(string) (int64, error) { return 1024, nil },
	)

	status, err := mgr.CacheStatus(context.Background())
	if err != nil {
		t.Fatalf("expected measurement error to be ignored, got %v", err)
	}
	if status.StateCount != 1 {
		t.Fatalf("expected state_count=1, got %d", status.StateCount)
	}
}

func TestCacheStatusSetupErrors(t *testing.T) {
	var nilMgr *PrepareService
	if _, err := nilMgr.CacheStatus(context.Background()); err == nil || !strings.Contains(err.Error(), "prepare service is not configured") {
		t.Fatalf("expected nil service error, got %v", err)
	}

	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), nil)
	mgr.stateStoreRoot = " "
	if _, err := mgr.CacheStatus(context.Background()); err == nil || !strings.Contains(err.Error(), "state store root is not configured") {
		t.Fatalf("expected blank root error, got %v", err)
	}
}

func TestCacheStatusReturnsMkdirAllError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		stateRoot: filepath.Join(t.TempDir(), "bad\x00root"),
	})

	_, err := mgr.CacheStatus(context.Background())
	if err == nil {
		t.Fatalf("expected mkdir error for invalid state root")
	}
}

func TestCacheStatusReturnsParentLookupErrorFromEvictionCandidates(t *testing.T) {
	faultyStore := &parentLookupErrStore{
		fakeStore: fakeStore{
			listStates: []store.StateEntry{
				{
					StateID:     "state-1",
					ImageID:     "image-1",
					CreatedAt:   "2025-12-01T00:00:00Z",
					PrepareKind: "psql",
					SizeBytes:   int64Ptr(128),
				},
			},
		},
		err: errors.New("children lookup boom"),
	}
	mgr := newManagerWithDeps(t, faultyStore, newQueueStore(t), &testDeps{
		stateRoot: t.TempDir() + "/nested/store-root",
		config: &fakeConfigStore{values: map[string]any{
			"cache.capacity.maxBytes":      int64(4096),
			"cache.capacity.reserveBytes":  int64(0),
			"cache.capacity.highWatermark": 0.90,
			"cache.capacity.lowWatermark":  0.80,
			"cache.capacity.minStateAge":   "0s",
		}},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 8192, 4096, nil },
		func(string) (int64, error) { return 128, nil },
	)

	_, err := mgr.CacheStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "children lookup boom") {
		t.Fatalf("expected parent lookup error, got %v", err)
	}
}

func TestLastCacheEvictionReturnsCopy(t *testing.T) {
	var nilMgr *PrepareService
	nilMgr.recordCacheEviction(CacheEvictionSummary{Trigger: "ignored"})
	if got := nilMgr.lastCacheEviction(); got != nil {
		t.Fatalf("expected nil receiver to return nil, got %+v", got)
	}

	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), nil)
	mgr.recordCacheEviction(CacheEvictionSummary{Trigger: "metadata_commit", EvictedCount: 1})
	got := mgr.lastCacheEviction()
	if got == nil {
		t.Fatalf("expected eviction summary")
	}
	got.Trigger = "mutated"
	if mgr.lastEviction == nil || mgr.lastEviction.Trigger != "metadata_commit" {
		t.Fatalf("expected stored summary to remain unchanged, got %+v", mgr.lastEviction)
	}
}
