package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/client"
)

func TestPrintLsUsageIncludesWideAndLongTimestampHint(t *testing.T) {
	var buf bytes.Buffer
	PrintLsUsage(&buf)
	out := buf.String()
	if !strings.Contains(out, "--wide") {
		t.Fatalf("expected --wide in usage, got %q", out)
	}
	if !strings.Contains(out, "absolute timestamps") {
		t.Fatalf("expected --long timestamp hint in usage, got %q", out)
	}
}

func TestPrintLsStatesTableHeaderUsesKind(t *testing.T) {
	result := LsResult{States: &[]client.StateEntry{sampleStateEntry(longPrepareArgs())}}
	out := renderStatesToFakeTTY(t, result, LsPrintOptions{Quiet: true}, 140)
	firstLine := strings.Split(strings.TrimRight(out, "\n"), "\n")[0]
	if !strings.Contains(firstLine, "KIND") {
		t.Fatalf("expected KIND header, got %q", firstLine)
	}
	if strings.Contains(firstLine, "PREPARE_KIND") {
		t.Fatalf("expected PREPARE_KIND header removed, got %q", firstLine)
	}
}

func TestPrintLsStatesTableCreatedRelativeByDefault(t *testing.T) {
	withLSNow(t, time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC), func() {
		state := sampleStateEntry("-f prepare.sql")
		state.CreatedAt = "2026-03-07T12:00:00Z"
		result := LsResult{States: &[]client.StateEntry{state}}
		var buf bytes.Buffer
		PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true})
		out := buf.String()
		if !strings.Contains(out, "3d ago") {
			t.Fatalf("expected relative created time, got %q", out)
		}
	})
}

func TestPrintLsStatesTableLongShowsAbsoluteCreatedAtSeconds(t *testing.T) {
	state := sampleStateEntry("-f prepare.sql")
	state.CreatedAt = "2026-03-07T12:34:56.789Z"
	result := LsResult{States: &[]client.StateEntry{state}}
	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true, LongIDs: true})
	out := buf.String()
	if !strings.Contains(out, "2026-03-07T12:34:56Z") {
		t.Fatalf("expected absolute second-precision created time, got %q", out)
	}
	if strings.Contains(out, ".789") {
		t.Fatalf("expected fractional seconds omitted, got %q", out)
	}
}

func TestPrintLsStatesTableTruncatesPrepareArgsInMiddleOnTTY(t *testing.T) {
	result := LsResult{States: &[]client.StateEntry{sampleStateEntry(longPrepareArgs())}}
	out := renderStatesToFakeTTY(t, result, LsPrintOptions{Quiet: true, NoHeader: true}, 96)
	if !strings.Contains(out, " ... ") {
		t.Fatalf("expected middle truncation marker, got %q", out)
	}
	if !strings.Contains(out, "-f /works") {
		t.Fatalf("expected leading prefix fragment preserved, got %q", out)
	}
	if !strings.Contains(out, "finish.sql") {
		t.Fatalf("expected trailing suffix fragment preserved, got %q", out)
	}
	if strings.Contains(out, longPrepareArgs()) {
		t.Fatalf("expected truncated args, got %q", out)
	}
}

func TestPrintLsStatesTableKeepsShortPrepareArgsOnTTY(t *testing.T) {
	shortArgs := "-f prepare.sql -X -v ON_ERROR_STOP=1"
	result := LsResult{States: &[]client.StateEntry{sampleStateEntry(shortArgs)}}
	out := renderStatesToFakeTTY(t, result, LsPrintOptions{Quiet: true, NoHeader: true}, 120)
	if !strings.Contains(out, shortArgs) {
		t.Fatalf("expected short args unchanged, got %q", out)
	}
	if strings.Contains(out, " ... ") {
		t.Fatalf("expected no truncation marker, got %q", out)
	}
}

func TestPrintLsStatesTableUsesCompactBudgetWhenNotTTY(t *testing.T) {
	withLSNow(t, time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC), func() {
		state := sampleStateEntry(longPrepareArgs())
		state.CreatedAt = "2026-03-09T12:00:00Z"
		result := LsResult{States: &[]client.StateEntry{state}}
		var buf bytes.Buffer
		PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true})
		out := buf.String()
		if !strings.Contains(out, " ... ") {
			t.Fatalf("expected compact truncation outside tty, got %q", out)
		}
		if !strings.Contains(out, "1d ago") {
			t.Fatalf("expected relative created time outside tty, got %q", out)
		}
	})
}

func TestPrintLsStatesTableKeepsSinglePhysicalLineOnNarrowTTY(t *testing.T) {
	result := LsResult{States: &[]client.StateEntry{sampleStateEntry(longPrepareArgs())}}
	out := renderStatesToFakeTTY(t, result, LsPrintOptions{Quiet: true, NoHeader: true}, 48)
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected single physical line plus trailing newline, got %q", out)
	}
}

func TestPrintLsStatesTableFitsTTYWidth140WithDeepTree(t *testing.T) {
	withLSNow(t, time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC), func() {
		result := LsResult{States: buildDeepStateRows(12, longPrepareArgs())}
		out := renderStatesToFakeTTY(t, result, LsPrintOptions{Quiet: true}, 140)
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if line == "" {
				continue
			}
			if len([]rune(line)) > 140 {
				t.Fatalf("expected line to fit width 140, got %d chars in %q", len([]rune(line)), line)
			}
		}
	})
}

