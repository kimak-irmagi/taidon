package prepare

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"sqlrs/engine/internal/store"
)

func TestLoadCapacitySettingsUsesStoreCoupledEffectiveMax(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(900),
				"cache.capacity.reserveBytes":  int64(200),
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "15m",
			},
		},
	})

	settings, err := mgr.loadCapacitySettings(1000)
	if err != nil {
		t.Fatalf("loadCapacitySettings: %v", err)
	}
	if !settings.Enabled {
		t.Fatalf("expected enabled settings")
	}
	if settings.ReserveBytes != 200 {
		t.Fatalf("expected reserve=200, got %d", settings.ReserveBytes)
	}
	if settings.EffectiveMax != 800 {
		t.Fatalf("expected effective max=800, got %d", settings.EffectiveMax)
	}
}

func TestEnsureCacheCapacityReturnsTooSmallWhenEffectiveMaxZero(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(0),
				"cache.capacity.reserveBytes":  int64(2000),
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "10m",
			},
		},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 700, nil },
		func(string) (int64, error) { return 0, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil {
		t.Fatalf("expected capacity error")
	}
	if errResp.Code != "cache_limit_too_small" {
		t.Fatalf("expected cache_limit_too_small, got %+v", errResp)
	}
	details := decodeCapacityDetails(t, errResp)
	if details["phase"] != "prepare_step" {
		t.Fatalf("expected phase=prepare_step, got %#v", details["phase"])
	}
	if got, ok := details["effective_max_bytes"].(float64); !ok || int64(got) != 0 {
		t.Fatalf("expected effective_max_bytes=0, got %#v", details["effective_max_bytes"])
	}
}

func overrideCapacitySignals(t *testing.T, fsFn func(path string) (int64, int64, error), usageFn func(path string) (int64, error)) {
	t.Helper()
	oldFS := filesystemStatsFn
	oldUsage := storeUsageFn
	if fsFn != nil {
		filesystemStatsFn = fsFn
	}
	if usageFn != nil {
		storeUsageFn = usageFn
	}
	t.Cleanup(func() {
		filesystemStatsFn = oldFS
		storeUsageFn = oldUsage
	})
}

func decodeCapacityDetails(t *testing.T, errResp *ErrorResponse) map[string]any {
	t.Helper()
	if errResp == nil {
		return map[string]any{}
	}
	if errResp.Details == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(errResp.Details), &out); err != nil {
		t.Fatalf("details is not json: %v", err)
	}
	return out
}

func TestListEvictionCandidatesSortsByLastUsedThenSize(t *testing.T) {
	now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
	oldNow := nowUTCFn
	nowUTCFn = func() time.Time { return now }
	t.Cleanup(func() { nowUTCFn = oldNow })

	lastUsedRecent := now.Add(-10 * time.Minute).Format(time.RFC3339Nano)
	lastUsedOld := now.Add(-2 * time.Hour).Format(time.RFC3339Nano)
	created := now.Add(-3 * time.Hour).Format(time.RFC3339Nano)
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-newer-use",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				LastUsedAt:  strPtr(lastUsedRecent),
				SizeBytes:   int64Ptr(500),
			},
			{
				StateID:     "state-older-use-small",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				LastUsedAt:  strPtr(lastUsedOld),
				SizeBytes:   int64Ptr(100),
			},
			{
				StateID:     "state-older-use-large",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				LastUsedAt:  strPtr(lastUsedOld),
				SizeBytes:   int64Ptr(900),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
	candidates, blocked, _, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{
		MinStateAge: 0,
	})
	if err != nil {
		t.Fatalf("listEvictionCandidates: %v", err)
	}
	if blocked != 0 {
		t.Fatalf("expected blocked=0, got %d", blocked)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}
	if candidates[0].StateID != "state-older-use-large" {
		t.Fatalf("expected oldest and larger first, got %q", candidates[0].StateID)
	}
	if candidates[1].StateID != "state-older-use-small" {
		t.Fatalf("expected second oldest same last_used smaller, got %q", candidates[1].StateID)
	}
	if candidates[2].StateID != "state-newer-use" {
		t.Fatalf("expected newest last_used last, got %q", candidates[2].StateID)
	}
}

