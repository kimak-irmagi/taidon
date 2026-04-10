package cli

import "io"

func PrintPlanUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs plan [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] <ref>\n")
	io.WriteString(w, "  sqlrs plan:psql [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--image <image-id>] [--] [psql-args...]\n")
	io.WriteString(w, "  sqlrs plan:lb [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] -- [liquibase-args...]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --ref <git-ref>      Read plan inputs from a selected Git revision\n")
	io.WriteString(w, "  --ref-mode <mode>    Ref mode: worktree (default) or blob\n")
	io.WriteString(w, "  --ref-keep-worktree  Keep detached worktree after exit (worktree mode only)\n")
	io.WriteString(w, "  --image <image-id>  Override base image id\n")
	io.WriteString(w, "  -h, --help          Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  Alias mode resolves <ref> from the current working directory.\n")
	io.WriteString(w, "  Paths inside the alias file resolve relative to that alias file.\n")
	io.WriteString(w, "  Use -- to pass flags that would otherwise conflict with sqlrs options.\n")
	io.WriteString(w, "  Under --ref, relative paths resolve from the projected current working directory in that revision.\n")
}
