package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
)

// maxSnippetBytes is the maximum bytes of file content embedded when --include-content is set.
const maxSnippetBytes = 8192

// fileSnippet reads up to maxSnippetBytes from root/relPath (relPath uses slash separators).
// If the file is larger, content is truncated and a suffix notes truncation.
func fileSnippet(root, relPath string) string {
	if root == "" {
		return ""
	}
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	b, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	if len(b) <= maxSnippetBytes {
		return string(b)
	}
	return string(b[:maxSnippetBytes]) + "\n... (truncated)"
}

// fileSnippetCtx reads a snippet like fileSnippet but uses git objects when ctx.GitRef is set.
func fileSnippetCtx(ctx Context, relPath string) string {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return ""
	}
	if strings.TrimSpace(ctx.GitRef) != "" {
		fs := inputset.NewGitRevFileSystem(ctx.Root, ctx.GitRef)
		abs := filepath.Join(ctx.Root, filepath.FromSlash(relPath))
		b, err := fs.ReadFile(abs)
		if err != nil {
			return fmt.Sprintf("<error: %v>", err)
		}
		if len(b) <= maxSnippetBytes {
			return string(b)
		}
		return string(b[:maxSnippetBytes]) + "\n... (truncated)"
	}
	return fileSnippet(ctx.Root, relPath)
}