func TestListEvictionCandidatesBlocksMinRetentionUntil(t *testing.T) {
	now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
	oldNow := nowUTCFn
	nowUTCFn = func() time.Time { return now }
	t.Cleanup(func() { nowUTCFn = oldNow })

	created := now.Add(-2 * time.Hour).Format(time.RFC3339Nano)
	retained := now.Add(30 * time.Minute).Format(time.RFC3339Nano)
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:           "state-retained",
				ImageID:           "image-1",
				PrepareKind:       "psql",
				CreatedAt:         created,
				MinRetentionUntil: strPtr(retained),
				SizeBytes:         int64Ptr(200),
			},
			{
				StateID:     "state-evictable",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				SizeBytes:   int64Ptr(300),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
	candidates, blocked, reclaimable, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{
		MinStateAge: 0,
	})
	if err != nil {
		t.Fatalf("listEvictionCandidates: %v", err)
	}
	if blocked != 1 {
		t.Fatalf("expected blocked=1, got %d", blocked)
	}
	if len(candidates) != 1 || candidates[0].StateID != "state-evictable" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	if reclaimable != 300 {
		t.Fatalf("expected reclaimable=300, got %d", reclaimable)
	}
}

func TestListEvictionCandidatesSkipsProtectedStateIDs(t *testing.T) {
	created := "2026-02-22T10:00:00Z"
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-protected",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				SizeBytes:   int64Ptr(500),
			},
			{
				StateID:     "state-evictable",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   created,
				SizeBytes:   int64Ptr(300),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
	candidates, blocked, reclaimable, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{
		MinStateAge: 0,
	}, "state-protected")
	if err != nil {
		t.Fatalf("listEvictionCandidates: %v", err)
	}
	if blocked != 1 {
		t.Fatalf("expected blocked=1, got %d", blocked)
	}
	if len(candidates) != 1 || candidates[0].StateID != "state-evictable" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	if reclaimable != 300 {
		t.Fatalf("expected reclaimable=300, got %d", reclaimable)
	}
}

func TestEnsureCacheCapacityReturnsUnreclaimableUnderPressure(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-blocked",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				RefCount:  1,
				SizeBytes: int64Ptr(100),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(1000),
				"cache.capacity.reserveBytes":  int64(200),
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "0s",
			},
		},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 100, nil },
		func(string) (int64, error) { return 950, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil {
		t.Fatalf("expected capacity error")
	}
	if errResp.Code != "cache_full_unreclaimable" {
		t.Fatalf("expected cache_full_unreclaimable, got %+v", errResp)
	}
	details := decodeCapacityDetails(t, errResp)
	reasons, ok := details["reasons"].([]any)
	if !ok || len(reasons) == 0 {
		t.Fatalf("expected reasons, got %#v", details["reasons"])
	}
	hasReserveReason := false
	for _, reason := range reasons {
		if reason == "physical_free_below_reserve" {
			hasReserveReason = true
			break
		}
	}
	if !hasReserveReason {
		t.Fatalf("expected physical_free_below_reserve reason, got %#v", details["reasons"])
	}
}

func TestEnsureCacheCapacityReturnsTooSmallWhenNothingReclaimable(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(1000),
				"cache.capacity.reserveBytes":  int64(200),
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "0s",
			},
		},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 100, nil },
		func(string) (int64, error) { return 950, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil {
		t.Fatalf("expected capacity error")
	}
	if errResp.Code != "cache_limit_too_small" {
		t.Fatalf("expected cache_limit_too_small, got %+v", errResp)
	}
	details := decodeCapacityDetails(t, errResp)
	effectiveMax, ok := details["effective_max_bytes"].(float64)
	if !ok {
		t.Fatalf("expected effective_max_bytes in details, got %#v", details)
	}
	if got, ok := details["recommended_min_bytes"].(float64); !ok || got <= effectiveMax {
		t.Fatalf("expected recommended_min_bytes > effective max, got %#v (effective=%v)", details["recommended_min_bytes"], effectiveMax)
	}
}

