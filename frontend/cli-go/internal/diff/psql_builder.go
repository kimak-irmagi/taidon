package diff

import (
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
)

func fileSystemForContext(ctx Context) inputset.FileSystem {
	if strings.TrimSpace(ctx.GitRef) != "" {
		return inputset.NewGitRevFileSystem(ctx.Root, ctx.GitRef)
	}
	return inputset.OSFileSystem{}
}

// BuildPsqlFileList builds the ordered file list for plan:psql / prepare:psql
// by projecting the shared psql input set into diff.FileList.
func BuildPsqlFileList(ctx Context, args []string) (FileList, error) {
	set, err := inputpsql.Collect(args, inputset.NewWorkspaceResolver(ctx.Root, ctx.BaseDir, nil), fileSystemForContext(ctx))
	if err != nil {
		return FileList{}, err
	}
	entries := make([]FileEntry, 0, len(set.Entries))
	for _, entry := range set.Entries {
		entries = append(entries, FileEntry{
			Path: entry.Path,
			Hash: entry.Hash,
		})
	}
	return FileList{Entries: entries}, nil
}
