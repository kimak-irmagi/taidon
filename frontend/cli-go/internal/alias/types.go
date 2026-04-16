package alias

import "strings"

const (
	prepareSuffix = ".prep.s9s.yaml"
	runSuffix     = ".run.s9s.yaml"
)

// Class identifies the alias-file schema described in
// docs/architecture/alias-inspection-component-structure.md.
type Class string

const (
	ClassPrepare Class = "prepare"
	ClassRun     Class = "run"
)

// Depth controls bounded scan breadth for alias inspection.
type Depth string

const (
	DepthSelf      Depth = "self"
	DepthChildren  Depth = "children"
	DepthRecursive Depth = "recursive"
)

// ScanOptions carries the bounded scan inputs owned by the CLI app layer.
type ScanOptions struct {
	WorkspaceRoot string
	CWD           string
	From          string
	Depth         string
	Classes       []Class
}

// Entry is one inventory row produced by alias scan mode.
type Entry struct {
	Class  Class  `json:"type"`
	Ref    string `json:"ref"`
	File   string `json:"file"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Path   string `json:"-"`
}

// ResolveOptions describes single-alias target resolution for check mode.
type ResolveOptions struct {
	WorkspaceRoot string
	CWD           string
	Ref           string
	Class         Class
}

// Target is one resolved alias file selected for static validation.
type Target struct {
	Class Class  `json:"type"`
	Ref   string `json:"ref"`
	File  string `json:"file"`
	Path  string `json:"-"`
}

// Definition is the canonical loaded alias model reused by execution and
// inspection after the CLI maintainability refactor.
type Definition struct {
	Class Class    `json:"type"`
	Kind  string   `json:"kind"`
	Image string   `json:"image,omitempty"`
	Args  []string `json:"args"`
}

// Issue is one static validation finding.
type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// CheckResult is the per-alias validation output consumed by app/cli rendering.
type CheckResult struct {
	Type   Class   `json:"type"`
	Ref    string  `json:"ref"`
	File   string  `json:"file"`
	Kind   string  `json:"kind,omitempty"`
	Valid  bool    `json:"valid"`
	Error  string  `json:"error,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
	Path   string  `json:"-"`
}

// CheckReport aggregates scan-mode or single-target check results.
type CheckReport struct {
	Checked      int           `json:"checked"`
	ValidCount   int           `json:"valid"`
	InvalidCount int           `json:"invalid"`
	Results      []CheckResult `json:"results"`
}

func normalizeClass(value Class) Class {
	switch Class(strings.ToLower(strings.TrimSpace(string(value)))) {
	case ClassPrepare:
		return ClassPrepare
	case ClassRun:
		return ClassRun
	default:
		return ""
	}
}

func normalizeClasses(values []Class) []Class {
	if len(values) == 0 {
		return []Class{ClassPrepare, ClassRun}
	}
	seen := map[Class]struct{}{}
	classes := make([]Class, 0, 2)
	for _, value := range values {
		class := normalizeClass(value)
		if class == "" {
			continue
		}
		if _, ok := seen[class]; ok {
			continue
		}
		seen[class] = struct{}{}
		classes = append(classes, class)
	}
	if len(classes) == 0 {
		return []Class{ClassPrepare, ClassRun}
	}
	return classes
}

func normalizeDepth(value string) Depth {
	switch Depth(strings.ToLower(strings.TrimSpace(value))) {
	case DepthSelf:
		return DepthSelf
	case DepthChildren:
		return DepthChildren
	case "", DepthRecursive:
		return DepthRecursive
	default:
		return ""
	}
}
