package prepare

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type psqlInput struct {
	kind  string
	value string
}

type psqlContentDigest struct {
	hash      string
	filePaths []string
}

func computePsqlContentDigest(inputs []psqlInput, workDir string) (psqlContentDigest, error) {
	locker := &contentLock{files: map[string]*os.File{}}
	defer locker.Close()
	return computePsqlContentDigestWithLock(inputs, workDir, locker)
}

func computePsqlContentDigestWithLock(inputs []psqlInput, workDir string, locker *contentLock) (psqlContentDigest, error) {
	builder := &strings.Builder{}
	tracker := &psqlContentTracker{
		workDir: workDir,
		locker:  locker,
		seen:    map[string]struct{}{},
		stack:   map[string]struct{}{},
	}
	for idx, input := range inputs {
		if idx > 0 {
			builder.WriteString("\n-- sqlrs: input-boundary\n")
		}
		switch input.kind {
		case "command", "stdin":
			if err := tracker.expandContent(input.value, "", builder); err != nil {
				return psqlContentDigest{}, err
			}
		case "file":
			if err := tracker.expandFile(input.value, builder); err != nil {
				return psqlContentDigest{}, err
			}
		default:
			return psqlContentDigest{}, fmt.Errorf("unsupported input kind: %s", input.kind)
		}
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return psqlContentDigest{
		hash:      hex.EncodeToString(sum[:]),
		filePaths: tracker.lockedFiles(),
	}, nil
}

type psqlContentTracker struct {
	workDir string
	locker  *contentLock
	seen    map[string]struct{}
	stack   map[string]struct{}
}

func (t *psqlContentTracker) lockedFiles() []string {
	paths := make([]string, 0, len(t.locker.files))
	for path := range t.locker.files {
		paths = append(paths, path)
	}
	return paths
}

func (t *psqlContentTracker) expandFile(path string, builder *strings.Builder) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty include path")
	}
	path = filepath.Clean(path)
	if _, ok := t.stack[path]; ok {
		return fmt.Errorf("recursive include: %s", path)
	}
	if err := t.lockFile(path); err != nil {
		return err
	}
	t.stack[path] = struct{}{}
	defer delete(t.stack, path)

	data, err := t.locker.readFile(path)
	if err != nil {
		return err
	}
	return t.expandContent(string(data), path, builder)
}

func (t *psqlContentTracker) expandContent(content string, currentFile string, builder *strings.Builder) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if cmd, arg, ok := parsePsqlInclude(line); ok {
			includePath, err := t.resolveIncludePath(cmd, arg, currentFile)
			if err != nil {
				return err
			}
			builder.WriteString("\n-- sqlrs: include-boundary\n")
			if err := t.expandFile(includePath, builder); err != nil {
				return err
			}
			builder.WriteString("\n-- sqlrs: include-boundary\n")
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return scanner.Err()
}

func (t *psqlContentTracker) resolveIncludePath(cmd string, arg string, currentFile string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("include path is empty")
	}
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg), nil
	}
	base := t.workDir
	switch cmd {
	case "\\ir", "\\include_relative":
		if currentFile != "" {
			base = filepath.Dir(currentFile)
		}
	}
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("include path requires working directory")
	}
	return filepath.Clean(filepath.Join(base, arg)), nil
}

func (t *psqlContentTracker) lockFile(path string) error {
	if _, ok := t.seen[path]; ok {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	f, err := lockContentFile(path)
	if err != nil {
		return err
	}
	if t.locker.files == nil {
		t.locker.files = map[string]*os.File{}
	}
	t.locker.files[path] = f
	t.locker.order = append(t.locker.order, path)
	t.seen[path] = struct{}{}
	return nil
}

func parsePsqlInclude(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, `\`) {
		return "", "", false
	}
	parts := splitPsqlCommand(trimmed)
	if len(parts) < 2 {
		return "", "", false
	}
	cmd := parts[0]
	switch cmd {
	case `\i`, `\ir`, `\include`, `\include_relative`:
		return cmd, parts[1], true
	default:
		return "", "", false
	}
}

func splitPsqlCommand(line string) []string {
	out := []string{}
	buf := &strings.Builder{}
	inQuote := rune(0)
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
