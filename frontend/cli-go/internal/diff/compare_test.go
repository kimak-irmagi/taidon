package diff

import (
	"testing"
)

func TestCompare_EmptyBoth(t *testing.T) {
	result := Compare(FileList{}, FileList{}, Options{})
	if len(result.Added) != 0 || len(result.Modified) != 0 || len(result.Removed) != 0 {
		t.Fatalf("expected empty result, got %+v", result)
	}
}

func TestCompare_Added(t *testing.T) {
	from := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h1"}}}
	to := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h1"}, {Path: "b.sql", Hash: "h2"}}}
	result := Compare(from, to, Options{})
	if len(result.Added) != 1 || result.Added[0].Path != "b.sql" {
		t.Fatalf("expected one added b.sql, got %+v", result.Added)
	}
	if len(result.Modified) != 0 || len(result.Removed) != 0 {
		t.Fatalf("expected no modified/removed, got %d %d", len(result.Modified), len(result.Removed))
	}
}

func TestCompare_Removed(t *testing.T) {
	from := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h1"}, {Path: "b.sql", Hash: "h2"}}}
	to := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h1"}}}
	result := Compare(from, to, Options{})
	if len(result.Removed) != 1 || result.Removed[0].Path != "b.sql" {
		t.Fatalf("expected one removed b.sql, got %+v", result.Removed)
	}
	if len(result.Added) != 0 || len(result.Modified) != 0 {
		t.Fatalf("expected no added/modified, got %d %d", len(result.Added), len(result.Modified))
	}
}

func TestCompare_Modified(t *testing.T) {
	from := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h1"}}}
	to := FileList{Entries: []FileEntry{{Path: "a.sql", Hash: "h2"}}}
	result := Compare(from, to, Options{})
	if len(result.Modified) != 1 || result.Modified[0].Path != "a.sql" || result.Modified[0].Hash != "h2" {
		t.Fatalf("expected one modified a.sql, got %+v", result.Modified)
	}
	if len(result.Added) != 0 || len(result.Removed) != 0 {
		t.Fatalf("expected no added/removed, got %d %d", len(result.Added), len(result.Removed))
	}
}
