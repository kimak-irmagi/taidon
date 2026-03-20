package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/diff"
)

// RunDiff runs the diff workflow: resolve scope, build file lists for both sides,
// compare, and render. kind is the wrapped command (e.g. plan:psql, prepare:lb).
// Returns an error for unsupported kind or build/compare errors.
func RunDiff(stdout io.Writer, parsed diff.ParsedDiff, cwd string, outputFormat string) error {
	scope := parsed.Scope
	opts := diff.Options{Limit: parsed.Limit, IncludeContent: parsed.IncludeContent}

	fromCtx, toCtx, err := diff.ResolvePathScope(scope, cwd)
	if err != nil {
		return err
	}

	kind := parseDiffKind(parsed.WrappedName)
	if kind == "" {
		return fmt.Errorf("diff only supports plan:psql, plan:lb, prepare:psql, prepare:lb (got %q)", parsed.WrappedName)
	}

	var fromList, toList diff.FileList
	if kind == "psql" {
		fromList, err = diff.BuildPsqlFileList(fromCtx, parsed.WrappedArgs)
		if err != nil {
			return fmt.Errorf("from-path: %w", err)
		}
		toList, err = diff.BuildPsqlFileList(toCtx, parsed.WrappedArgs)
		if err != nil {
			return fmt.Errorf("to-path: %w", err)
		}
	} else {
		fromList, err = diff.BuildLbFileList(fromCtx, parsed.WrappedArgs)
		if err != nil {
			return fmt.Errorf("from-path: %w", err)
		}
		toList, err = diff.BuildLbFileList(toCtx, parsed.WrappedArgs)
		if err != nil {
			return fmt.Errorf("to-path: %w", err)
		}
	}

	result := diff.Compare(fromList, toList, opts)
	if outputFormat == "json" {
		return diff.RenderJSON(stdout, result, scope, parsed.WrappedName, opts, fromCtx, toCtx)
	}
	diff.RenderHuman(stdout, result, scope, parsed.WrappedName, opts, fromCtx, toCtx)
	return nil
}

// parseDiffKind returns "psql" or "lb" for plan:psql, plan:lb, prepare:psql, prepare:lb; else "".
func parseDiffKind(name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "plan:psql", "prepare:psql":
		return "psql"
	case "plan:lb", "prepare:lb":
		return "lb"
	}
	return ""
}
