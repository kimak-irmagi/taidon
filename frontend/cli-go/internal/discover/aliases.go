package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
)

// Options carries the workspace-bounded inputs for the advisory discover slice.
type Options struct {
	WorkspaceRoot string
	CWD           string
}

// Finding is one ranked advisory suggestion or validation note.
type Finding struct {
	Type          alias.Class `json:"type"`
	Kind          string      `json:"kind"`
	Ref           string      `json:"ref"`
	File          string      `json:"file"`
	AliasPath     string      `json:"alias_path"`
	Reason        string      `json:"reason"`
	CreateCommand string      `json:"create_command"`
	Score         int         `json:"score"`
	Valid         bool        `json:"valid"`
	Error         string      `json:"error,omitempty"`
}

// Report aggregates the ranked findings produced by the aliases analyzer.
type Report struct {
	Scanned     int       `json:"scanned"`
	Prefiltered int       `json:"prefiltered"`
	Validated   int       `json:"validated"`
	Suppressed  int       `json:"suppressed"`
	Findings    []Finding `json:"findings"`
}

type fileRecord struct {
	AbsPath      string
	WorkspaceRel string
	CwdRel       string
	Ext          string
	LowerPath    string
	LowerBase    string
	Content      string
	BinaryOnly   bool
}

type candidateProposal struct {
	fileRecord
	Class   alias.Class
	Kind    string
	Score   int
	Reason  string
	Ref     string
	Command string
}

type validatedCandidate struct {
	candidateProposal
	Valid   bool
	Error   string
	Closure map[string]struct{}
}

// AnalyzeAliases walks the workspace, scores likely workflow roots, validates
// supported candidates, and suppresses suggestions already covered by alias
// files on disk.
func AnalyzeAliases(opts Options) (Report, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Report{}, fmt.Errorf("workspace root is required for discover")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd = workspaceRoot
	}
	cwd = filepath.Clean(cwd)

	coverage, err := loadAliasCoverage(workspaceRoot)
	if err != nil {
		return Report{}, err
	}

	files, err := walkDiscoverFiles(workspaceRoot, cwd)
	if err != nil {
		return Report{}, err
	}

	report := Report{Scanned: len(files)}
	proposals := make([]candidateProposal, 0, len(files))
	for _, file := range files {
		proposal, ok := proposeCandidate(file)
		if !ok {
			continue
		}
		proposals = append(proposals, proposal)
	}
	report.Prefiltered = len(proposals)

	validated := make([]validatedCandidate, 0, len(proposals))
	for _, proposal := range proposals {
		result, err := validateCandidate(proposal, workspaceRoot, cwd)
		if err != nil {
			return Report{}, err
		}
		validated = append(validated, result)
	}
	report.Validated = len(validated)

	inbound := inboundEdges(validated)
	findings := make([]Finding, 0, len(validated))
	for _, candidate := range validated {
		if inbound[candidate.AbsPath] > 0 {
			report.Suppressed++
			continue
		}
		target, err := alias.ResolveCreateTarget(alias.CreateOptions{
			WorkspaceRoot: workspaceRoot,
			CWD:           cwd,
			Ref:           candidate.Ref,
			Class:         candidate.Class,
		})
		if err != nil {
			return Report{}, err
		}
		if _, ok := coverage[target.File]; ok {
			report.Suppressed++
			continue
		}

		createCommand := candidate.Command
		if strings.TrimSpace(createCommand) == "" {
			createCommand = buildCreateCommand(candidate.Ref, candidate.Class, candidate.Kind, candidate.CwdRel)
		}

		findings = append(findings, Finding{
			Type:          candidate.Class,
			Kind:          candidate.Kind,
			Ref:           candidate.Ref,
			File:          candidate.WorkspaceRel,
			AliasPath:     target.File,
			Reason:        candidate.Reason,
			CreateCommand: createCommand,
			Score:         candidate.Score,
			Valid:         candidate.Valid,
			Error:         candidate.Error,
		})
	}

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
	report.Findings = findings
	return report, nil
}

func loadAliasCoverage(workspaceRoot string) (map[string]struct{}, error) {
	entries, err := alias.Scan(alias.ScanOptions{
		WorkspaceRoot: workspaceRoot,
		CWD:           workspaceRoot,
		From:          "workspace",
		Depth:         "recursive",
	})
	if err != nil {
		return nil, err
	}
	coverage := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		coverage[entry.File] = struct{}{}
	}
	return coverage, nil
}