func TestEnsureCacheCapacityReturnsTooSmallWhenObservedMinExceedsEffectiveMax(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-blocked",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				RefCount:  1,
				SizeBytes: int64Ptr(300),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(200),
				"cache.capacity.reserveBytes":  int64(0),
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "0s",
			},
		},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 50, nil },
		func(string) (int64, error) { return 950, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil {
		t.Fatalf("expected capacity error")
	}
	if errResp.Code != "cache_limit_too_small" {
		t.Fatalf("expected cache_limit_too_small, got %+v", errResp)
	}
	details := decodeCapacityDetails(t, errResp)
	if got := int64(details["effective_max_bytes"].(float64)); got != 200 {
		t.Fatalf("expected effective_max_bytes=200, got %d", got)
	}
	if got := int64(details["observed_required_bytes"].(float64)); got != 300 {
		t.Fatalf("expected observed_required_bytes=300, got %d", got)
	}
}

func TestRunEvictionEvictsCandidates(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:     "state-1",
				ImageID:     "image-1",
				PrepareKind: "psql",
				CreatedAt:   "2026-02-22T10:00:00Z",
				SizeBytes:   int64Ptr(250),
			},
		},
	}
	fs := &fakeStateFS{}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{statefs: fs})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 450, nil },
		func(path string) (int64, error) {
			if strings.Contains(path, "state-1") {
				return 250, nil
			}
			return 700, nil
		},
	)

	summary, err := mgr.runEviction(context.Background(), capacitySettings{
		EffectiveMax:  1000,
		ReserveBytes:  300,
		LowWatermark:  0.80,
		MinStateAge:   0,
		HighWatermark: 0.90,
	}, 900, 100)
	if err != nil {
		t.Fatalf("runEviction: %v", err)
	}
	if summary.EvictedCount != 1 || summary.FreedBytes != 250 {
		t.Fatalf("unexpected eviction summary: %+v", summary)
	}
	if len(st.deletedStates) != 1 || st.deletedStates[0] != "state-1" {
		t.Fatalf("expected deleted state-1, got %+v", st.deletedStates)
	}
	if len(fs.removeCalls) != 1 {
		t.Fatalf("expected one remove call, got %+v", fs.removeCalls)
	}
}

