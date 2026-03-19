package prepare

import "testing"

func TestPsqlTaskHashSchemaAffectsHash(t *testing.T) {
	v1 := psqlTaskHashWithSchema("psql", "content", "engine", "psql-task-hash-v1")
	v2 := psqlTaskHashWithSchema("psql", "content", "engine", "psql-task-hash-v2")
	if v1 == "" || v2 == "" {
		t.Fatalf("expected non-empty hashes")
	}
	if v1 == v2 {
		t.Fatalf("expected schema salt to change hash")
	}
}

func TestPsqlTaskHashUsesCurrentSchemaConstant(t *testing.T) {
	got := psqlTaskHash("psql", "content", "engine")
	want := psqlTaskHashWithSchema("psql", "content", "engine", psqlTaskHashSchema)
	if got != want {
		t.Fatalf("psqlTaskHash() = %q, want %q", got, want)
	}
}
