package diff

import (
	"errors"
	"fmt"
	"strings"
)

// ParsedDiff holds the result of parsing "diff" arguments.
type ParsedDiff struct {
	Scope          Scope
	WrappedName    string
	WrappedArgs    []string
	Limit          int
	IncludeContent bool
}

// ParseDiffScope parses args after "diff": scope (path or ref), optional --limit,
// --include-content, --ref-mode, --ref-keep-worktree, then wrapped command.
func ParseDiffScope(args []string) (ParsedDiff, error) {
	var fromPath, toPath, fromRef, toRef *string
	refMode := ""
	refKeep := false
	opts := Options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--from-path":
			if i+1 >= len(args) || strings.HasPrefix(strings.TrimSpace(args[i+1]), "-") {
				return ParsedDiff{}, errors.New("missing value for --from-path")
			}
			v := strings.TrimSpace(args[i+1])
			if v == "" {
				return ParsedDiff{}, errors.New("--from-path value is empty")
			}
			fromPath = &v
			i += 2
		case arg == "--to-path":
			if i+1 >= len(args) || strings.HasPrefix(strings.TrimSpace(args[i+1]), "-") {
				return ParsedDiff{}, errors.New("missing value for --to-path")
			}
			v := strings.TrimSpace(args[i+1])
			if v == "" {
				return ParsedDiff{}, errors.New("--to-path value is empty")
			}
			toPath = &v
			i += 2
		case arg == "--from-ref":
			if i+1 >= len(args) || strings.HasPrefix(strings.TrimSpace(args[i+1]), "-") {
				return ParsedDiff{}, errors.New("missing value for --from-ref")
			}
			v := strings.TrimSpace(args[i+1])
			if v == "" {
				return ParsedDiff{}, errors.New("--from-ref value is empty")
			}
			fromRef = &v
			i += 2
		case arg == "--to-ref":
			if i+1 >= len(args) || strings.HasPrefix(strings.TrimSpace(args[i+1]), "-") {
				return ParsedDiff{}, errors.New("missing value for --to-ref")
			}
			v := strings.TrimSpace(args[i+1])
			if v == "" {
				return ParsedDiff{}, errors.New("--to-ref value is empty")
			}
			toRef = &v
			i += 2
		case arg == "--ref-mode":
			if i+1 >= len(args) {
				return ParsedDiff{}, errors.New("missing value for --ref-mode")
			}
			refMode = strings.TrimSpace(strings.ToLower(args[i+1]))
			i += 2
		case arg == "--ref-keep-worktree":
			refKeep = true
			i++
		case arg == "--limit":
			if i+1 >= len(args) {
				return ParsedDiff{}, errors.New("missing value for --limit")
			}
			n, err := parseInt(args[i+1])
			if err != nil || n < 0 {
				return ParsedDiff{}, errors.New("--limit must be a non-negative integer")
			}
			opts.Limit = n
			i += 2
		case arg == "--include-content":
			opts.IncludeContent = true
			i++
		default:
			return finishParseDiffScope(fromPath, toPath, fromRef, toRef, refMode, refKeep, opts, args[i:])
		}
	}
	return ParsedDiff{}, errors.New("diff requires a wrapped command (e.g. plan:psql or prepare:lb)")
}

func finishParseDiffScope(
	fromPath, toPath, fromRef, toRef *string,
	refMode string,
	refKeep bool,
	opts Options,
	rest []string,
) (ParsedDiff, error) {
	hasPath := fromPath != nil || toPath != nil
	hasRef := fromRef != nil || toRef != nil
	if hasPath && hasRef {
		return ParsedDiff{}, errors.New("diff: do not mix --from-path/--to-path with --from-ref/--to-ref")
	}
	if len(rest) == 0 {
		return ParsedDiff{}, errors.New("diff requires a wrapped command (e.g. plan:psql or prepare:lb)")
	}

	if fromPath != nil && toPath != nil {
		return ParsedDiff{
			Scope: Scope{
				Kind:     ScopeKindPath,
				FromPath: *fromPath,
				ToPath:   *toPath,
			},
			WrappedName:    rest[0],
			WrappedArgs:    rest[1:],
			Limit:          opts.Limit,
			IncludeContent: opts.IncludeContent,
		}, nil
	}

	if fromRef != nil && toRef != nil {
		rm := refMode
		if rm == "" {
			rm = "blob"
		}
<<<<<<< Updated upstream
		if rm != "blob" && rm != "worktree" {
			return ParsedDiff{}, fmt.Errorf("diff: --ref-mode %q is not supported (use blob or worktree)", rm)
=======
		if rm != "worktree" && rm != "blob" {
			return ParsedDiff{}, fmt.Errorf("diff: --ref-mode %q is not supported (use blob or worktree)", rm)
		}
		if refKeep && rm != "worktree" {
			return ParsedDiff{}, errors.New("diff: --ref-keep-worktree is only valid with --ref-mode worktree")
>>>>>>> Stashed changes
		}
		return ParsedDiff{
			Scope: Scope{
				Kind:            ScopeKindRef,
				FromRef:         *fromRef,
				ToRef:           *toRef,
				RefMode:         rm,
				RefKeepWorktree: refKeep,
			},
			WrappedName:    rest[0],
			WrappedArgs:    rest[1:],
			Limit:          opts.Limit,
			IncludeContent: opts.IncludeContent,
		}, nil
	}

	if hasPath || hasRef {
		return ParsedDiff{}, errors.New("diff requires both --from-path and --to-path, or both --from-ref and --to-ref")
	}
	return ParsedDiff{}, errors.New("diff requires a scope: --from-path/--to-path or --from-ref/--to-ref")
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}
