package diff

import (
	"fmt"
	"os"
	"path/filepath"
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
