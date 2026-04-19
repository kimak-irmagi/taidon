package discover

import "github.com/sqlrs/cli/internal/alias"

// Options carries the workspace-bounded inputs for the advisory discover
// analyzers described in docs/architecture/discover-component-structure.md.
type Options struct {
	WorkspaceRoot     string
	CWD               string
	SelectedAnalyzers []string
	ShellFamily       string
	Progress          Progress
}

// Finding is one advisory suggestion or validation note emitted by a discover
// analyzer.
type Finding struct {
	Analyzer         string           `json:"analyzer,omitempty"`
	Target           string           `json:"target,omitempty"`
	Action           string           `json:"action,omitempty"`
	FollowUpCommand  *FollowUpCommand `json:"follow_up_command,omitempty"`
	SuggestedEntries []string         `json:"suggested_entries,omitempty"`
	JSONPayload      string           `json:"json_payload,omitempty"`
	Type             alias.Class      `json:"type"`
	Kind             string           `json:"kind"`
	Ref              string           `json:"ref"`
	File             string           `json:"file"`
	AliasPath        string           `json:"alias_path"`
	Reason           string           `json:"reason"`
	CreateCommand    string           `json:"create_command"`
	Score            int              `json:"score"`
	Valid            bool             `json:"valid"`
	Error            string           `json:"error,omitempty"`
}

// Report aggregates the advisory findings emitted by the selected discover
// analyzers.
type Report struct {
	SelectedAnalyzers []string          `json:"selected_analyzers,omitempty"`
	Summaries         []AnalyzerSummary `json:"summaries,omitempty"`
	Scanned           int               `json:"scanned"`
	Prefiltered       int               `json:"prefiltered"`
	Validated         int               `json:"validated"`
	Suppressed        int               `json:"suppressed"`
	Findings          []Finding         `json:"findings"`
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
