package diff

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildPsqlFileList builds the ordered file list for plan:psql / prepare:psql:
// entry point from -f in args, closure over \i, \ir, \include, \include_relative.
// Root is the context root (directory). Args are the wrapped command args (after --).
func BuildPsqlFileList(ctx Context, args []string) (FileList, error) {
	entryPaths, err := extractPsqlFArgs(args)
	if err != nil {
		return FileList{}, err
	}
	if len(entryPaths) == 0 {
		return FileList{}, fmt.Errorf("psql command has no -f file (required for diff)")
	}
	tracker := &psqlTracker{root: ctx.Root, seen: make(map[string]struct{}), stack: make(map[string]struct{})}
	var order []string
	for _, rel := range entryPaths {
		abs := filepath.Join(ctx.Root, rel)
		if err := tracker.collect(abs, &order); err != nil {
			return FileList{}, err
		}
	}
	entries := make([]FileEntry, 0, len(order))
	for _, p := range order {
		content, err := os.ReadFile(p)
		if err != nil {
			return FileList{}, fmt.Errorf("read %s: %w", p, err)
		}
		rel, _ := filepath.Rel(ctx.Root, p)
		entries = append(entries, FileEntry{Path: filepath.ToSlash(rel), Hash: HashContent(content)})
	}
	return FileList{Entries: entries}, nil
}

func extractPsqlFArgs(args []string) ([]string, error) {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			continue
		}
		if arg == "-f" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for -f")
			}
			v := strings.TrimSpace(args[i+1])
			if v != "-" && v != "" {
				out = append(out, v)
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "-f=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "-f="))
			if v != "-" && v != "" {
				out = append(out, v)
			}
		}
	}
	return out, nil
}

type psqlTracker struct {
	root  string
	seen  map[string]struct{}
	stack map[string]struct{}
}

func (t *psqlTracker) collect(absPath string, order *[]string) error {
	absPath = filepath.Clean(absPath)
	if _, ok := t.stack[absPath]; ok {
		return fmt.Errorf("recursive include: %s", absPath)
	}
	if _, ok := t.seen[absPath]; ok {
		return nil
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}
	t.seen[absPath] = struct{}{}
	t.stack[absPath] = struct{}{}
	defer delete(t.stack, absPath)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	*order = append(*order, absPath)

	// Expand includes
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		cmd, arg, ok := parsePsqlInclude(line)
		if !ok {
			continue
		}
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		includePath, err := t.resolveInclude(cmd, arg, absPath)
		if err != nil {
			return err
		}
		if err := t.collect(includePath, order); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (t *psqlTracker) resolveInclude(cmd, arg, currentFile string) (string, error) {
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg), nil
	}
	base := t.root
	if cmd == `\ir` || cmd == `\include_relative` {
		if currentFile != "" {
			base = filepath.Dir(currentFile)
		}
	}
	return filepath.Clean(filepath.Join(base, arg)), nil
}

func parsePsqlInclude(line string) (cmd, arg string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, `\`) {
		return "", "", false
	}
	parts := splitPsqlCommand(trimmed)
	if len(parts) < 2 {
		return "", "", false
	}
	switch parts[0] {
	case `\i`, `\ir`, `\include`, `\include_relative`:
		return parts[0], parts[1], true
	}
	return "", "", false
}

func splitPsqlCommand(line string) []string {
	var out []string
	var buf strings.Builder
	var inQuote rune
	for _, r := range line {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
				continue
			}
			buf.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			inQuote = r
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