func TestDeleteEvictionCandidateReturnsDeleteError(t *testing.T) {
	st := &deleteStateErrStore{
		fakeStore: fakeStore{},
		deleteErr: errors.New("boom"),
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{})
	err := mgr.deleteEvictionCandidate(context.Background(), evictCandidate{
		StateID: "state-1",
		ImageID: "image-1",
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestWithEvictLockBranches(t *testing.T) {
	if err := withEvictLock(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatalf("expected callback validation error")
	}

	root := t.TempDir()
	called := false
	if err := withEvictLock(context.Background(), root, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withEvictLock: %v", err)
	}
	if !called {
		t.Fatalf("expected callback call")
	}
	lockPath := filepath.Join(root, evictLockFileName)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file cleanup, err=%v", err)
	}

	if err := os.WriteFile(lockPath, []byte("busy"), 0o600); err != nil {
		t.Fatalf("create lock: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := withEvictLock(ctx, root, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestCapacityHelperFunctions(t *testing.T) {
	settings := capacitySettings{EffectiveMax: 1000, HighWatermark: 0.90, ReserveBytes: 200}
	if !isCapacityPressure(901, 500, settings) {
		t.Fatalf("expected usage pressure")
	}
	if !isCapacityPressure(100, 100, settings) {
		t.Fatalf("expected reserve pressure")
	}
	reasons := pressureReasons(950, 100, settings)
	if len(reasons) != 2 {
		t.Fatalf("expected two reasons, got %+v", reasons)
	}
	if maxInt64(10, 20) != 20 {
		t.Fatalf("maxInt64 mismatch")
	}
}

func TestCapacityParsersAndNoSpaceHelpers(t *testing.T) {
	if _, ok := parseRFC3339Any(""); ok {
		t.Fatalf("expected empty timestamp to fail")
	}
	if _, ok := parseRFC3339Any("2026-02-22T12:00:00Z"); !ok {
		t.Fatalf("expected rfc3339 timestamp to parse")
	}

	if !isNoSpaceError(errors.New("No space left on device")) {
		t.Fatalf("expected no-space detection")
	}
	if isNoSpaceError(errors.New("other error")) {
		t.Fatalf("unexpected no-space detection")
	}

	if noSpaceErrorResponse("msg", "phase", errors.New("other")) != nil {
		t.Fatalf("expected nil for non-no-space error")
	}
	if noSpaceErrorResponse("msg", "phase", errors.New("not enough space on the disk")) == nil {
		t.Fatalf("expected capacity response")
	}
}

func TestCapacityNumberParsingAndUsageMeasurement(t *testing.T) {
	if _, ok := asInt64(json.Number("1.2")); ok {
		t.Fatalf("expected fractional json number to fail")
	}
	if value, ok := asInt64(json.Number("42")); !ok || value != 42 {
		t.Fatalf("expected int64 parse, got %d ok=%v", value, ok)
	}
	if _, ok := asFloat64("bad"); ok {
		t.Fatalf("expected invalid float value")
	}
	if value, ok := asFloat64(json.Number("0.75")); !ok || value != 0.75 {
		t.Fatalf("expected float parse, got %f ok=%v", value, ok)
	}

	if size, err := measureStoreUsage(""); err != nil || size != 0 {
		t.Fatalf("expected empty path usage=0, got size=%d err=%v", size, err)
	}
	filePath := filepath.Join(t.TempDir(), "state", "size.dat")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("12345"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if size, err := measureStoreUsage(filepath.Dir(filePath)); err != nil || size < 5 {
		t.Fatalf("expected measured usage >= 5, got size=%d err=%v", size, err)
	}
}

func TestObservedMinStateBytes(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{
		listStates: []store.StateEntry{
			{StateID: "a", SizeBytes: int64Ptr(200)},
			{StateID: "b", SizeBytes: int64Ptr(120)},
		},
	}, newQueueStore(t), nil)
	if got := mgr.observedMinStateBytes(context.Background()); got != 120 {
		t.Fatalf("expected observed min 120, got %d", got)
	}

	mgr = newManagerWithDeps(t, &fakeStore{listStatesErr: errors.New("boom")}, newQueueStore(t), nil)
	if got := mgr.observedMinStateBytes(context.Background()); got != 0 {
		t.Fatalf("expected observed min 0 on error, got %d", got)
	}
}

func TestEnsureCacheCapacityReturnsUnavailableOnStatsError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 0, 0, errors.New("boom") },
		func(string) (int64, error) { return 0, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
		t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
	}
}

func TestEnsureCacheCapacityReturnsUnavailableOnLoadSettingsError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      int64(1000),
				"cache.capacity.reserveBytes":  int64(0),
				"cache.capacity.highWatermark": "bad",
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "10m",
			},
		},
	})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 900, nil },
		func(string) (int64, error) { return 100, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
		t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
	}
}

func TestEnsureCacheCapacityReturnsUnavailableOnStoreUsageErrors(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-1",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				SizeBytes: int64Ptr(100),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{})

	t.Run("pre-eviction-usage", func(t *testing.T) {
		overrideCapacitySignals(t,
			func(string) (int64, int64, error) { return 1000, 100, nil },
			func(string) (int64, error) { return 0, errors.New("usage pre") },
		)
		errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
		if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
			t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
		}
	})

	t.Run("post-eviction-usage", func(t *testing.T) {
		usageCalls := 0
		overrideCapacitySignals(t,
			func(string) (int64, int64, error) { return 1000, 100, nil },
			func(string) (int64, error) {
				usageCalls++
				switch usageCalls {
				case 1:
					return 950, nil
				case 2:
					return 700, nil
				default:
					return 0, errors.New("usage after")
				}
			},
		)
		errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
		if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
			t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
		}
	})
}

func TestEnsureCacheCapacityReturnsUnavailableOnFreeAfterEvictionError(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-1",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				SizeBytes: int64Ptr(100),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{})
	freeCalls := 0
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) {
			freeCalls++
			switch freeCalls {
			case 1:
				return 1000, 100, nil
			case 2:
				return 1000, 400, nil
			default:
				return 0, 0, errors.New("free after")
			}
		},
		func(path string) (int64, error) {
			if strings.Contains(path, "state-1") {
				return 100, nil
			}
			return 950, nil
		},
	)
	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
		t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
	}
}

