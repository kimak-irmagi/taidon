package prepare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sqlrs/engine/internal/runtime"
)

type LiquibaseChangeset struct {
	ID       string
	Author   string
	Path     string
	SQL      string
	SQLHash  string
	Checksum string
}

type liquibasePrepared struct {
	normalizedArgs []string
	argsNormalized string
	mounts         []runtime.Mount
	lockPaths      []string
	searchPaths    []string
	workDir        string
}

func prepareLiquibaseArgs(args []string, cwd string, windowsMode bool) (liquibasePrepared, error) {
	preArgs := make([]string, 0, len(args))
	postArgs := make([]string, 0, len(args))
	mounts := []runtime.Mount{}
	mountIndex := 0
	mounted := map[string]string{}
	command := ""
	seenCommand := false

	if strings.TrimSpace(cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}

		if arg == "--" {
			continue
		}

		if isLiquibaseConnectionFlag(arg) {
			return liquibasePrepared{}, ValidationError{Code: "invalid_argument", Message: "connection flags are not allowed", Details: arg}
		}
		if isLiquibaseRuntimeFlag(arg) {
			return liquibasePrepared{}, ValidationError{Code: "invalid_argument", Message: "runtime flags are not allowed", Details: arg}
		}

		if handled, err := handleLiquibasePathFlag(args, &i, cwd, windowsMode, &preArgs, &mounts, &mountIndex, mounted); err != nil {
			return liquibasePrepared{}, err
		} else if handled {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			if seenCommand {
				postArgs = append(postArgs, arg)
			} else {
				preArgs = append(preArgs, arg)
			}
			continue
		}

		if command == "" {
			command = arg
			seenCommand = true
		}
		postArgs = append(postArgs, arg)
	}

	if strings.TrimSpace(command) == "" {
		return liquibasePrepared{}, ValidationError{Code: "invalid_argument", Message: "lb command is required"}
	}
	if !strings.HasPrefix(strings.ToLower(command), "update") {
		return liquibasePrepared{}, ValidationError{Code: "invalid_argument", Message: "unsupported lb command", Details: command}
	}

	if !windowsMode && len(mounts) == 0 && strings.TrimSpace(cwd) != "" {
		mountIndex++
		mounts = append(mounts, runtime.Mount{
			HostPath:      cwd,
			ContainerPath: fmt.Sprintf("/sqlrs/mnt/path%d", mountIndex),
			ReadOnly:      true,
		})
	}

	normalized := append(preArgs, postArgs...)
	lockPaths, searchPaths, err := collectLiquibasePaths(args, cwd)
	if err != nil {
		return liquibasePrepared{}, err
	}
	return liquibasePrepared{
		normalizedArgs: normalized,
		argsNormalized: strings.Join(normalized, " "),
		mounts:         mounts,
		lockPaths:      lockPaths,
		searchPaths:    searchPaths,
		workDir:        cwd,
	}, nil
}

func isLiquibaseConnectionFlag(arg string) bool {
	switch arg {
	case "--url", "--username", "--password":
		return true
	}
	return strings.HasPrefix(arg, "--url=") ||
		strings.HasPrefix(arg, "--username=") ||
		strings.HasPrefix(arg, "--password=")
}

func isLiquibaseRuntimeFlag(arg string) bool {
	switch arg {
	case "--classpath", "--driver":
		return true
	}
	return strings.HasPrefix(arg, "--classpath=") ||
		strings.HasPrefix(arg, "--driver=")
}

