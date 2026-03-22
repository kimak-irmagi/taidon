// Package pgbench owns the shared pgbench file-bearing semantics described in
// docs/architecture/inputset-component-structure.md.
package pgbench

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
)

const stdinPath = "/dev/stdin"

// RebaseArgs applies alias/workspace path rebasing for pgbench file-bearing args.
func RebaseArgs(args []string, resolver inputset.Resolver) ([]string, error) {
	rebased := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			value, err := rebaseValue(args[i+1], resolver)
			if err != nil {
				return nil, err
			}
			rebased = append(rebased, arg, value)
			i++
		case strings.HasPrefix(arg, "--file="):
			value, err := rebaseValue(strings.TrimPrefix(arg, "--file="), resolver)
			if err != nil {
				return nil, err
			}
			rebased = append(rebased, "--file="+value)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value, err := rebaseValue(arg[2:], resolver)
			if err != nil {
				return nil, err
			}
			rebased = append(rebased, "-f"+value)
		default:
			rebased = append(rebased, arg)
		}
	}
	return rebased, nil
}

// MaterializeArgs builds the runtime pgbench args/stdin projection from shared semantics.
func MaterializeArgs(args []string, resolver inputset.Resolver, stdin io.Reader, fs inputset.FileSystem) ([]string, *string, error) {
	normalized := make([]string, 0, len(args))
	var source *fileSource

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, nil, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			value, nextSource, err := rewriteFileValue(args[i+1], resolver)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, arg, value)
			i++
		case strings.HasPrefix(arg, "--file="):
			value, nextSource, err := rewriteFileValue(strings.TrimPrefix(arg, "--file="), resolver)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, "--file="+value)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value, nextSource, err := rewriteFileValue(arg[2:], resolver)
			if err != nil {
				return nil, nil, err
			}
			if nextSource != nil {
				if source != nil {
					return nil, nil, inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported")
				}
				source = nextSource
			}
			normalized = append(normalized, "-f"+value)
		default:
			normalized = append(normalized, arg)
		}
	}

	if source == nil {
		return normalized, nil, nil
	}
	text, err := readSource(*source, stdin, fs)
	if err != nil {
		return nil, nil, err
	}
	return normalized, &text, nil
}

// ValidateArgs accumulates alias-check issues for the shared pgbench file syntax.
func ValidateArgs(args []string, resolver inputset.Resolver, fs inputset.FileSystem) []inputset.UserError {
	issues := make([]inputset.UserError, 0, 2)
	fileArgCount := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				issues = append(issues, *inputset.Errorf("missing_file_arg", "missing value for %s", arg))
				continue
			}
			fileArgCount++
			if fileArgCount > 1 {
				issues = append(issues, *inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported"))
			}
			if issue, ok := validateLocalFileArg(args[i+1], resolver, fs); ok {
				issues = append(issues, issue)
			}
			i++
		case strings.HasPrefix(arg, "--file="):
			fileArgCount++
			if fileArgCount > 1 {
				issues = append(issues, *inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported"))
			}
			if issue, ok := validateLocalFileArg(strings.TrimPrefix(arg, "--file="), resolver, fs); ok {
				issues = append(issues, issue)
			}
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			fileArgCount++
			if fileArgCount > 1 {
				issues = append(issues, *inputset.Errorf("multiple_file_args", "Multiple pgbench file arguments are not supported"))
			}
			if issue, ok := validateLocalFileArg(arg[2:], resolver, fs); ok {
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

type fileSource struct {
	Path      string
	UsesStdin bool
}

func rebaseValue(value string, resolver inputset.Resolver) (string, error) {
	path, weight := inputset.SplitPgbenchFileArgValue(value)
	if path == "-" || path == stdinPath {
		return stdinPath + weight, nil
	}
	resolved, err := resolver.ResolvePath(path)
	if err != nil {
		return "", err
	}
	return resolved + weight, nil
}

func rewriteFileValue(value string, resolver inputset.Resolver) (string, *fileSource, error) {
	path, weight := inputset.SplitPgbenchFileArgValue(value)
	if strings.TrimSpace(path) == "" {
		return "", nil, inputset.Errorf("missing_file_arg", "Missing value for --file")
	}
	if path == "-" || path == stdinPath {
		return stdinPath + weight, &fileSource{UsesStdin: true}, nil
	}
	resolved, err := resolver.ResolvePath(path)
	if err != nil {
		return "", nil, err
	}
	return stdinPath + weight, &fileSource{Path: resolved}, nil
}

func readSource(source fileSource, stdin io.Reader, fs inputset.FileSystem) (string, error) {
	if source.UsesStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := fs.ReadFile(source.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateLocalFileArg(value string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.UserError, bool) {
	path, _ := inputset.SplitPgbenchFileArgValue(value)
	cleaned := strings.TrimSpace(path)
	switch cleaned {
	case "":
		return *inputset.Errorf("empty_path", "file path is empty"), true
	case "-", stdinPath:
		return inputset.UserError{}, false
	}
	resolved, err := resolver.ResolvePath(cleaned)
	if err != nil {
		if issue, ok := err.(*inputset.UserError); ok {
			return *issue, true
		}
		return inputset.UserError{Code: "invalid_path", Message: err.Error()}, true
	}
	info, err := fs.Stat(resolved)
	if err != nil {
		return *inputset.Errorf("missing_path", "referenced path not found: %s", cleaned), true
	}
	if info.IsDir() {
		return *inputset.Errorf("expected_file", "referenced path must be a file: %s", cleaned), true
	}
	return inputset.UserError{}, false
}

// Collect builds the direct pgbench file set for future diff/provenance consumers.
func Collect(args []string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.InputSet, error) {
	var entries []inputset.InputEntry
	seen := make(map[string]struct{})
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var value string
		var ok bool
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return inputset.InputSet{}, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			value = args[i+1]
			i++
			ok = true
		case strings.HasPrefix(arg, "--file="):
			value = strings.TrimPrefix(arg, "--file=")
			ok = true
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value = arg[2:]
			ok = true
		}
		if !ok {
			continue
		}
		path, _ := inputset.SplitPgbenchFileArgValue(value)
		if path == "-" || path == stdinPath || strings.TrimSpace(path) == "" {
			continue
		}
		resolved, err := resolver.ResolvePath(path)
		if err != nil {
			return inputset.InputSet{}, err
		}
		if _, seenAlready := seen[resolved]; seenAlready {
			continue
		}
		seen[resolved] = struct{}{}
		content, err := fs.ReadFile(resolved)
		if err != nil {
			return inputset.InputSet{}, err
		}
		rel, _ := filepath.Rel(resolver.Root, resolved)
		entries = append(entries, inputset.InputEntry{
			Path:    filepath.ToSlash(rel),
			AbsPath: resolved,
			Hash:    inputset.HashContent(content),
		})
	}
	return inputset.InputSet{Entries: entries}, nil
}