func TestEnsureCacheCapacityReturnsUnavailableOnLockError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "state-store-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-1",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				SizeBytes: int64Ptr(100),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{stateRoot: blocker})
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) { return 1000, 100, nil },
		func(string) (int64, error) { return 950, nil },
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp == nil || errResp.Code != "cache_enforcement_unavailable" {
		t.Fatalf("expected cache_enforcement_unavailable, got %+v", errResp)
	}
}

func TestEnsureCacheCapacityReturnsNilAfterSuccessfulEviction(t *testing.T) {
	st := &fakeStore{
		listStates: []store.StateEntry{
			{
				StateID:   "state-1",
				ImageID:   "image-1",
				CreatedAt: "2026-02-22T10:00:00Z",
				SizeBytes: int64Ptr(100),
			},
		},
	}
	mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{})
	usageCalls := 0
	freeCalls := 0
	overrideCapacitySignals(t,
		func(string) (int64, int64, error) {
			freeCalls++
			switch freeCalls {
			case 1:
				return 1000, 100, nil
			default:
				return 1000, 400, nil
			}
		},
		func(path string) (int64, error) {
			if strings.Contains(path, "state-1") {
				return 100, nil
			}
			usageCalls++
			if usageCalls == 1 {
				return 950, nil
			}
			return 700, nil
		},
	)

	errResp := mgr.ensureCacheCapacity(context.Background(), "job-1", "prepare_step")
	if errResp != nil {
		t.Fatalf("expected nil after successful eviction, got %+v", errResp)
	}
}

func TestLoadCapacitySettingsValidationBranches(t *testing.T) {
	base := map[string]any{
		"cache.capacity.maxBytes":      int64(1000),
		"cache.capacity.reserveBytes":  int64(100),
		"cache.capacity.highWatermark": 0.90,
		"cache.capacity.lowWatermark":  0.80,
		"cache.capacity.minStateAge":   "10m",
	}
	cases := []struct {
		name string
		set  map[string]any
	}{
		{name: "max invalid", set: map[string]any{"cache.capacity.maxBytes": "bad"}},
		{name: "reserve invalid", set: map[string]any{"cache.capacity.reserveBytes": "bad"}},
		{name: "high invalid", set: map[string]any{"cache.capacity.highWatermark": "bad"}},
		{name: "low invalid", set: map[string]any{"cache.capacity.lowWatermark": "bad"}},
		{name: "min age type", set: map[string]any{"cache.capacity.minStateAge": 10}},
		{name: "min age parse", set: map[string]any{"cache.capacity.minStateAge": "bad"}},
		{name: "max negative", set: map[string]any{"cache.capacity.maxBytes": int64(-1)}},
		{name: "reserve negative", set: map[string]any{"cache.capacity.reserveBytes": int64(-1)}},
		{name: "high out of range", set: map[string]any{"cache.capacity.highWatermark": 1.2}},
		{name: "low >= high", set: map[string]any{"cache.capacity.lowWatermark": 0.95}},
		{name: "min age negative", set: map[string]any{"cache.capacity.minStateAge": "-1s"}},
	}
	for _, tc := range cases {
		values := map[string]any{}
		for key, value := range base {
			values[key] = value
		}
		for key, value := range tc.set {
			values[key] = value
		}
		mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
			config: &fakeConfigStore{values: values},
		})
		if _, err := mgr.loadCapacitySettings(1000); err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
}

func TestLoadCapacitySettingsNegativeStoreTotalAndDefaults(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{
			values: map[string]any{
				"cache.capacity.maxBytes":      nil,
				"cache.capacity.reserveBytes":  nil,
				"cache.capacity.highWatermark": 0.90,
				"cache.capacity.lowWatermark":  0.80,
				"cache.capacity.minStateAge":   "10m",
			},
		},
	})
	settings, err := mgr.loadCapacitySettings(-1)
	if err != nil {
		t.Fatalf("loadCapacitySettings: %v", err)
	}
	if settings.EffectiveMax != 0 {
		t.Fatalf("expected effective max 0 for negative total, got %d", settings.EffectiveMax)
	}
}

