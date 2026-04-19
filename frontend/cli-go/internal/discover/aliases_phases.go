package discover

import (
	"sort"

	"github.com/sqlrs/cli/internal/alias"
)

func rankValidatedCandidates(validated []validatedCandidate) {
	sort.Slice(validated, func(i, j int) bool {
		if validated[i].Score != validated[j].Score {
			return validated[i].Score > validated[j].Score
		}
		if validated[i].WorkspaceRel != validated[j].WorkspaceRel {
			return validated[i].WorkspaceRel < validated[j].WorkspaceRel
		}
		if validated[i].Class != validated[j].Class {
			return validated[i].Class < validated[j].Class
		}
		return validated[i].Kind < validated[j].Kind
	})
}

func selectAliasFindings(validated []validatedCandidate, coverage map[string]struct{}, workspaceRoot string, cwd string, shellFamily string, progress Progress) ([]Finding, int, error) {
	inbound := inboundEdges(validated)
	findings := make([]Finding, 0, len(validated))
	seenAliasPaths := make(map[string]struct{}, len(validated))
	normalizedShell := normalizedShellFamily(shellFamily)
	suppressed := 0

	for _, candidate := range validated {
		candidateKeys := discoverPathKeys(candidate.WorkspaceRoot, candidate.WorkspaceRel, candidate.AbsPath)
		if hasAnyInbound(inbound, candidateKeys) {
			suppressed++
			emitAliasSuppressed(progress, candidate, "covered by inbound dependency")
			continue
		}
		target, err := alias.ResolveCreateTarget(alias.CreateOptions{
			WorkspaceRoot: workspaceRoot,
			CWD:           cwd,
			Ref:           candidate.Ref,
			Class:         candidate.Class,
		})
		if err != nil {
			return nil, suppressed, err
		}
		if _, ok := seenAliasPaths[target.File]; ok {
			suppressed++
			emitAliasSuppressed(progress, candidate, "duplicate alias path")
			continue
		}
		seenAliasPaths[target.File] = struct{}{}
		if _, ok := coverage[target.File]; ok {
			suppressed++
			emitAliasSuppressed(progress, candidate, "covered by existing alias")
			continue
		}

		findings = append(findings, buildAliasFinding(candidate, target.File, normalizedShell))
	}
	return findings, suppressed, nil
}

func sortAliasFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Type != findings[j].Type {
			return findings[i].Type < findings[j].Type
		}
		return findings[i].Kind < findings[j].Kind
	})
}

func buildAliasFinding(candidate validatedCandidate, aliasPath string, shellFamily string) Finding {
	return Finding{
		Analyzer:      AnalyzerAliases,
		Target:        candidate.WorkspaceRel,
		Action:        "materialize a repo-tracked alias file",
		Type:          candidate.Class,
		Kind:          candidate.Kind,
		Ref:           candidate.Ref,
		File:          candidate.WorkspaceRel,
		AliasPath:     aliasPath,
		Reason:        candidate.Reason,
		CreateCommand: candidate.Command,
		FollowUpCommand: &FollowUpCommand{
			ShellFamily: normalizedShellFamily(shellFamily),
			Command:     candidate.Command,
		},
		Score: candidate.Score,
		Valid: candidate.Valid,
		Error: candidate.Error,
	}
}

func emitAliasSuppressed(progress Progress, candidate validatedCandidate, reason string) {
	emitProgress(progress, ProgressEvent{
		Stage:  ProgressStageSuppressed,
		Class:  candidate.Class,
		Kind:   candidate.Kind,
		Ref:    candidate.Ref,
		File:   candidate.WorkspaceRel,
		Score:  candidate.Score,
		Reason: reason,
	})
}
