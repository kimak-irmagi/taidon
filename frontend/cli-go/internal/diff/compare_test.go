package diff

import (
	"reflect"
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

func TestCompare_StableOrderByPath(t *testing.T) {
	from := FileList{}
	to := FileList{Entries: []FileEntry{
		{Path: "z/b.sql", Hash: "h2"},
		{Path: "m/a.sql", Hash: "h1"},
		{Path: "m/c.sql", Hash: "h3"},
	}}
	results := make([][]string, 3)
	for i := 0; i < len(results); i++ {
		r := Compare(from, to, Options{})
		paths := make([]string, len(r.Added))
		for j, e := range r.Added {
			paths[j] = e.Path
		}
		results[i] = paths
	}
	want := []string{"m/a.sql", "m/c.sql", "z/b.sql"}
	for i := 1; i < len(results); i++ {
		if !reflect.DeepEqual(results[i], results[0]) || !reflect.DeepEqual(results[0], want) {
			t.Fatalf("iteration 0=%v iteration %d=%v want %v", results[0], i, results[i], want)
		}
	}
}