func TestRunEvictionErrorBranches(t *testing.T) {
	t.Run("list candidates error", func(t *testing.T) {
		mgr := newManagerWithDeps(t, &fakeStore{listStatesErr: errors.New("boom")}, newQueueStore(t), nil)
		if _, err := mgr.runEviction(context.Background(), capacitySettings{}, 10, 10); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z", SizeBytes: int64Ptr(10)},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := mgr.runEviction(ctx, capacitySettings{EffectiveMax: 1000, LowWatermark: 0.8}, 900, 100); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	})

	t.Run("delete candidate continue", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z", SizeBytes: int64Ptr(10)},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), &testDeps{statefs: &fakeStateFS{removeErr: errors.New("boom")}})
		summary, err := mgr.runEviction(context.Background(), capacitySettings{EffectiveMax: 1000, LowWatermark: 0.8}, 900, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.EvictedCount != 0 {
			t.Fatalf("expected no evictions, got %+v", summary)
		}
	})
}

func TestListEvictionCandidatesBranches(t *testing.T) {
	t.Run("child lookup error", func(t *testing.T) {
		mgr := newManagerWithDeps(t, &parentLookupErrStore{
			fakeStore: fakeStore{
				listStates: []store.StateEntry{
					{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z"},
				},
			},
			err: errors.New("boom"),
		}, newQueueStore(t), nil)
		if _, _, _, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("parent has children and age block", func(t *testing.T) {
		parentID := "state-parent"
		now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
		oldNow := nowUTCFn
		nowUTCFn = func() time.Time { return now }
		t.Cleanup(func() { nowUTCFn = oldNow })
		mgr := newManagerWithDeps(t, &fakeStore{
			listStates: []store.StateEntry{
				{StateID: parentID, ImageID: "image-1", CreatedAt: "bad-time", SizeBytes: int64Ptr(10)},
				{StateID: "state-child", ParentStateID: strPtr(parentID), ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z"},
			},
		}, newQueueStore(t), nil)
		_, blocked, _, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{MinStateAge: time.Hour})
		if err != nil {
			t.Fatalf("listEvictionCandidates: %v", err)
		}
		if blocked == 0 {
			t.Fatalf("expected blocked candidates")
		}
	})

	t.Run("size measured and clamped", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-a", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z"},
				{StateID: "state-b", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z"},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
		overrideCapacitySignals(t, nil, func(path string) (int64, error) {
			if strings.Contains(path, "state-a") {
				return 123, nil
			}
			return -5, nil
		})
		candidates, _, reclaimable, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{})
		if err != nil {
			t.Fatalf("listEvictionCandidates: %v", err)
		}
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(candidates))
		}
		if reclaimable != 123 {
			t.Fatalf("expected reclaimable=123, got %d", reclaimable)
		}
	})
}

func TestDeleteEvictionCandidatePathAndRemoveErrors(t *testing.T) {
	mgr := newManagerWithStateFS(t, &fakeStore{}, &errorStateFS{stateErr: errors.New("boom")})
	if err := mgr.deleteEvictionCandidate(context.Background(), evictCandidate{StateID: "state-1", ImageID: "image-1"}); err == nil {
		t.Fatalf("expected resolve path error")
	}

	mgr = newManagerWithStateFS(t, &fakeStore{}, &fakeStateFS{removeErr: errors.New("boom")})
	if err := mgr.deleteEvictionCandidate(context.Background(), evictCandidate{StateID: "state-1", ImageID: "image-1"}); err == nil {
		t.Fatalf("expected remove error")
	}
}

func TestCapacityErrorBranches(t *testing.T) {
	if resp := capacityError("code", "message", nil); resp == nil || resp.Details != "" {
		t.Fatalf("expected empty details response, got %+v", resp)
	}
	if resp := capacityError("code", "message", map[string]any{"bad": make(chan int)}); resp == nil || resp.Details != "" {
		t.Fatalf("expected marshal failure to drop details, got %+v", resp)
	}
}

func TestCapacityHelperAdditionalBranches(t *testing.T) {
	if defaultReserveBytes(0) != defaultReserveMinBytes {
		t.Fatalf("expected default reserve for zero total")
	}
	if _, ok := parseRFC3339Any("2026-02-22T12:00:00Z"); !ok {
		t.Fatalf("expected rfc3339 parse")
	}
	if isNoSpaceError(nil) {
		t.Fatalf("nil error must not be no-space")
	}
	if !isNoSpaceError(syscall.ENOSPC) {
		t.Fatalf("syscall ENOSPC must be detected")
	}
	if maxInt64(20, 10) != 20 {
		t.Fatalf("maxInt64 mismatch for first arg")
	}
}

func TestAsInt64AndAsFloat64AdditionalBranches(t *testing.T) {
	if _, ok := asInt64(int(1)); !ok {
		t.Fatalf("expected int parse")
	}
	if _, ok := asInt64(int32(1)); !ok {
		t.Fatalf("expected int32 parse")
	}
	if _, ok := asInt64(float64(1)); !ok {
		t.Fatalf("expected float64 integral parse")
	}
	if _, ok := asInt64(float64(1.2)); ok {
		t.Fatalf("expected float64 fractional rejection")
	}
	if _, ok := asInt64(json.Number("1e2")); ok {
		t.Fatalf("expected exponent rejection")
	}
	if _, ok := asFloat64(float32(0.5)); !ok {
		t.Fatalf("expected float32 parse")
	}
	if _, ok := asFloat64(int(1)); !ok {
		t.Fatalf("expected int parse")
	}
	if _, ok := asFloat64(int32(1)); !ok {
		t.Fatalf("expected int32 parse")
	}
	if _, ok := asFloat64(int64(1)); !ok {
		t.Fatalf("expected int64 parse")
	}
}

func TestMeasureStoreUsageMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	size, err := measureStoreUsage(path)
	if err != nil || size != 0 {
		t.Fatalf("expected missing path to be treated as zero usage, got size=%d err=%v", size, err)
	}
}

