package discover

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
)

func TestSelectAliasFindingsPreservesSuppressionAndRanking(t *testing.T) {
	workspace := t.TempDir()
	validated := []validatedCandidate{
		{
			candidateProposal: candidateProposal{
				fileRecord: fileRecord{
					AbsPath:       filepath.Join(workspace, "schema.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  "schema.sql",
					CwdRel:        "schema.sql",
				},
				Class:   alias.ClassPrepare,
				Kind:    "psql",
				Score:   80,
				Reason:  "migration/setup path",
				Ref:     "schema",
				Command: "sqlrs alias create schema prepare:psql -- -f schema.sql",
			},
			Valid: true,
			Closure: map[string]struct{}{
				"schema.sql": {},
			},
		},
		{
			candidateProposal: candidateProposal{
				fileRecord: fileRecord{
					AbsPath:       filepath.Join(workspace, "child.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  "child.sql",
					CwdRel:        "child.sql",
				},
				Class:   alias.ClassPrepare,
				Kind:    "psql",
				Score:   40,
				Reason:  "query fragment",
				Ref:     "child",
				Command: "sqlrs alias create child prepare:psql -- -f child.sql",
			},
			Valid: true,
			Closure: map[string]struct{}{
				"child.sql": {},
			},
		},
		{
			candidateProposal: candidateProposal{
				fileRecord: fileRecord{
					AbsPath:       filepath.Join(workspace, "bench", "run.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  filepath.ToSlash(filepath.Join("bench", "run.sql")),
					CwdRel:        filepath.ToSlash(filepath.Join("bench", "run.sql")),
				},
				Class:   alias.ClassRun,
				Kind:    "pgbench",
				Score:   70,
				Reason:  "benchmark path",
				Ref:     "bench",
				Command: "sqlrs alias create bench run:pgbench -- -f bench/run.sql",
			},
			Valid: true,
			Closure: map[string]struct{}{
				filepath.ToSlash(filepath.Join("bench", "run.sql")): {},
				"child.sql": {},
			},
		},
		{
			candidateProposal: candidateProposal{
				fileRecord: fileRecord{
					AbsPath:       filepath.Join(workspace, "ignored.sql"),
					WorkspaceRoot: workspace,
					WorkspaceRel:  "ignored.sql",
					CwdRel:        "ignored.sql",
				},
				Class:   alias.ClassPrepare,
				Kind:    "psql",
				Score:   10,
				Reason:  "covered by existing alias",
				Ref:     "ignored",
				Command: "sqlrs alias create ignored prepare:psql -- -f ignored.sql",
			},
			Valid: true,
		},
	}

	rankValidatedCandidates(validated)
	coverage := map[string]struct{}{
		"ignored.prep.s9s.yaml": {},
	}

	findings, suppressed, err := selectAliasFindings(validated, coverage, workspace, workspace, ShellFamilyPOSIX, nil)
	if err != nil {
		t.Fatalf("selectAliasFindings: %v", err)
	}
	if suppressed != 2 {
		t.Fatalf("suppressed = %d, want %d", suppressed, 2)
	}
	if len(findings) != 2 {
		t.Fatalf("expected two surviving findings, got %+v", findings)
	}

	if findings[0].AliasPath != "schema.prep.s9s.yaml" {
		t.Fatalf("unexpected first finding: %+v", findings[0])
	}
	if findings[1].AliasPath != "bench.run.s9s.yaml" {
		t.Fatalf("unexpected second finding: %+v", findings[1])
	}
	if findings[0].FollowUpCommand == nil || findings[0].FollowUpCommand.ShellFamily != ShellFamilyPOSIX {
		t.Fatalf("expected POSIX follow-up command: %+v", findings[0])
	}
	if !strings.Contains(findings[0].CreateCommand, "prepare:psql") {
		t.Fatalf("unexpected create command: %+v", findings[0])
	}
	if !strings.Contains(findings[1].CreateCommand, "run:pgbench") {
		t.Fatalf("unexpected create command: %+v", findings[1])
	}
}
