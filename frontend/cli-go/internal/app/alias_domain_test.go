package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
)

func TestResolvePrepareAliasWithOptionalRefLoadsDefinitionsViaAliasPackage(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	aliasPath := writePrepareAliasFile(t, cwd, "chinook.prep.s9s.yaml", "kind: psql\nimage: image\nargs:\n  - -f\n  - prepare.sql\n")

	def, gotPath, ref, err := resolvePrepareAliasWithOptionalRef(workspace, cwd, "chinook", "", "", false)
	if err != nil {
		t.Fatalf("resolvePrepareAliasWithOptionalRef: %v", err)
	}
	if ref != nil {
		t.Fatalf("expected nil ref context, got %+v", ref)
	}
	if gotPath != aliasPath {
		t.Fatalf("path = %q, want %q", gotPath, aliasPath)
	}
	if def.Class != aliaspkg.ClassPrepare {
		t.Fatalf("Class = %q, want %q", def.Class, aliaspkg.ClassPrepare)
	}
	if def.Kind != "psql" || def.Image != "image" {
		t.Fatalf("unexpected definition: %+v", def)
	}
	if got := strings.Join(def.Args, "|"); got != "-f|prepare.sql" {
		t.Fatalf("Args = %q, want %q", got, "-f|prepare.sql")
	}
}

func TestRunAliasExecutionLoadsDefinitionsViaAliasPackage(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	aliasPath := writeRunAliasFile(t, cwd, "smoke.run.s9s.yaml", "kind: pgbench\nargs:\n  - -c\n  - 10\n")

	def, gotPath, err := resolveRunAliasDefinition(workspace, cwd, "smoke")
	if err != nil {
		t.Fatalf("resolveRunAliasDefinition: %v", err)
	}
	if gotPath != aliasPath {
		t.Fatalf("path = %q, want %q", gotPath, aliasPath)
	}
	if def.Class != aliaspkg.ClassRun {
		t.Fatalf("Class = %q, want %q", def.Class, aliaspkg.ClassRun)
	}
	if def.Kind != "pgbench" || strings.TrimSpace(def.Image) != "" {
		t.Fatalf("unexpected definition: %+v", def)
	}
	if got := strings.Join(def.Args, "|"); got != "-c|10" {
		t.Fatalf("Args = %q, want %q", got, "-c|10")
	}
}
