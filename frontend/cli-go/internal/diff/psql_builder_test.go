package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPsqlFileList_SingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.sql")
	if err := os.WriteFile(f, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Root: dir}
	list, err := BuildPsqlFileList(ctx, []string{"-f", "a.sql"})
	if err != nil {
		t.Fatalf("BuildPsqlFileList: %v", err)
	}
	if len(list.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list.Entries))
	}
	if list.Entries[0].Path != "a.sql" {
		t.Fatalf("expected path a.sql, got %q", list.Entries[0].Path)
	}
	if list.Entries[0].Hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestBuildPsqlFileList_WithInclude(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.sql")
	b := filepath.Join(dir, "b.sql")
	if err := os.WriteFile(b, []byte("select 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, []byte("\\i b.sql\nselect 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Root: dir}
	list, err := BuildPsqlFileList(ctx, []string{"--", "-f", "a.sql"})
	if err != nil {
		t.Fatalf("BuildPsqlFileList: %v", err)
	}
	if len(list.Entries) != 2 {
		t.Fatalf("expected 2 entries (a then b), got %d", len(list.Entries))
	}
	if list.Entries[0].Path != "a.sql" || list.Entries[1].Path != "b.sql" {
		t.Fatalf("unexpected order: %q %q", list.Entries[0].Path, list.Entries[1].Path)
	}
}

func TestBuildPsqlFileList_NoF(t *testing.T) {
	ctx := Context{Root: t.TempDir()}
	_, err := BuildPsqlFileList(ctx, []string{"--", "-c", "select 1"})
	if err == nil || err.Error() != "psql command has no -f file (required for diff)" {
		t.Fatalf("expected error about -f, got %v", err)
	}
}
