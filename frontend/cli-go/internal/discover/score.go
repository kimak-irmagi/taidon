package discover

import (
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
)

func scoreDiscoverFiles(files []fileRecord) []candidateProposal {
	proposals := make([]candidateProposal, 0, len(files))
	for _, file := range files {
		proposal, ok := proposeCandidate(file)
		if !ok {
			continue
		}
		proposals = append(proposals, proposal)
	}
	return proposals
}

func proposeCandidate(file fileRecord) (candidateProposal, bool) {
	candidates := []candidateProposal{
		scorePrepareLiquibase(file),
		scorePreparePsql(file),
		scoreRunPgbench(file),
		scoreRunPsql(file),
	}
	best := candidateProposal{}
	found := false
	for _, proposal := range candidates {
		if proposal.Score <= 0 {
			continue
		}
		if !found || proposal.Score > best.Score || (proposal.Score == best.Score && proposalPriority(proposal) < proposalPriority(best)) {
			best = proposal
			found = true
		}
	}
	if !found {
		return candidateProposal{}, false
	}
	return best, true
}

func proposalPriority(proposal candidateProposal) int {
	switch {
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "lb":
		return 0
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "psql":
		return 1
	case proposal.Class == alias.ClassRun && proposal.Kind == "pgbench":
		return 2
	case proposal.Class == alias.ClassRun && proposal.Kind == "psql":
		return 3
	default:
		return 4
	}
}

func scorePrepareLiquibase(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassPrepare, Kind: "lb"}
	if !isLiquibaseCandidateExtension(file.Ext) {
		return result
	}
	if file.BinaryOnly {
		if points, reason := scoreContains(file.LowerPath, []string{"liquibase", "changelog", "change-log", "dbchangelog", "changeset", "master", "migration", "migrations"}, 40, "Liquibase binary artifact path"); points > 0 {
			result.Score += points
			result.Reason = appendReason(result.Reason, reason)
			result.Score += 5
			result.Reason = appendReason(result.Reason, "binary Liquibase artifact")
			result.Ref = suggestedAliasRef(file)
		}
		return result
	}
	if points, reason := scoreContains(file.LowerPath, []string{"liquibase", "changelog", "change-log", "dbchangelog", "changeset", "master", "migration", "migrations"}, 40, "Liquibase changelog path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"databasechangelog", "changeset", "includeall", "<include", "relativetochangelogfile", "--liquibase formatted sql", "--changeset", "--rollback"}, 30, "Liquibase changelog markup"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"include file=", "includeall path=", "file=\"", "path=\""}, 10, "Liquibase include graph"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score > 0 {
		if ref := liquibaseRootHint(file.CwdRel); ref != "" {
			result.Ref = ref
		} else {
			result.Ref = suggestedAliasRef(file)
		}
	}
	return result
}

func scorePreparePsql(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassPrepare, Kind: "psql"}
	if file.Ext != ".sql" {
		return result
	}
	if points, reason := scoreContains(file.LowerPath, []string{"schema", "migration", "migrations", "init", "setup", "seed", "bootstrap", "ddl", "prepare", "db"}, 40, "migration/setup path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"create table", "alter table", "drop table", "insert into", "update ", "delete from", "create schema", "grant ", "revoke "}, 30, "DDL or write statement"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"\\i ", "\\include ", "\\ir ", "\\include_relative "}, 10, "psql include graph"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score > 0 {
		result.Ref = suggestedAliasRef(file)
	}
	return result
}

func scoreRunPgbench(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassRun, Kind: "pgbench"}
	if file.Ext != ".sql" {
		return result
	}
	if points, reason := scoreContains(file.LowerPath, []string{"bench", "benchmark", "pgbench", "perf", "performance", "load", "stress", "tps"}, 40, "benchmark path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"\\setrandom", "\\shell", "\\sleep", "pgbench"}, 30, "pgbench workload markers"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score > 0 {
		result.Ref = suggestedAliasRef(file)
	}
	return result
}

func scoreRunPsql(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassRun, Kind: "psql"}
	if file.Ext != ".sql" {
		return result
	}
	if points, reason := scoreContains(file.LowerPath, []string{"query", "queries", "smoke", "test", "verify", "check", "report", "read", "readonly", "select", "run"}, 40, "query/test path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"select ", "with ", "explain ", "show ", "describe ", "\\echo ", "\\timing "}, 30, "query fragment"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"create table", "alter table", "drop table", "insert into", "update ", "delete from"}, 5, "mixed SQL"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score > 0 {
		result.Ref = suggestedAliasRef(file)
	}
	return result
}

func suggestedAliasRef(file fileRecord) string {
	workspaceRoot := filepath.ToSlash(strings.TrimSpace(file.WorkspaceRoot))
	cwdRel := filepath.ToSlash(strings.TrimSpace(file.CwdRel))
	if cwdRel == "" {
		return filepath.ToSlash(filepath.Join(workspaceRoot, pathStem(file.AbsPath)))
	}
	if filepath.IsAbs(cwdRel) {
		if filepath.Dir(file.WorkspaceRel) == "." {
			return filepath.ToSlash(filepath.Join(workspaceRoot, pathStem(file.AbsPath)))
		}
		return filepath.ToSlash(filepath.Join(workspaceRoot, filepath.Dir(file.WorkspaceRel)))
	}

	dir := filepath.ToSlash(filepath.Dir(cwdRel))
	if dir == "." || dir == "" {
		return pathStem(file.AbsPath)
	}
	if isAncestorOnlyPath(dir) {
		stem := pathStem(cwdRel)
		return filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), stem))
	}
	return dir
}

func pathStem(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		return base
	}
	return stem
}

func isAncestorOnlyPath(value string) bool {
	cleaned := filepath.ToSlash(strings.TrimSpace(value))
	if cleaned == "" {
		return false
	}
	parts := strings.Split(cleaned, "/")
	for _, part := range parts {
		if part != ".." {
			return false
		}
	}
	return true
}

func scoreContains(value string, keywords []string, points int, reason string) (int, string) {
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			return points, reason
		}
	}
	return 0, ""
}

func appendReason(base string, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return reason
	}
	return base + "; " + reason
}

func liquibaseDiscoverArgs(fileRef string) []string {
	args := []string{"update"}
	if hint := liquibaseRootHint(fileRef); hint != "" {
		args = append(args, "--searchPath", hint)
	}
	args = append(args, "--changelog-file", fileRef)
	return args
}

func liquibaseDiscoverCreateCommand(ref string, fileRef string) []string {
	args := []string{"sqlrs", "alias", "create", ref, "prepare:lb", "--"}
	return append(args, liquibaseDiscoverArgs(fileRef)...)
}

func liquibaseRootHint(cwdRel string) string {
	rel := filepath.ToSlash(strings.TrimSpace(cwdRel))
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, "/")
	markers := [][]string{
		{"config", "liquibase"},
		{"db", "changelog"},
	}
	for _, marker := range markers {
		for i := 0; i+len(marker) <= len(parts); i++ {
			match := true
			for j, segment := range marker {
				if !strings.EqualFold(parts[i+j], segment) {
					match = false
					break
				}
			}
			if match && i > 0 {
				return strings.Join(parts[:i], "/")
			}
		}
	}
	return ""
}
