package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildLbFileList_SingleChangelog(t *testing.T) {
	dir := t.TempDir()
	changelog := filepath.Join(dir, "changelog.xml")
	if err := os.WriteFile(changelog, []byte(`<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Root: dir}
	list, err := BuildLbFileList(ctx, []string{"--changelog-file", "changelog.xml", "update"})
	if err != nil {
		t.Fatalf("BuildLbFileList: %v", err)
	}
	if len(list.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list.Entries))
	}
	if list.Entries[0].Path != "changelog.xml" {
		t.Fatalf("expected path changelog.xml, got %q", list.Entries[0].Path)
	}
}

func TestBuildLbFileList_WithInclude(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	inc := filepath.Join(sub, "001.xml")
	if err := os.WriteFile(inc, []byte(`<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	changelog := filepath.Join(dir, "master.xml")
	if err := os.WriteFile(changelog, []byte(`<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog">
  <include file="sub/001.xml"/>
</databaseChangeLog>`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Root: dir}
	list, err := BuildLbFileList(ctx, []string{"--changelog-file", "master.xml"})
	if err != nil {
		t.Fatalf("BuildLbFileList: %v", err)
	}
	if len(list.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list.Entries))
	}
}

func TestBuildLbFileList_NoChangelogFile(t *testing.T) {
	ctx := Context{Root: t.TempDir()}
	_, err := BuildLbFileList(ctx, []string{"update"})
	if err == nil || err.Error() != "liquibase command has no --changelog-file (required for diff)" {
		t.Fatalf("expected error about --changelog-file, got %v", err)
	}
}