func TestPrintLsStatesTableWideShowsFullPrepareArgs(t *testing.T) {
	result := LsResult{States: &[]client.StateEntry{sampleStateEntry(longPrepareArgs())}}
	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true, Wide: true})
	out := buf.String()
	if !strings.Contains(out, longPrepareArgs()) {
		t.Fatalf("expected full args in wide output, got %q", out)
	}
	if strings.Contains(out, " ... ") {
		t.Fatalf("expected no truncation marker in wide output, got %q", out)
	}
}

func TestPrintLsStatesTableLongStillTruncatesPrepareArgs(t *testing.T) {
	state := sampleStateEntry(longPrepareArgs())
	state.CreatedAt = "2026-03-07T12:34:56.789Z"
	result := LsResult{States: &[]client.StateEntry{state}}
	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true, LongIDs: true})
	out := buf.String()
	if !strings.Contains(out, fullStateID()) {
		t.Fatalf("expected full state id in long output, got %q", out)
	}
	if !strings.Contains(out, "2026-03-07T12:34:56Z") {
		t.Fatalf("expected absolute created time in long output, got %q", out)
	}
	if !strings.Contains(out, " ... ") {
		t.Fatalf("expected prepare args to remain truncated in --long output, got %q", out)
	}
	if strings.Contains(out, longPrepareArgs()) {
		t.Fatalf("expected --long not to imply --wide, got %q", out)
	}
}

func TestPrintLsStatesTableWideAndLongShowsFullIDsAndArgs(t *testing.T) {
	state := sampleStateEntry(longPrepareArgs())
	state.CreatedAt = "2026-03-07T12:34:56.789Z"
	result := LsResult{States: &[]client.StateEntry{state}}
	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true, NoHeader: true, LongIDs: true, Wide: true})
	out := buf.String()
	if !strings.Contains(out, fullStateID()) {
		t.Fatalf("expected full state id, got %q", out)
	}
	if !strings.Contains(out, longPrepareArgs()) {
		t.Fatalf("expected full args, got %q", out)
	}
	if !strings.Contains(out, "2026-03-07T12:34:56Z") {
		t.Fatalf("expected absolute created time, got %q", out)
	}
}

func TestPrintLsStatesTableWithCacheDetailsWideShowsFullPrepareArgs(t *testing.T) {
	size := int64(42)
	useCount := int64(7)
	lastUsedAt := "2026-03-09T12:00:00Z"
	minRetentionUntil := "2026-03-09T12:10:00Z"
	state := sampleStateEntry(longPrepareArgs())
	state.SizeBytes = &size
	state.LastUsedAt = &lastUsedAt
	state.UseCount = &useCount
	state.MinRetentionUntil = &minRetentionUntil
	result := LsResult{States: &[]client.StateEntry{state}}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true, LongIDs: true, Wide: true, CacheDetails: true})
	out := buf.String()
	for _, want := range []string{longPrepareArgs(), "LAST_USED", lastUsedAt, minRetentionUntil} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func renderStatesToFakeTTY(t *testing.T, result LsResult, opts LsPrintOptions, width int) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "ls-tty-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	oldIsTerminal := isTerminal
	oldGetTermSize := getTermSize
	isTerminal = func(fd int) bool {
		return fd == int(file.Fd())
	}
	getTermSize = func(fd int) (int, int, error) {
		if fd != int(file.Fd()) {
			return 0, 0, fmt.Errorf("unexpected fd %d", fd)
		}
		return width, 24, nil
	}
	defer func() {
		isTerminal = oldIsTerminal
		getTermSize = oldGetTermSize
	}()

	PrintLs(file, result, opts)
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func withLSNow(t *testing.T, now time.Time, fn func()) {
	t.Helper()
	old := lsNow
	lsNow = func() time.Time { return now.UTC() }
	defer func() { lsNow = old }()
	fn()
}

func sampleStateEntry(args string) client.StateEntry {
	size := int64(42)
	return client.StateEntry{
		StateID:     fullStateID(),
		ImageID:     "postgres@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PrepareKind: "psql",
		PrepareArgs: args,
		CreatedAt:   "2025-01-01T00:00:00Z",
		SizeBytes:   &size,
		RefCount:    2,
	}
}

func buildDeepStateRows(depth int, args string) *[]client.StateEntry {
	rows := make([]client.StateEntry, 0, depth)
	var parent *string
	for i := 0; i < depth; i++ {
		id := fmt.Sprintf("%064x", i+1)
		row := sampleStateEntry(args)
		row.StateID = id
		row.ParentStateID = parent
		row.CreatedAt = "2026-03-07T12:00:00Z"
		rows = append(rows, row)
		parent = &rows[len(rows)-1].StateID
	}
	return &rows
}

func fullStateID() string {
	return "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
}

func longPrepareArgs() string {
	return "-f /workspace/sql/prepare-start.sql --set some.really.long.setting.value=abcdefghijklmnopqrstuvwxyz0123456789 -X -v ON_ERROR_STOP=1 --tail /workspace/sql/prepare-finish.sql"
}
