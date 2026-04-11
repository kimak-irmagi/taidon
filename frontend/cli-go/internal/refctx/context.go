package refctx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
)

// Context describes one projected filesystem view for a selected git ref.
// It keeps host paths so callers can reuse the same projected root for alias
// loading, argument normalization, and any later cleanup.
type Context struct {
	RepoRoot        string
	WorkspaceRoot   string
	BaseDir         string
	GitRef          string
	RefMode         string
	KeepWorktree    bool
	FileSystem      inputset.FileSystem
	cleanupWorktree func() error
}

// Resolve builds one projected filesystem context for the selected git ref.
// Worktree mode materializes a detached checkout; blob mode serves reads
// directly from git objects under the repository root.
func Resolve(workspaceRoot, cwd, gitRef, refMode string, keepWorktree bool) (Context, error) {
	gitRef = strings.TrimSpace(gitRef)
	if gitRef == "" {
		return Context{}, fmt.Errorf("git ref is required")
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}

	mode := strings.TrimSpace(strings.ToLower(refMode))
	if mode == "" {
		mode = "worktree"
	}
	if mode != "worktree" && mode != "blob" {
		return Context{}, fmt.Errorf("unsupported ref mode: %s", refMode)
	}

	if reason := gitUnavailable(); reason != "" {
		return Context{}, fmt.Errorf("git: %s", reason)
	}

	repoRoot, err := gitTopLevel(cwd)
	if err != nil {
		return Context{}, err
	}
	relCwd, err := pathWithinRepo(repoRoot, cwd)
	if err != nil {
		return Context{}, fmt.Errorf("resolve cwd: %w", err)
	}

	relWorkspace, hasWorkspaceRoot, err := optionalPathWithinRepo(repoRoot, workspaceRoot)
	if err != nil {
		return Context{}, fmt.Errorf("resolve workspace root: %w", err)
	}

	switch mode {
	case "blob":
		rootCanon := inputset.CanonicalizeBoundaryPath(filepath.Clean(repoRoot))
		baseDir := inputset.CanonicalizeBoundaryPath(filepath.Join(rootCanon, relCwd))
		if err := ensureProjectedDir(inputset.NewGitRevFileSystem(rootCanon, gitRef), baseDir, "projected cwd missing at ref"); err != nil {
			return Context{}, err
		}
		ctx := Context{
			RepoRoot:      rootCanon,
			WorkspaceRoot: "",
			BaseDir:       baseDir,
			GitRef:        gitRef,
			RefMode:       mode,
			KeepWorktree:  keepWorktree,
			FileSystem:    inputset.NewGitRevFileSystem(rootCanon, gitRef),
		}
		if hasWorkspaceRoot {
			ctx.WorkspaceRoot = inputset.CanonicalizeBoundaryPath(filepath.Join(rootCanon, relWorkspace))
		}
		return ctx, nil
	default:
		worktreeDir, err := os.MkdirTemp("", "sqlrs-ref-*")
		if err != nil {
			return Context{}, fmt.Errorf("mkdir worktree: %w", err)
		}
		cleanup := func() error {
			if keepWorktree {
				return nil
			}
			var errs []string
			if err := gitWorktreeRemove(repoRoot, worktreeDir); err != nil {
				errs = append(errs, err.Error())
			}
			if err := os.RemoveAll(worktreeDir); err != nil && !os.IsNotExist(err) {
				errs = append(errs, err.Error())
			}
			if len(errs) > 0 {
				return fmt.Errorf("%s", strings.Join(errs, "; "))
			}
			return nil
		}
		if err := gitWorktreeAddDetach(repoRoot, worktreeDir, gitRef); err != nil {
			_ = cleanup()
			return Context{}, err
		}
		baseDir := inputset.CanonicalizeBoundaryPath(filepath.Join(worktreeDir, relCwd))
		if stat, err := os.Stat(baseDir); err != nil || !stat.IsDir() {
			_ = cleanup()
			if err != nil {
				return Context{}, fmt.Errorf("projected cwd missing at ref: %w", err)
			}
			return Context{}, fmt.Errorf("projected cwd missing at ref: %s", baseDir)
		}
		ctx := Context{
			RepoRoot:        inputset.CanonicalizeBoundaryPath(worktreeDir),
			WorkspaceRoot:   "",
			BaseDir:         baseDir,
			GitRef:          gitRef,
			RefMode:         mode,
			KeepWorktree:    keepWorktree,
			FileSystem:      inputset.OSFileSystem{},
			cleanupWorktree: cleanup,
		}
		if hasWorkspaceRoot {
			ctx.WorkspaceRoot = inputset.CanonicalizeBoundaryPath(filepath.Join(worktreeDir, relWorkspace))
		}
		return ctx, nil
	}
}

// Cleanup releases any temporary resources owned by the ref context.
func (c Context) Cleanup() error {
	if c.cleanupWorktree == nil {
		return nil
	}
	return c.cleanupWorktree()
}

func ensureProjectedDir(fs inputset.FileSystem, path string, message string) error {
	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %s", message, path)
	}
	return nil
}

func optionalPathWithinRepo(repoRoot, raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, nil
	}
	rel, err := pathWithinRepo(repoRoot, raw)
	return rel, true, err
}

func pathWithinRepo(repoRoot, path string) (string, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	absRepo = inputset.CanonicalizeBoundaryPath(absRepo)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = inputset.CanonicalizeBoundaryPath(absPath)

	rel, err := filepath.Rel(absRepo, absPath)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return rel, nil
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("path is outside repository root: %s", path)
	}
	return filepath.Clean(rel), nil
}

func gitUnavailable() string {
	_, err := exec.LookPath("git")
	if err != nil {
		return "git not found in PATH"
	}
	return ""
}

func gitTopLevel(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("not a git repository (or git failed): %w", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

func gitWorktreeAddDetach(repoRoot, path, ref string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "--detach", path, ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add detached worktree for %s: %w: %s", ref, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitWorktreeRemove(repoRoot, path string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove detached worktree %s: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}
