package cli

import "io"

func PrintPrepareUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] <ref>\n")
	io.WriteString(w, "  sqlrs prepare:psql [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--image <image-id>] [--] [psql-args...]\n")
	io.WriteString(w, "  sqlrs prepare:lb [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--image <image-id>] [--] [liquibase-args...]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --ref <git-ref>      Read prepare inputs from a selected Git revision\n")
	io.WriteString(w, "  --ref-mode <mode>    Ref mode: worktree (default) or blob\n")
	io.WriteString(w, "  --ref-keep-worktree  Keep detached worktree after exit (worktree mode only)\n")
	io.WriteString(w, "  --watch             Watch progress until terminal status (default)\n")
	io.WriteString(w, "  --no-watch          Submit job and exit immediately with job references\n")
	io.WriteString(w, "  --image <image-id>  Override base image id\n")
	io.WriteString(w, "  -h, --help          Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  Alias mode resolves <ref> from the current working directory.\n")
	io.WriteString(w, "  Paths inside the alias file resolve relative to that alias file.\n")
	io.WriteString(w, "  Use -- to pass flags that would otherwise conflict with sqlrs options.\n")
	io.WriteString(w, "  In composite form, prepare ... run may mix raw and alias stages.\n")
	io.WriteString(w, "  Bounded --ref support is single-stage only; prepare --ref ... run ... is not supported yet.\n")
}
