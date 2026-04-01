package cli

import "io"

func PrintAliasUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs alias create <ref> <wrapped-command> [-- <command>...]\n")
	io.WriteString(w, "  sqlrs alias ls [--prepare] [--run] [--from <workspace|cwd|path>] [--depth <self|children|recursive>]\n")
	io.WriteString(w, "  sqlrs alias check [--prepare] [--run] [--from <workspace|cwd|path>] [--depth <self|children|recursive>] [<ref>]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --prepare             Inspect prepare aliases only\n")
	io.WriteString(w, "  --run                 Inspect run aliases only\n")
	io.WriteString(w, "  --from <scope>        Scan from workspace, cwd, or an explicit path\n")
	io.WriteString(w, "  --depth <scope>       Limit scan breadth: self, children, recursive\n")
	io.WriteString(w, "  -h, --help            Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  create materializes a repo-tracked alias file from a wrapped command.\n")
	io.WriteString(w, "  Scan mode defaults to --from cwd --depth recursive.\n")
	io.WriteString(w, "  check <ref> reuses the same cwd-relative alias-ref rules as execution.\n")
	io.WriteString(w, "  Exact-file refs with non-standard filenames require --prepare or --run.\n")
}
