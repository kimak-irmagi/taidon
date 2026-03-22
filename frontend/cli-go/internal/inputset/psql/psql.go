// Package psql owns the shared psql file-bearing semantics described in
// docs/architecture/inputset-component-structure.md.
package psql

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
)

// NormalizeArgs applies shared host-path normalization for `psql` file-bearing args.
func NormalizeArgs(args []string, resolver inputset.Resolver, stdin io.Reader) ([]string, *string, error) {
	normalized := make([]string, 0, len(args))
	usesStdin := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, nil, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			value, useStdin, err := normalizeFileArg(args[i+1], resolver)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, arg, value)
			i++
		case strings.HasPrefix(arg, "--file="):
			value, useStdin, err := normalizeFileArg(strings.TrimPrefix(arg, "--file="), resolver)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, "--file="+value)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value, useStdin, err := normalizeFileArg(arg[2:], resolver)
			if err != nil {
				return nil, nil, err
			}
			usesStdin = usesStdin || useStdin
			normalized = append(normalized, "-f"+value)
		default:
			normalized = append(normalized, arg)
		}
	}

	if !usesStdin {
		return normalized, nil, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)
	return normalized, &text, nil
}

// BuildRunSteps materializes the run-facing `psql` step projection from shared semantics.
func BuildRunSteps(args []string, resolver inputset.Resolver, stdin io.Reader, fs inputset.FileSystem) ([]inputset.RunStep, error) {
	var shared []string
	var steps []inputset.RunStep
	stdinStep := -1

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "--command":
			if i+1 >= len(args) {
				return nil, inputset.Errorf("missing_value", "Missing value for %s", arg)
			}
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", args[i+1])
			steps = append(steps, inputset.RunStep{Args: stepArgs})
			i++
		case strings.HasPrefix(arg, "--command="):
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", strings.TrimPrefix(arg, "--command="))
			steps = append(steps, inputset.RunStep{Args: stepArgs})
		case strings.HasPrefix(arg, "-c") && len(arg) > 2:
			stepArgs := append([]string{}, shared...)
			stepArgs = append(stepArgs, "-c", arg[2:])
			steps = append(steps, inputset.RunStep{Args: stepArgs})
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			step, useStdin, err := buildFileStep(shared, args[i+1], resolver, fs)
			if err != nil {
				return nil, err
			}
			if useStdin {
				if stdinStep != -1 {
					return nil, inputset.Errorf("multiple_file_args", "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
			i++
		case strings.HasPrefix(arg, "--file="):
			step, useStdin, err := buildFileStep(shared, strings.TrimPrefix(arg, "--file="), resolver, fs)
			if err != nil {
				return nil, err
			}
			if useStdin {
				if stdinStep != -1 {
					return nil, inputset.Errorf("multiple_file_args", "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			step, useStdin, err := buildFileStep(shared, arg[2:], resolver, fs)
			if err != nil {
				return nil, err
			}
			if useStdin {
				if stdinStep != -1 {
					return nil, inputset.Errorf("multiple_file_args", "Multiple stdin file arguments are not supported")
				}
				stdinStep = len(steps)
			}
			steps = append(steps, step)
		default:
			shared = append(shared, arg)
		}
	}

	if stdinStep != -1 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		text := string(data)
		steps[stdinStep].Stdin = &text
	}

	if len(steps) == 0 {
		return []inputset.RunStep{{Args: shared}}, nil
	}
	return steps, nil
}

// Collect builds the deterministic psql file closure used by diff-facing consumers.
func Collect(args []string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.InputSet, error) {
	entryPaths, err := collectEntryPaths(args, resolver)
	if err != nil {
		return inputset.InputSet{}, err
	}
	if len(entryPaths) == 0 {
		return inputset.InputSet{}, fmt.Errorf("psql command has no -f file (required for diff)")
	}

	tracker := &tracker{
		root:  resolver.Root,
		seen:  make(map[string]struct{}),
		stack: make(map[string]struct{}),
		fs:    fs,
	}
	var order []string
	for _, path := range entryPaths {
		if err := tracker.collect(path, &order); err != nil {
			return inputset.InputSet{}, err
		}
	}

	entries := make([]inputset.InputEntry, 0, len(order))
	for _, path := range order {
		content, err := fs.ReadFile(path)
		if err != nil {
			return inputset.InputSet{}, fmt.Errorf("read %s: %w", path, err)
		}
		rel, _ := filepath.Rel(resolver.Root, path)
		entries = append(entries, inputset.InputEntry{
			Path:    filepath.ToSlash(rel),
			AbsPath: path,
			Hash:    inputset.HashContent(content),
		})
	}
	return inputset.InputSet{Entries: entries}, nil
}

// ValidateArgs accumulates alias-check issues for the same psql file-bearing syntax.
func ValidateArgs(args []string, resolver inputset.Resolver, fs inputset.FileSystem) []inputset.UserError {
	issues := make([]inputset.UserError, 0, 2)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				issues = append(issues, *inputset.Errorf("missing_file_arg", "missing value for %s", arg))
				continue
			}
			if issue, ok := validateLocalFileArg(args[i+1], resolver, fs); ok {
				issues = append(issues, issue)
			}
			i++
		case strings.HasPrefix(arg, "--file="):
			if issue, ok := validateLocalFileArg(strings.TrimPrefix(arg, "--file="), resolver, fs); ok {
				issues = append(issues, issue)
			}
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			if issue, ok := validateLocalFileArg(arg[2:], resolver, fs); ok {
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

func normalizeFileArg(value string, resolver inputset.Resolver) (string, bool, error) {
	if value == "-" {
		return value, true, nil
	}
	path, err := resolver.ResolvePath(value)
	if err != nil {
		return "", false, err
	}
	return path, false, nil
}

func buildFileStep(shared []string, value string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.RunStep, bool, error) {
	if strings.TrimSpace(value) == "" {
		return inputset.RunStep{}, false, inputset.Errorf("missing_file_arg", "Missing value for --file")
	}
	if value == "-" {
		stepArgs := append([]string{}, shared...)
		stepArgs = append(stepArgs, "-f", "-")
		return inputset.RunStep{Args: stepArgs}, true, nil
	}
	path, err := resolver.ResolvePath(value)
	if err != nil {
		return inputset.RunStep{}, false, err
	}
	data, err := fs.ReadFile(path)
	if err != nil {
		return inputset.RunStep{}, false, err
	}
	text := string(data)
	stepArgs := append([]string{}, shared...)
	stepArgs = append(stepArgs, "-f", "-")
	return inputset.RunStep{Args: stepArgs, Stdin: &text}, false, nil
}

func collectEntryPaths(args []string, resolver inputset.Resolver) ([]string, error) {
	out := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			continue
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, inputset.Errorf("missing_file_arg", "Missing value for %s", arg)
			}
			value := strings.TrimSpace(args[i+1])
			if value != "-" && value != "" {
				path, err := resolver.ResolvePath(value)
				if err != nil {
					return nil, err
				}
				out = append(out, path)
			}
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--file="))
			if value != "-" && value != "" {
				path, err := resolver.ResolvePath(value)
				if err != nil {
					return nil, err
				}
				out = append(out, path)
			}
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := strings.TrimSpace(arg[2:])
			if value != "-" && value != "" {
				path, err := resolver.ResolvePath(value)
				if err != nil {
					return nil, err
				}
				out = append(out, path)
			}
		}
	}
	return out, nil
}

type tracker struct {
	root  string
	seen  map[string]struct{}
	stack map[string]struct{}
	fs    inputset.FileSystem
}

func (t *tracker) collect(path string, order *[]string) error {
	path = filepath.Clean(path)
	if _, ok := t.stack[path]; ok {
		return fmt.Errorf("recursive include: %s", path)
	}
	if _, ok := t.seen[path]; ok {
		return nil
	}
	if _, err := t.fs.Stat(path); err != nil {
		return err
	}
	t.seen[path] = struct{}{}
	t.stack[path] = struct{}{}
	defer delete(t.stack, path)

	content, err := t.fs.ReadFile(path)
	if err != nil {
		return err
	}
	*order = append(*order, path)

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		cmd, arg, ok := parseInclude(scanner.Text())
		if !ok {
			continue
		}
		next := t.resolveInclude(cmd, arg, path)
		if err := t.collect(next, order); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (t *tracker) resolveInclude(cmd string, arg string, currentFile string) string {
	base := t.root
	if cmd == `\ir` || cmd == `\include_relative` {
		base = filepath.Dir(currentFile)
	}
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg)
	}
	return filepath.Clean(filepath.Join(base, arg))
}

func parseInclude(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, `\`) {
		return "", "", false
	}
	parts := splitCommand(trimmed)
	if len(parts) < 2 {
		return "", "", false
	}
	switch parts[0] {
	case `\i`, `\ir`, `\include`, `\include_relative`:
		return parts[0], strings.TrimSpace(parts[1]), true
	default:
		return "", "", false
	}
}

func splitCommand(line string) []string {
	var out []string
	var buf strings.Builder
	var quote rune
	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			buf.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t':
			if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

func validateLocalFileArg(value string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.UserError, bool) {
	cleaned := strings.TrimSpace(value)
	switch cleaned {
	case "":
		return *inputset.Errorf("empty_path", "file path is empty"), true
	case "-":
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