func TestRunEvictionAdditionalBranches(t *testing.T) {
	t.Run("break when already below threshold", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z", SizeBytes: int64Ptr(10)},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
		summary, err := mgr.runEviction(context.Background(), capacitySettings{
			EffectiveMax: 1000,
			LowWatermark: 0.80,
			ReserveBytes: 100,
		}, 700, 200)
		if err != nil {
			t.Fatalf("runEviction: %v", err)
		}
		if summary.EvictedCount != 0 {
			t.Fatalf("expected no evictions, got %+v", summary)
		}
	})

	t.Run("usage error after delete", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z", SizeBytes: int64Ptr(10)},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
		overrideCapacitySignals(t,
			func(string) (int64, int64, error) { return 1000, 100, nil },
			func(path string) (int64, error) {
				if strings.Contains(path, "state-1") {
					return 10, nil
				}
				return 0, errors.New("usage boom")
			},
		)
		_, err := mgr.runEviction(context.Background(), capacitySettings{
			EffectiveMax: 1000,
			LowWatermark: 0.80,
			ReserveBytes: 200,
		}, 950, 50)
		if err == nil || !strings.Contains(err.Error(), "usage boom") {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	t.Run("free error after delete", func(t *testing.T) {
		st := &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "2026-02-22T10:00:00Z", SizeBytes: int64Ptr(10)},
			},
		}
		mgr := newManagerWithDeps(t, st, newQueueStore(t), nil)
		overrideCapacitySignals(t,
			func(string) (int64, int64, error) { return 0, 0, errors.New("free boom") },
			func(path string) (int64, error) {
				if strings.Contains(path, "state-1") {
					return 10, nil
				}
				return 700, nil
			},
		)
		_, err := mgr.runEviction(context.Background(), capacitySettings{
			EffectiveMax: 1000,
			LowWatermark: 0.80,
			ReserveBytes: 200,
		}, 950, 50)
		if err == nil || !strings.Contains(err.Error(), "free boom") {
			t.Fatalf("expected free error, got %v", err)
		}
	})
}

