package prepare

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sqlrs/engine-local/internal/store"
)

type CacheEvictionSummary struct {
	CompletedAt      string `json:"completed_at"`
	Trigger          string `json:"trigger"`
	EvictedCount     int    `json:"evicted_count"`
	FreedBytes       int64  `json:"freed_bytes"`
	BlockedCount     int    `json:"blocked_count"`
	ReclaimableBytes int64  `json:"reclaimable_bytes"`
	UsageBytesBefore int64  `json:"usage_bytes_before"`
	UsageBytesAfter  int64  `json:"usage_bytes_after"`
	FreeBytesBefore  int64  `json:"free_bytes_before"`
	FreeBytesAfter   int64  `json:"free_bytes_after"`
}

type CacheStatus struct {
	UsageBytes         int64                 `json:"usage_bytes"`
	ConfiguredMaxBytes *int64                `json:"configured_max_bytes,omitempty"`
	EffectiveMaxBytes  int64                 `json:"effective_max_bytes"`
	ReserveBytes       int64                 `json:"reserve_bytes"`
	HighWatermark      float64               `json:"high_watermark"`
	LowWatermark       float64               `json:"low_watermark"`
	MinStateAge        string                `json:"min_state_age"`
	StoreTotalBytes    int64                 `json:"store_total_bytes"`
	StoreFreeBytes     int64                 `json:"store_free_bytes"`
	StateCount         int                   `json:"state_count"`
	ReclaimableBytes   int64                 `json:"reclaimable_bytes"`
	BlockedCount       int                   `json:"blocked_count"`
	PressureReasons    []string              `json:"pressure_reasons"`
	LastEviction       *CacheEvictionSummary `json:"last_eviction,omitempty"`
}

func (m *PrepareService) CacheStatus(ctx context.Context) (CacheStatus, error) {
	if m == nil || m.store == nil {
		return CacheStatus{}, fmt.Errorf("prepare service is not configured")
	}
	root := strings.TrimSpace(m.stateStoreRoot)
	if root == "" {
		return CacheStatus{}, fmt.Errorf("state store root is not configured")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return CacheStatus{}, err
	}

	totalBytes, freeBytes, err := filesystemStatsFn(root)
	if err != nil {
		return CacheStatus{}, err
	}
	settings, err := m.loadCapacitySettings(totalBytes)
	if err != nil {
		return CacheStatus{}, err
	}
	usageBytes, err := cacheUsageFn(root)
	if err != nil {
		return CacheStatus{}, err
	}
	entries, err := m.store.ListStates(ctx, store.StateFilters{})
	if err != nil {
		return CacheStatus{}, err
	}
	_, blockedCount, reclaimableBytes, err := m.listEvictionCandidates(ctx, settings)
	if err != nil {
		return CacheStatus{}, err
	}
	status := CacheStatus{
		UsageBytes:        usageBytes,
		EffectiveMaxBytes: settings.EffectiveMax,
		ReserveBytes:      settings.ReserveBytes,
		HighWatermark:     settings.HighWatermark,
		LowWatermark:      settings.LowWatermark,
		MinStateAge:       settings.MinStateAge.String(),
		StoreTotalBytes:   totalBytes,
		StoreFreeBytes:    freeBytes,
		StateCount:        len(entries),
		ReclaimableBytes:  reclaimableBytes,
		BlockedCount:      blockedCount,
		PressureReasons:   pressureReasons(usageBytes, freeBytes, settings),
		LastEviction:      m.lastCacheEviction(),
	}
	if len(status.PressureReasons) == 0 {
		status.PressureReasons = []string{}
	}
	if settings.MaxBytes > 0 {
		configured := settings.MaxBytes
		status.ConfiguredMaxBytes = &configured
	}
	return status, nil
}

func (m *PrepareService) lastCacheEviction() *CacheEvictionSummary {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastEviction == nil {
		return nil
	}
	copy := *m.lastEviction
	return &copy
}

func (m *PrepareService) recordCacheEviction(summary CacheEvictionSummary) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := summary
	m.lastEviction = &copy
}