func handleLiquibasePathFlag(args []string, index *int, cwd string, windowsMode bool, normalized *[]string, mounts *[]runtime.Mount, mountIndex *int, mounted map[string]string) (bool, error) {
	arg := args[*index]
	switch {
	case arg == "--changelog-file" || arg == "--defaults-file" || arg == "--searchPath" || arg == "--search-path":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for flag", Details: arg}
		}
		value := args[*index+1]
		if windowsMode {
			flag := arg
			if flag == "--searchPath" {
				flag = "--searchPath"
			}
			if flag == "--search-path" {
				flag = "--searchPath"
			}
			*normalized = append(*normalized, flag, value)
			*index++
			return true, nil
		}
		flag := arg
		if flag == "--searchPath" {
			flag = "--searchPath"
		}
		if flag == "--search-path" {
			flag = "--searchPath"
		}
		rewritten, err := rewriteLiquibasePathValue(flag, value, cwd, mounts, mountIndex, mounted)
		if err != nil {
			return true, err
		}
		*normalized = append(*normalized, flag, rewritten)
		*index++
		return true, nil
	case strings.HasPrefix(arg, "--changelog-file="):
		value := strings.TrimPrefix(arg, "--changelog-file=")
		if windowsMode {
			*normalized = append(*normalized, arg)
			return true, nil
		}
		rewritten, err := rewriteLiquibasePathValue("--changelog-file", value, cwd, mounts, mountIndex, mounted)
		if err != nil {
			return true, err
		}
		*normalized = append(*normalized, "--changelog-file="+rewritten)
		return true, nil
	case strings.HasPrefix(arg, "--defaults-file="):
		value := strings.TrimPrefix(arg, "--defaults-file=")
		if windowsMode {
			*normalized = append(*normalized, arg)
			return true, nil
		}
		rewritten, err := rewriteLiquibasePathValue("--defaults-file", value, cwd, mounts, mountIndex, mounted)
		if err != nil {
			return true, err
		}
		*normalized = append(*normalized, "--defaults-file="+rewritten)
		return true, nil
	case strings.HasPrefix(arg, "--searchPath="):
		value := strings.TrimPrefix(arg, "--searchPath=")
		if windowsMode {
			*normalized = append(*normalized, "--searchPath="+value)
			return true, nil
		}
		rewritten, err := rewriteLiquibasePathValue("--searchPath", value, cwd, mounts, mountIndex, mounted)
		if err != nil {
			return true, err
		}
		*normalized = append(*normalized, "--searchPath="+rewritten)
		return true, nil
	case strings.HasPrefix(arg, "--search-path="):
		value := strings.TrimPrefix(arg, "--search-path=")
		if windowsMode {
			*normalized = append(*normalized, "--searchPath="+value)
			return true, nil
		}
		rewritten, err := rewriteLiquibasePathValue("--searchPath", value, cwd, mounts, mountIndex, mounted)
		if err != nil {
			return true, err
		}
		*normalized = append(*normalized, "--searchPath="+rewritten)
		return true, nil
	default:
		return false, nil
	}
}

func rewriteLiquibasePathValue(flag string, value string, cwd string, mounts *[]runtime.Mount, mountIndex *int, mounted map[string]string) (string, error) {
	if flag == "--searchPath" || flag == "--search-path" {
		if strings.TrimSpace(value) == "" {
			return "", ValidationError{Code: "invalid_argument", Message: "searchPath is empty"}
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				return "", ValidationError{Code: "invalid_argument", Message: "searchPath is empty"}
			}
			rewritten, err := rewriteSinglePath(item, cwd, mounts, mountIndex, mounted, "searchPath path does not exist")
			if err != nil {
				return "", err
			}
			out = append(out, rewritten)
		}
		return strings.Join(out, ","), nil
	}
	return rewriteSinglePath(value, cwd, mounts, mountIndex, mounted, "path does not exist")
}

func rewriteSinglePath(value string, cwd string, mounts *[]runtime.Mount, mountIndex *int, mounted map[string]string, notFoundMessage string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", ValidationError{Code: "invalid_argument", Message: "path is empty"}
	}
	if looksLikeRemoteRef(value) {
		return value, nil
	}
	path := value
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(cwd) == "" {
			return "", ValidationError{Code: "invalid_argument", Message: "relative path requires working directory"}
		}
		path = filepath.Join(cwd, path)
	}
	if _, err := os.Stat(path); err != nil {
		return "", ValidationError{Code: "invalid_argument", Message: notFoundMessage, Details: path}
	}
	if mapped, ok := mounted[path]; ok {
		return mapped, nil
	}
	*mountIndex++
	mapped := fmt.Sprintf("/sqlrs/mnt/path%d", *mountIndex)
	*mounts = append(*mounts, runtime.Mount{
		HostPath:      path,
		ContainerPath: mapped,
		ReadOnly:      true,
	})
	mounted[path] = mapped
	return mapped, nil
}

