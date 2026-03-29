package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
	"gopkg.in/yaml.v3"
)

// Options carries the workspace-bounded inputs for the advisory discover slice.
type Options struct {
	WorkspaceRoot string
	CWD           string
	Progress      Progress
}

const discoverScanHeartbeat = 64

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
	AbsPath       string
	WorkspaceRoot string
	WorkspaceRel  string
	CwdRel        string
	Ext           string
	LowerPath     string
	LowerBase     string
	Content       string
	BinaryOnly    bool
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
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return Report{}, err
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd = workspaceRoot
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return Report{}, err
	}
	cwd = filepath.Clean(cwd)

	emitProgress(opts.Progress, ProgressEvent{Stage: ProgressStageScanStart})

	coverage, err := loadAliasCoverage(workspaceRoot)
	if err != nil {
		return Report{}, err
	}

	files, scanned, err := walkDiscoverFiles(workspaceRoot, cwd, opts.Progress)
	if err != nil {
		return Report{}, err
	}

	report := Report{Scanned: scanned}
	proposals := make([]candidateProposal, 0, len(files))
	for _, file := range files {
		proposal, ok := proposeCandidate(file)
		if !ok {
			continue
		}
		proposals = append(proposals, proposal)
	}
	report.Prefiltered = len(proposals)
	emitProgress(opts.Progress, ProgressEvent{
		Stage:       ProgressStagePrefilterDone,
		Scanned:     report.Scanned,
		Prefiltered: report.Prefiltered,
	})

	validated := make([]validatedCandidate, 0, len(proposals))
	for _, proposal := range proposals {
		emitProgress(opts.Progress, ProgressEvent{
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
			return Report{}, err
		}
		emitProgress(opts.Progress, ProgressEvent{
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
	report.Validated = len(validated)

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

	inbound := inboundEdges(validated)
	findings := make([]Finding, 0, len(validated))
	seenAliasPaths := make(map[string]struct{}, len(validated))
	for _, candidate := range validated {
		if inbound[candidate.AbsPath] > 0 {
			report.Suppressed++
			emitProgress(opts.Progress, ProgressEvent{
				Stage:  ProgressStageSuppressed,
				Class:  candidate.Class,
				Kind:   candidate.Kind,
				Ref:    candidate.Ref,
				File:   candidate.WorkspaceRel,
				Score:  candidate.Score,
				Reason: "covered by inbound dependency",
			})
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
		if _, ok := seenAliasPaths[target.File]; ok {
			report.Suppressed++
			emitProgress(opts.Progress, ProgressEvent{
				Stage:  ProgressStageSuppressed,
				Class:  candidate.Class,
				Kind:   candidate.Kind,
				Ref:    candidate.Ref,
				File:   candidate.WorkspaceRel,
				Score:  candidate.Score,
				Reason: "duplicate alias path",
			})
			continue
		}
		seenAliasPaths[target.File] = struct{}{}
		if _, ok := coverage[target.File]; ok {
			report.Suppressed++
			emitProgress(opts.Progress, ProgressEvent{
				Stage:  ProgressStageSuppressed,
				Class:  candidate.Class,
				Kind:   candidate.Kind,
				Ref:    candidate.Ref,
				File:   candidate.WorkspaceRel,
				Score:  candidate.Score,
				Reason: "covered by existing alias",
			})
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
	emitProgress(opts.Progress, ProgressEvent{
		Stage:       ProgressStageSummary,
		Scanned:     report.Scanned,
		Prefiltered: report.Prefiltered,
		Validated:   report.Validated,
		Suppressed:  report.Suppressed,
		Findings:    len(report.Findings),
	})
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
		covered, err := aliasCoveragePaths(workspaceRoot, entry)
		if err != nil {
			continue
		}
		for path := range covered {
			coverage[path] = struct{}{}
		}
	}
	return coverage, nil
}

type discoverAliasDefinition struct {
	Kind string   `yaml:"kind"`
	Args []string `yaml:"args"`
}

func aliasCoveragePaths(workspaceRoot string, entry alias.Entry) (map[string]struct{}, error) {
	if strings.TrimSpace(entry.Path) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, err
	}
	var def discoverAliasDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}

	def.Kind = strings.ToLower(strings.TrimSpace(def.Kind))
	if len(def.Args) == 0 {
		return nil, nil
	}

	resolver := inputset.NewAliasResolver(workspaceRoot, entry.Path)
	var inputSet inputset.InputSet
	switch entry.Class {
	case alias.ClassPrepare:
		switch def.Kind {
		case "psql":
			inputSet, err = inputpsql.Collect(def.Args, resolver, inputset.OSFileSystem{})
		case "lb":
			inputSet, err = inputliquibase.Collect(def.Args, resolver, inputset.OSFileSystem{})
		default:
			return nil, nil
		}
	case alias.ClassRun:
		switch def.Kind {
		case "psql":
			inputSet, err = inputpsql.Collect(def.Args, resolver, inputset.OSFileSystem{})
		case "pgbench":
			inputSet, err = inputpgbench.Collect(def.Args, resolver, inputset.OSFileSystem{})
		default:
			return nil, nil
		}
	default:
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}

	covered := make(map[string]struct{}, len(inputSet.Entries))
	for _, entry := range inputSet.Entries {
		covered[entry.AbsPath] = struct{}{}
	}
	return covered, nil
}

func walkDiscoverFiles(workspaceRoot string, cwd string, progress Progress) ([]fileRecord, int, error) {
	records := make([]fileRecord, 0, 32)
	scanned := 0
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
		scanned++
		if progress != nil && scanned%discoverScanHeartbeat == 0 {
			emitProgress(progress, ProgressEvent{
				Stage:   ProgressStageScanProgress,
				Scanned: scanned,
			})
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
		return nil, 0, err
	}
	if progress != nil && scanned > 0 {
		emitProgress(progress, ProgressEvent{
			Stage:   ProgressStageScanProgress,
			Scanned: scanned,
		})
	}
	return records, scanned, nil
}

func classifyDiscoverFile(workspaceRoot string, cwd string, path string) (fileRecord, bool) {
	relWorkspace, ok := stableDiscoverRelativePath(workspaceRoot, path, false)
	if !ok {
		return fileRecord{}, false
	}
	relCWD, ok := stableDiscoverRelativePath(cwd, path, true)
	if !ok {
		// Keep the full path when a cwd-relative path cannot be formed
		// (for example, when cwd and the workspace live on different drives).
		relCWD = filepath.ToSlash(path)
	}

	lowerPath := strings.ToLower(filepath.ToSlash(relWorkspace))
	lowerBase := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if !isSupportedDiscoverExtension(ext) {
		return fileRecord{}, false
	}
	binaryOnly := ext == ".class" || ext == ".jar"

	record := fileRecord{
		AbsPath:       path,
		WorkspaceRoot: workspaceRoot,
		WorkspaceRel:  filepath.ToSlash(relWorkspace),
		CwdRel:        filepath.ToSlash(relCWD),
		Ext:           ext,
		LowerPath:     lowerPath,
		LowerBase:     lowerBase,
		BinaryOnly:    binaryOnly,
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

// stableDiscoverRelativePath keeps discover output stable when the same
// workspace is reachable through different symlinked path forms.
func stableDiscoverRelativePath(base string, target string, fallbackAbsolute bool) (string, bool) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		if fallbackAbsolute {
			return filepath.ToSlash(target), true
		}
		return "", false
	}

	canonicalBase := inputset.CanonicalizeBoundaryPath(base)
	canonicalTarget := inputset.CanonicalizeBoundaryPath(target)
	if canonicalBase != "" && canonicalTarget != "" {
		if canonicalRel, canonicalErr := filepath.Rel(canonicalBase, canonicalTarget); canonicalErr == nil {
			rawRel := filepath.ToSlash(strings.TrimSpace(rel))
			canonicalRel = filepath.ToSlash(strings.TrimSpace(canonicalRel))
			if rawRel != "" && rawRel != "." && strings.HasPrefix(rawRel, "..") && !strings.HasPrefix(canonicalRel, "..") {
				return canonicalRel, true
			}
		}
	}
	return filepath.ToSlash(rel), true
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
		result.Closure[entry.AbsPath] = struct{}{}
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
		return shellJoin(liquibaseDiscoverCreateCommand(ref, fileRef))
	case class == alias.ClassRun && kind == "psql":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:psql", "--", "-f", fileRef})
	case class == alias.ClassRun && kind == "pgbench":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:pgbench", "--", "-f", fileRef})
	default:
		return ""
	}
}

func shellJoin(args []string) string {
	return shellJoinForGoOS(runtime.GOOS, args)
}

func shellJoinForGoOS(goos string, args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuoteForGoOS(goos, arg))
	}
	return strings.Join(parts, " ")
}

func shellQuoteForGoOS(goos string, value string) string {
	if value == "" {
		return "''"
	}
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		if isPowerShellBareWord(value) {
			return value
		}
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	default:
		if !strings.ContainsAny(value, " \t\n\r'\"$&|<>;()[]{}*?!`\\") {
			return value
		}
		return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
	}
}

func isPowerShellBareWord(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '/' || r == ':' || r == '-':
		default:
			return false
		}
	}
	return true
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
		if stem == "" {
			return dir
		}
		return filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), stem))
	}
	return dir
}

func pathStem(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	if base == "" {
		return ""
	}
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
	if len(parts) == 0 {
		return false
	}
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

func isSupportedDiscoverExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".sql", ".xml", ".yaml", ".yml", ".json", ".class", ".jar":
		return true
	default:
		return false
	}
}

func isLiquibaseCandidateExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".xml", ".yaml", ".yml", ".json", ".sql", ".class", ".jar":
		return true
	default:
		return false
	}
}
