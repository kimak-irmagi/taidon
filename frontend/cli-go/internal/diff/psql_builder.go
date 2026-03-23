package diff

import (
	"github.com/sqlrs/cli/internal/inputset"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
)

// BuildPsqlFileList builds the ordered file list for plan:psql / prepare:psql
// by projecting the shared psql input set into diff.FileList.
func BuildPsqlFileList(ctx Context, args []string) (FileList, error) {
	set, err := inputpsql.Collect(args, inputset.NewWorkspaceResolver(ctx.Root, ctx.BaseDir, nil), inputset.OSFileSystem{})
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