func walkDiscoverFiles(workspaceRoot string, cwd string) ([]fileRecord, error) {
	records := make([]fileRecord, 0, 32)
	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".sqlrs", ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		lowerName := strings.ToLower(name)
		if strings.HasSuffix(lowerName, ".prep.s9s.yaml") || strings.HasSuffix(lowerName, ".run.s9s.yaml") {
			return nil
		}
		record, ok := classifyDiscoverFile(workspaceRoot, cwd, path)
		if !ok {
			return nil
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func classifyDiscoverFile(workspaceRoot string, cwd string, path string) (fileRecord, bool) {
	relWorkspace, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return fileRecord{}, false
	}
	relCWD, err := filepath.Rel(cwd, path)
	if err != nil {
		relCWD = filepath.Base(path)
	}

	lowerPath := strings.ToLower(filepath.ToSlash(relWorkspace))
	lowerBase := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	binaryOnly := ext == ".class" || ext == ".jar"

	record := fileRecord{
		AbsPath:      path,
		WorkspaceRel: filepath.ToSlash(relWorkspace),
		CwdRel:       filepath.ToSlash(relCWD),
		Ext:          ext,
		LowerPath:    lowerPath,
		LowerBase:    lowerBase,
		BinaryOnly:   binaryOnly,
	}

	if !binaryOnly {
		content, err := readDiscoverSnippet(path)
		if err != nil {
			return fileRecord{}, false
		}
		record.Content = strings.ToLower(content)
	}

	return record, true
}

func readDiscoverSnippet(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return string(data), nil
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

func validateCandidate(proposal candidateProposal, workspaceRoot string, cwd string) (validatedCandidate, error) {
	result := validatedCandidate{
		candidateProposal: proposal,
		Closure:           map[string]struct{}{},
	}

	if proposal.BinaryOnly {
		result.Valid = false
		result.Error = "binary Liquibase artifacts are not yet supported"
		result.Command = buildCreateCommand(proposal.Ref, proposal.Class, proposal.Kind, proposal.CwdRel)
		return result, nil
	}

	resolver := inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil)
	var (
		inputSet inputset.InputSet
		err      error
	)
	switch {
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "psql":
		inputSet, err = inputpsql.Collect([]string{"-f", proposal.CwdRel}, resolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassPrepare && proposal.Kind == "lb":
		inputSet, err = inputliquibase.Collect([]string{"update", "--changelog-file", proposal.CwdRel}, resolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassRun && proposal.Kind == "psql":
		inputSet, err = inputpsql.Collect([]string{"-f", proposal.CwdRel}, resolver, inputset.OSFileSystem{})
	case proposal.Class == alias.ClassRun && proposal.Kind == "pgbench":
		inputSet, err = inputpgbench.Collect([]string{"-f", proposal.CwdRel}, resolver, inputset.OSFileSystem{})
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
		result.Closure[entry.AbsPath] = struct{}{}
	}
	result.Command = buildCreateCommand(proposal.Ref, proposal.Class, proposal.Kind, proposal.CwdRel)
	return result, nil
}

func inboundEdges(candidates []validatedCandidate) map[string]int {
	inbound := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		if len(candidate.Closure) == 0 {
			continue
		}
		for _, other := range candidates {
			if other.AbsPath == candidate.AbsPath {
				continue
			}
			if _, ok := other.Closure[candidate.AbsPath]; ok {
				inbound[candidate.AbsPath]++
			}
		}
	}
	return inbound
}

func buildCreateCommand(ref string, class alias.Class, kind string, fileRef string) string {
	switch {
	case class == alias.ClassPrepare && kind == "psql":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "prepare:psql", "--", "-f", fileRef})
	case class == alias.ClassPrepare && kind == "lb":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "prepare:lb", "--", "update", "--changelog-file", fileRef})
	case class == alias.ClassRun && kind == "psql":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:psql", "--", "-f", fileRef})
	case class == alias.ClassRun && kind == "pgbench":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:pgbench", "--", "-f", fileRef})
	default:
		return ""
	}
}

func shellJoin(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n\r'\"$&|<>;()[]{}*?!`\\") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
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
	if file.BinaryOnly {
		if points, reason := scoreContains(file.LowerPath, []string{"liquibase", "changelog", "change-log", "dbchangelog", "changeset", "master"}, 40, "Liquibase binary artifact path"); points > 0 {
			result.Score += points
			result.Reason = appendReason(result.Reason, reason)
		}
		result.Score += 5
		result.Reason = appendReason(result.Reason, "binary Liquibase artifact")
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
		return result
	}
	result.Score += 20
	result.Reason = appendReason(result.Reason, "markup changelog file")
	if points, reason := scoreContains(file.LowerPath, []string{"liquibase", "changelog", "change-log", "dbchangelog", "changeset", "master", "migration", "migrations"}, 40, "Liquibase changelog path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"databasechangelog", "changeset", "includeall"}, 30, "Liquibase changelog markup"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"include file=", "includeall path="}, 10, "Liquibase include graph"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score >= 35 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	if result.Ref == "" && result.Score > 0 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	return result
}

func scorePreparePsql(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassPrepare, Kind: "psql"}
	result.Score += 10
	result.Reason = appendReason(result.Reason, "SQL file")
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
	if result.Score >= 35 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	if result.Ref == "" && result.Score > 0 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	return result
}

func scoreRunPgbench(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassRun, Kind: "pgbench"}
	result.Score += 10
	result.Reason = appendReason(result.Reason, "SQL benchmark file")
	if points, reason := scoreContains(file.LowerPath, []string{"bench", "benchmark", "pgbench", "perf", "performance", "load", "stress", "tps"}, 40, "benchmark path"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if points, reason := scoreContains(file.Content, []string{"\\setrandom", "\\shell", "\\sleep", "pgbench"}, 30, "pgbench workload markers"); points > 0 {
		result.Score += points
		result.Reason = appendReason(result.Reason, reason)
	}
	if result.Score >= 35 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	if result.Ref == "" && result.Score > 0 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	return result
}

func scoreRunPsql(file fileRecord) candidateProposal {
	result := candidateProposal{fileRecord: file, Class: alias.ClassRun, Kind: "psql"}
	result.Score += 10
	result.Reason = appendReason(result.Reason, "SQL query file")
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
	if result.Score >= 35 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	if result.Ref == "" && result.Score > 0 {
		result.Ref = suggestedAliasRef(file.CwdRel, file.AbsPath)
	}
	return result
}

func suggestedAliasRef(cwdRel string, absPath string) string {
	base := filepath.Base(absPath)
	baseStem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(cwdRel)
	if dir == "." || dir == "" {
		if baseStem == "" {
			return strings.TrimSpace(base)
		}
		return filepath.ToSlash(baseStem)
	}
	return filepath.ToSlash(dir)
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
