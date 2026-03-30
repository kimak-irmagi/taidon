package diff

import (
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
)

// BuildLbFileList builds the ordered file list for plan:lb / prepare:lb by
// projecting the shared Liquibase input set into diff.FileList.
func BuildLbFileList(ctx Context, args []string) (FileList, error) {
	set, err := inputliquibase.Collect(args, inputset.NewWorkspaceResolver(ctx.Root, ctx.BaseDir, nil), fileSystemForContext(ctx))
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