func collectLiquibasePaths(args []string, cwd string) ([]string, []string, error) {
	var lockPaths []string
	var searchPaths []string
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" || arg == "--" {
			continue
		}
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file" || arg == "--searchPath" || arg == "--search-path":
			if i+1 >= len(args) {
				return nil, nil, ValidationError{Code: "invalid_argument", Message: "missing value for flag", Details: arg}
			}
			value := strings.TrimSpace(args[i+1])
			i++
			paths, err := normalizeLiquibasePathValues(arg, value, cwd)
			if err != nil {
				return nil, nil, err
			}
			if isSearchPathFlag(arg) {
				searchPaths = append(searchPaths, paths...)
			} else {
				lockPaths = append(lockPaths, paths...)
			}
		case strings.HasPrefix(arg, "--changelog-file="):
			value := strings.TrimPrefix(arg, "--changelog-file=")
			paths, err := normalizeLiquibasePathValues("--changelog-file", value, cwd)
			if err != nil {
				return nil, nil, err
			}
			lockPaths = append(lockPaths, paths...)
		case strings.HasPrefix(arg, "--defaults-file="):
			value := strings.TrimPrefix(arg, "--defaults-file=")
			paths, err := normalizeLiquibasePathValues("--defaults-file", value, cwd)
			if err != nil {
				return nil, nil, err
			}
			lockPaths = append(lockPaths, paths...)
		case strings.HasPrefix(arg, "--searchPath="):
			value := strings.TrimPrefix(arg, "--searchPath=")
			paths, err := normalizeLiquibasePathValues("--searchPath", value, cwd)
			if err != nil {
				return nil, nil, err
			}
			searchPaths = append(searchPaths, paths...)
		case strings.HasPrefix(arg, "--search-path="):
			value := strings.TrimPrefix(arg, "--search-path=")
			paths, err := normalizeLiquibasePathValues("--searchPath", value, cwd)
			if err != nil {
				return nil, nil, err
			}
			searchPaths = append(searchPaths, paths...)
		}
	}
	return lockPaths, searchPaths, nil
}

func normalizeLiquibasePathValues(flag string, value string, cwd string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, ValidationError{Code: "invalid_argument", Message: "path is empty", Details: flag}
	}
	if isSearchPathFlag(flag) {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				return nil, ValidationError{Code: "invalid_argument", Message: "searchPath is empty"}
			}
			if looksLikeRemoteRef(item) {
				continue
			}
			out = append(out, normalizeLiquibasePath(item, cwd))
		}
		return out, nil
	}
	if looksLikeRemoteRef(value) {
		return nil, nil
	}
	return []string{normalizeLiquibasePath(value, cwd)}, nil
}

func normalizeLiquibasePath(value string, cwd string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if looksLikeWindowsPath(value) {
		return value
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if strings.TrimSpace(cwd) == "" {
		return value
	}
	return filepath.Clean(filepath.Join(cwd, value))
}

func isSearchPathFlag(flag string) bool {
	return flag == "--searchPath" || flag == "--search-path"
}

func looksLikeRemoteRef(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "://") || strings.HasPrefix(lower, "classpath:")
}

func parseLiquibaseUpdateSQL(output string) ([]LiquibaseChangeset, error) {
	lines := strings.Split(output, "\n")
	var changesets []LiquibaseChangeset
	var current *LiquibaseChangeset
	var sqlLines []string

	flush := func() error {
		if current == nil {
			return nil
		}
		sql := strings.TrimSpace(strings.Join(sqlLines, "\n"))
		current.SQL = sql
		current.SQLHash = sha256Hex(sql)
		changesets = append(changesets, *current)
		current = nil
		sqlLines = nil
		return nil
	}

	for _, line := range lines {
		if meta, ok := parseChangesetHeader(line); ok {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &LiquibaseChangeset{Path: meta.path, ID: meta.id, Author: meta.author}
			continue
		}
		if current != nil {
			if checksum, ok := parseChangesetChecksum(line); ok && checksum != "" {
				current.Checksum = checksum
			}
		}
		if current != nil {
			sqlLines = append(sqlLines, line)
		}
	}

	if err := flush(); err != nil {
		return nil, err
	}
	if len(changesets) == 0 && strings.TrimSpace(output) != "" {
		return nil, ValidationError{Code: "invalid_argument", Message: "missing changeset"}
	}
	return changesets, nil
}

func replaceLiquibaseCommand(args []string, command string) []string {
	if len(args) == 0 {
		return []string{command}
	}
	out := make([]string, 0, len(args))
	replaced := false
	skipNext := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if skipNext {
			out = append(out, arg)
			skipNext = false
			continue
		}
		if strings.TrimSpace(arg) == "" || arg == "--" {
			out = append(out, arg)
			continue
		}
		if isLiquibaseFlagWithValue(arg) {
			out = append(out, arg)
			if !strings.Contains(arg, "=") {
				skipNext = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			out = append(out, arg)
			continue
		}
		if !replaced {
			out = append(out, command)
			replaced = true
			continue
		}
		out = append(out, arg)
	}
	if !replaced {
		return append(out, command)
	}
	return out
}

