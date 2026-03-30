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
	return fileSnippetCtx(Context{Root: root}, relPath)
}

// fileSnippetCtx reads a snippet for human/JSON render; uses git objects when ctx.GitRef is set.
func fileSnippetCtx(ctx Context, relPath string) string {
	if ctx.Root == "" {
		return ""
	}
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return ""
	}
	var b []byte
	var err error
	if strings.TrimSpace(ctx.GitRef) != "" {
		fs := inputset.NewGitRevFileSystem(ctx.Root, ctx.GitRef)
		abs := filepath.Join(ctx.Root, filepath.FromSlash(relPath))
		b, err = fs.ReadFile(abs)
	} else {
		abs := filepath.Join(ctx.Root, filepath.FromSlash(relPath))
		b, err = os.ReadFile(abs)
	}
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	if len(b) <= maxSnippetBytes {
		return string(b)
	}
	return string(b[:maxSnippetBytes]) + "\n... (truncated)"
}