func TestListEvictionCandidatesAdditionalBranches(t *testing.T) {
	t.Run("bad created_at with min age blocks candidate", func(t *testing.T) {
		now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
		oldNow := nowUTCFn
		nowUTCFn = func() time.Time { return now }
		t.Cleanup(func() { nowUTCFn = oldNow })

		mgr := newManagerWithDeps(t, &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-1", ImageID: "image-1", CreatedAt: "bad-time", SizeBytes: int64Ptr(10)},
			},
		}, newQueueStore(t), nil)
		candidates, blocked, _, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{
			MinStateAge: time.Hour,
		})
		if err != nil {
			t.Fatalf("listEvictionCandidates: %v", err)
		}
		if blocked != 1 || len(candidates) != 0 {
			t.Fatalf("expected blocked candidate, got blocked=%d candidates=%d", blocked, len(candidates))
		}
	})

	t.Run("tie-break by state id", func(t *testing.T) {
		now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
		created := now.Add(-2 * time.Hour).Format(time.RFC3339Nano)
		lastUsed := now.Add(-time.Hour).Format(time.RFC3339Nano)
		mgr := newManagerWithDeps(t, &fakeStore{
			listStates: []store.StateEntry{
				{StateID: "state-b", ImageID: "image-1", CreatedAt: created, LastUsedAt: strPtr(lastUsed), SizeBytes: int64Ptr(100)},
				{StateID: "state-a", ImageID: "image-1", CreatedAt: created, LastUsedAt: strPtr(lastUsed), SizeBytes: int64Ptr(100)},
			},
		}, newQueueStore(t), nil)
		candidates, blocked, _, err := mgr.listEvictionCandidates(context.Background(), capacitySettings{})
		if err != nil {
			t.Fatalf("listEvictionCandidates: %v", err)
		}
		if blocked != 0 || len(candidates) != 2 {
			t.Fatalf("unexpected result blocked=%d candidates=%d", blocked, len(candidates))
		}
		if candidates[0].StateID != "state-a" || candidates[1].StateID != "state-b" {
			t.Fatalf("expected state id tie-break ordering, got %+v", candidates)
		}
	})
}

func TestWithEvictLockAdditionalBranches(t *testing.T) {
	t.Run("returns non-exist lock error for directory lock path", func(t *testing.T) {
		root := t.TempDir()
		lockPath := filepath.Join(root, evictLockFileName)
		if err := os.MkdirAll(lockPath, 0o700); err != nil {
			t.Fatalf("mkdir lock dir: %v", err)
		}
		called := false
		err := withEvictLock(context.Background(), root, func() error {
			called = true
			return nil
		})
		if err == nil {
			t.Fatalf("expected lock acquire error")
		}
		if called {
			t.Fatalf("callback must not run on lock acquire failure")
		}
	})

	t.Run("retries until lock released", func(t *testing.T) {
		root := t.TempDir()
		lockPath := filepath.Join(root, evictLockFileName)
		if err := os.WriteFile(lockPath, []byte("busy"), 0o600); err != nil {
			t.Fatalf("write lock: %v", err)
		}
		go func() {
			time.Sleep(80 * time.Millisecond)
			_ = os.Remove(lockPath)
		}()
		called := false
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := withEvictLock(ctx, root, func() error {
			called = true
			return nil
		}); err != nil {
			t.Fatalf("withEvictLock: %v", err)
		}
		if !called {
			t.Fatalf("expected callback call after lock release")
		}
	})
}

func TestObservedMinStateBytesSkipsEmptySizes(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{
		listStates: []store.StateEntry{
			{StateID: "a", SizeBytes: nil},
			{StateID: "b", SizeBytes: int64Ptr(0)},
			{StateID: "c", SizeBytes: int64Ptr(50)},
		},
	}, newQueueStore(t), nil)
	if got := mgr.observedMinStateBytes(context.Background()); got != 50 {
		t.Fatalf("expected observed min 50, got %d", got)
	}
}

func TestCapacityParsingErrorBranches(t *testing.T) {
	if _, ok := asInt64(json.Number("999999999999999999999999")); ok {
		t.Fatalf("expected json int overflow to fail")
	}
	if _, ok := asFloat64(json.Number("not-a-number")); ok {
		t.Fatalf("expected invalid json float to fail")
	}
	if _, ok := parseRFC3339Any("not-a-time"); ok {
		t.Fatalf("expected invalid timestamp to fail")
	}
}

type parentLookupErrStore struct {
	fakeStore
	err error
}

func (s *parentLookupErrStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	if strings.TrimSpace(filters.ParentID) != "" && s.err != nil {
		return nil, s.err
	}
	return s.fakeStore.ListStates(ctx, filters)
}

type deleteStateErrStore struct {
	fakeStore
	deleteErr error
}

func (s *deleteStateErrStore) DeleteState(ctx context.Context, stateID string) error {
	s.deletedStates = append(s.deletedStates, stateID)
	return s.deleteErr
}

func int64Ptr(value int64) *int64 {
	return &value
}
