package alias

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveTarget resolves one alias file using the same cwd-relative ref rules
// accepted for execution commands.
func ResolveTarget(opts ResolveOptions) (Target, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Target{}, fmt.Errorf("workspace root is required to resolve aliases")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)
	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd = workspaceRoot
	}
	cwd = filepath.Clean(cwd)

	ref := strings.TrimSpace(opts.Ref)
	if ref == "" {
		return Target{}, fmt.Errorf("alias ref is required")
	}

	class := normalizeClass(opts.Class)
	exact := strings.HasSuffix(ref, ".")
	if exact {
		ref = strings.TrimSuffix(ref, ".")
		if strings.TrimSpace(ref) == "" {
			return Target{}, fmt.Errorf("alias ref is empty")
		}
		return resolveExactTarget(workspaceRoot, cwd, ref, class)
	}
	return resolveStemTarget(workspaceRoot, cwd, ref, class)
}

func resolveStemTarget(workspaceRoot string, cwd string, ref string, class Class) (Target, error) {
	relative := filepath.FromSlash(ref)
	switch class {
	case ClassPrepare:
		return resolveSingleStemTarget(workspaceRoot, cwd, relative+prepareSuffix, ClassPrepare)
	case ClassRun:
		return resolveSingleStemTarget(workspaceRoot, cwd, relative+runSuffix, ClassRun)
	}

	prepareTarget, prepareErr := resolveSingleStemTarget(workspaceRoot, cwd, relative+prepareSuffix, ClassPrepare)
	runTarget, runErr := resolveSingleStemTarget(workspaceRoot, cwd, relative+runSuffix, ClassRun)
	switch {
	case prepareErr == nil && runErr == nil:
		return Target{}, fmt.Errorf("ambiguous alias ref %q; add --prepare, --run, or an exact-file escape", ref)
	case prepareErr == nil:
		return prepareTarget, nil
	case runErr == nil:
		return runTarget, nil
	default:
		return Target{}, fmt.Errorf("alias file not found for ref %q", ref)
	}
}

func resolveSingleStemTarget(workspaceRoot string, cwd string, relative string, class Class) (Target, error) {
	path, err := resolvePathWithinWorkspace(relative, workspaceRoot, cwd)
	if err != nil {
		return Target{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Target{}, fmt.Errorf("alias file not found: %s", path)
	}
	return Target{
		Class: class,
		Ref:   invocationRef(path, cwd, class),
		File:  workspaceRelativePath(path, workspaceRoot),
		Path:  path,
	}, nil
}

func resolveExactTarget(workspaceRoot string, cwd string, ref string, class Class) (Target, error) {
	path, err := resolvePathWithinWorkspace(filepath.FromSlash(ref), workspaceRoot, cwd)
	if err != nil {
		return Target{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Target{}, fmt.Errorf("alias file not found: %s", path)
	}

	inferredClass := classifyPath(path)
	switch {
	case class == "" && inferredClass == "":
		return Target{}, fmt.Errorf("exact-file alias refs without a standard suffix require --prepare or --run")
	case class == "":
		class = inferredClass
	case inferredClass != "" && inferredClass != class:
		return Target{}, fmt.Errorf("selected alias class %q does not match file suffix %q", class, inferredClass)
	}

	target := Target{
		Class: class,
		File:  workspaceRelativePath(path, workspaceRoot),
		Path:  path,
	}
	if inferredClass != "" {
		target.Ref = invocationRef(path, cwd, inferredClass)
	} else {
		rel, relErr := portableRelativePath(cwd, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		target.Ref = filepath.ToSlash(rel)
	}
	return target, nil
}

func resolvePathWithinWorkspace(path string, workspaceRoot string, base string) (string, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return "", fmt.Errorf("workspace root is required")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)
	base = strings.TrimSpace(base)
	if base == "" {
		base = workspaceRoot
	}
	base = filepath.Clean(base)

	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(base, resolved)
	}
	resolved = filepath.Clean(resolved)

	canonicalRoot := canonicalizeBoundaryPath(workspaceRoot)
	canonicalResolved := canonicalizeBoundaryPath(resolved)
	if canonicalRoot != "" && !isWithin(canonicalRoot, canonicalResolved) {
		return "", fmt.Errorf("path must stay within workspace root: %s", resolved)
	}
	return resolved, nil
}
