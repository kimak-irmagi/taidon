package diff

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ParsedDiff holds the result of parsing "diff" arguments.
type ParsedDiff struct {
	Scope         PathScope
	WrappedName   string
	WrappedArgs   []string
	Limit         int
	IncludeContent bool
}

// ParsePathScope parses args after "diff" and splits them into path scope,
// optional --limit/--include-content, and wrapped command.
// Returns ParsedDiff or error.
func ParsePathScope(args []string) (ParsedDiff, error) {
	var fromPath, toPath *string
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
			if fromPath == nil || toPath == nil {
				return ParsedDiff{}, errors.New("diff requires --from-path and --to-path")
			}
			if i >= len(args) {
				return ParsedDiff{}, errors.New("diff requires a wrapped command (e.g. plan:psql or prepare:lb)")
			}
			return ParsedDiff{
				Scope:          PathScope{FromPath: *fromPath, ToPath: *toPath},
				WrappedName:    args[i],
				WrappedArgs:    args[i+1:],
				Limit:         opts.Limit,
				IncludeContent: opts.IncludeContent,
			}, nil
		}
	}
	if fromPath == nil || toPath == nil {
		return ParsedDiff{}, errors.New("diff requires --from-path and --to-path")
	}
	return ParsedDiff{}, errors.New("diff requires a wrapped command (e.g. plan:psql or prepare:lb)")
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

// ResolvePathScope resolves relative paths to absolute using cwd.
func ResolvePathScope(scope PathScope, cwd string) (fromCtx, toCtx Context, err error) {
	fromAbs, err := filepath.Abs(scope.FromPath)
	if err != nil {
		return Context{}, Context{}, fmt.Errorf("from-path: %w", err)
	}
	toAbs, err := filepath.Abs(scope.ToPath)
	if err != nil {
		return Context{}, Context{}, fmt.Errorf("to-path: %w", err)
	}
	return Context{Root: fromAbs}, Context{Root: toAbs}, nil
}
