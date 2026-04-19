package discover

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
)

func validateDiscoverProposals(proposals []candidateProposal, workspaceRoot string, cwd string, progress Progress) ([]validatedCandidate, error) {
	validated := make([]validatedCandidate, 0, len(proposals))
	for _, proposal := range proposals {
		emitProgress(progress, ProgressEvent{
			Stage:  ProgressStageCandidate,
			Class:  proposal.Class,
			Kind:   proposal.Kind,
			Ref:    proposal.Ref,
			File:   proposal.WorkspaceRel,
			Score:  proposal.Score,
			Reason: proposal.Reason,
		})
		result, err := validateCandidate(proposal, workspaceRoot, cwd)
		if err != nil {
			return nil, err
		}
		emitProgress(progress, ProgressEvent{
			Stage:  ProgressStageValidated,
			Class:  result.Class,
			Kind:   result.Kind,
			Ref:    result.Ref,
			File:   result.WorkspaceRel,
			Score:  result.Score,
			Reason: result.Reason,
			Error:  result.Error,
			Valid:  result.Valid,
		})
		validated = append(validated, result)
	}
	return validated, nil
}

func validateCandidate(proposal candidateProposal, workspaceRoot string, cwd string) (validatedCandidate, error) {
	result := validatedCandidate{
		candidateProposal: proposal,
		Closure:           map[string]struct{}{},
	}
	workspaceResolver := inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil)
	resolver := workspaceResolver
	aliasDir := ""
	if target, err := alias.ResolveCreateTarget(alias.CreateOptions{
		WorkspaceRoot: workspaceRoot,
		CWD:           cwd,
		Ref:           proposal.Ref,
		Class:         proposal.Class,
	}); err == nil {
		resolver = inputset.NewAliasResolver(workspaceRoot, target.Path)
		aliasDir = filepath.Dir(target.Path)
	}
	var (
		inputSet inputset.InputSet
		err      error
	)
	switch {
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "psql":
		inputSet, err = inputpsql.Collect([]string{"-f", validationPathForAliasDir(proposal.AbsPath, proposal.CwdRel, aliasDir)}, resolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "lb":
		inputSet, err = inputliquibase.Collect(liquibaseDiscoverArgs(proposal.CwdRel), workspaceResolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassRun && proposal.Kind == "psql":
		inputSet, err = inputpsql.Collect([]string{"-f", validationPathForAliasDir(proposal.AbsPath, proposal.CwdRel, aliasDir)}, resolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassRun && proposal.Kind == "pgbench":
		inputSet, err = inputpgbench.Collect([]string{"-f", validationPathForAliasDir(proposal.AbsPath, proposal.CwdRel, aliasDir)}, resolver, inputset.OSFileSystem{})
	default:
		err = fmt.Errorf("unsupported discover candidate kind: %s:%s", proposal.Class, proposal.Kind)
	}

	if err != nil {
		result.Valid = false
		result.Error = err.Error()
		result.Command = buildCreateCommand(proposal.Ref, proposal.Class, proposal.Kind, proposal.CwdRel)
		return result, nil
	}

	result.Valid = true
	result.Closure = make(map[string]struct{}, len(inputSet.Entries))
	for _, entry := range inputSet.Entries {
		for _, key := range discoverPathKeys(workspaceRoot, entry.Path, entry.AbsPath) {
			result.Closure[key] = struct{}{}
		}
	}
	result.Command = buildCreateCommand(proposal.Ref, proposal.Class, proposal.Kind, proposal.CwdRel)
	return result, nil
}

func validationPathForAliasDir(absPath string, fallback string, aliasDir string) string {
	rel, err := filepath.Rel(aliasDir, absPath)
	if err != nil {
		return fallback
	}
	if canonicalRel, ok := stableDiscoverRelativePath(aliasDir, absPath, false); ok {
		rel = canonicalRel
	}
	rel = filepath.ToSlash(rel)
	if strings.TrimSpace(rel) == "" || rel == "." {
		return fallback
	}
	return rel
}
