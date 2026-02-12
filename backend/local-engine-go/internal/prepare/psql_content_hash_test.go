package prepare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPsqlContentDigestIgnoresIncludeForm(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, filepath.Join(dirA, "b.sql"), "select 1;")
	writeFile(t, filepath.Join(dirA, "a.sql"), "\\i b.sql\n")

	writeFile(t, filepath.Join(dirB, "d.sql"), "select 1;")
	writeFile(t, filepath.Join(dirB, "c.sql"), "\\include_relative d.sql\n")

	digestA, err := computePsqlContentDigest([]psqlInput{{kind: "file", value: filepath.Join(dirA, "a.sql")}}, dirA)
	if err != nil {
		t.Fatalf("digest a: %v", err)
	}
	digestB, err := computePsqlContentDigest([]psqlInput{{kind: "file", value: filepath.Join(dirB, "c.sql")}}, dirB)
	if err != nil {
		t.Fatalf("digest b: %v", err)
	}
	if digestA.hash != digestB.hash {
		t.Fatalf("expected same hash, got %s vs %s", digestA.hash, digestB.hash)
	}
}

func TestPsqlContentDigestNestedIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "c.sql"), "select 3;")
	writeFile(t, filepath.Join(dir, "b.sql"), "\\i c.sql\nselect 2;")
	writeFile(t, filepath.Join(dir, "a.sql"), "select 1;\n\\i b.sql")

	digest, err := computePsqlContentDigest([]psqlInput{{kind: "file", value: filepath.Join(dir, "a.sql")}}, dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if digest.hash == "" {
		t.Fatalf("expected hash")
	}
}

func TestPsqlContentDigestMissingInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sql"), "\\i missing.sql\n")
	_, err := computePsqlContentDigest([]psqlInput{{kind: "file", value: filepath.Join(dir, "a.sql")}}, dir)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