func isLiquibaseFlagWithValue(arg string) bool {
	switch arg {
	case "--changelog-file", "--defaults-file", "--searchPath", "--search-path",
		"--classpath", "--driver", "--url", "--username", "--password":
		return true
	}
	return strings.HasPrefix(arg, "--changelog-file=") ||
		strings.HasPrefix(arg, "--defaults-file=") ||
		strings.HasPrefix(arg, "--searchPath=") ||
		strings.HasPrefix(arg, "--search-path=") ||
		strings.HasPrefix(arg, "--classpath=") ||
		strings.HasPrefix(arg, "--driver=") ||
		strings.HasPrefix(arg, "--url=") ||
		strings.HasPrefix(arg, "--username=") ||
		strings.HasPrefix(arg, "--password=")
}

func applyLiquibaseTaskArgs(args []string, task taskState) []string {
	_ = task
	args = replaceLiquibaseCommand(args, "update-count")
	return append(args, "--count", "1")
}

type changesetMeta struct {
	path   string
	id     string
	author string
}

func parseChangesetHeader(line string) (changesetMeta, bool) {
	const prefix = "-- Changeset "
	if !strings.HasPrefix(line, prefix) {
		return changesetMeta{}, false
	}
	meta := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	parts := strings.SplitN(meta, "::", 3)
	if len(parts) != 3 {
		return changesetMeta{}, false
	}
	return changesetMeta{path: parts[0], id: parts[1], author: parts[2]}, true
}

func parseChangesetChecksum(line string) (string, bool) {
	if !strings.Contains(strings.ToLower(line), "databasechangelog") {
		return "", false
	}
	cols, vals, ok := parseInsertColumnsValues(line)
	if !ok || len(cols) == 0 || len(cols) != len(vals) {
		return "", false
	}
	for i, col := range cols {
		if strings.EqualFold(strings.TrimSpace(col), "MD5SUM") {
			return strings.TrimSpace(vals[i]), true
		}
	}
	return "", false
}

func parseInsertColumnsValues(line string) ([]string, []string, bool) {
	lower := strings.ToLower(line)
	if !strings.Contains(lower, "insert into") || !strings.Contains(lower, "values") {
		return nil, nil, false
	}
	valuesIdx := strings.Index(lower, "values")
	if valuesIdx == -1 {
		return nil, nil, false
	}
	colStart := strings.Index(line, "(")
	if colStart == -1 || colStart > valuesIdx {
		return nil, nil, false
	}
	colEnd := strings.LastIndex(line[:valuesIdx], ")")
	if colEnd == -1 || colEnd <= colStart {
		return nil, nil, false
	}
	cols := splitCommaList(line[colStart+1 : colEnd])
	valStart := strings.Index(line[valuesIdx:], "(")
	if valStart == -1 {
		return nil, nil, false
	}
	valStart += valuesIdx
	valEnd := strings.LastIndex(line, ")")
	if valEnd == -1 || valEnd <= valStart {
		return nil, nil, false
	}
	vals := splitSQLValues(line[valStart+1 : valEnd])
	return cols, vals, true
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func splitSQLValues(value string) []string {
	out := []string{}
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inQuote {
			if ch == '\'' {
				if i+1 < len(value) && value[i+1] == '\'' {
					buf.WriteByte('\'')
					i++
					continue
				}
				inQuote = false
				continue
			}
			buf.WriteByte(ch)
			continue
		}
		switch ch {
		case '\'':
			inQuote = true
		case ',':
			out = append(out, strings.TrimSpace(buf.String()))
			buf.Reset()
		default:
			buf.WriteByte(ch)
		}
	}
	if buf.Len() > 0 {
		out = append(out, strings.TrimSpace(buf.String()))
	}
	return out
}

func liquibaseFingerprint(prevStateID string, changesets []LiquibaseChangeset) string {
	hasher := newStateHasher()
	hasher.write("prepare_kind", "lb")
	hasher.write("prev_state_id", prevStateID)
	for _, cs := range changesets {
		hasher.write("changeset_hash", liquibaseChangesetHash(cs))
	}
	return hasher.sum()
}

func liquibaseChangesetHash(cs LiquibaseChangeset) string {
	if strings.TrimSpace(cs.Checksum) != "" {
		return strings.TrimSpace(cs.Checksum)
	}
	return cs.SQLHash
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
