package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/client"
)

func TestPrintLsStatesTableWithCacheDetails(t *testing.T) {
	size := int64(42)
	useCount := int64(7)
	lastUsedAt := "2026-03-09T12:00:00Z"
	minRetentionUntil := "2026-03-09T12:10:00Z"

	result := LsResult{
		States: &[]client.StateEntry{
			{
				StateID:           "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				ImageID:           "image-1",
				PrepareKind:       "psql",
				PrepareArgs:       "-c select 1",
				CreatedAt:         "2025-01-01T00:00:00Z",
				SizeBytes:         &size,
				LastUsedAt:        &lastUsedAt,
				UseCount:          &useCount,
				MinRetentionUntil: &minRetentionUntil,
				RefCount:          2,
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{LongIDs: true, CacheDetails: true})
	out := buf.String()
	for _, want := range []string{
		"LAST_USED",
		"USE_COUNT",
		"MIN_RETENTION_UNTIL",
		lastUsedAt,
		"7",
		minRetentionUntil,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}
