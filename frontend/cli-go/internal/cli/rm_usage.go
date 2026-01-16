package cli

import "io"

func PrintRmUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs rm [flags] <id_prefix>\n\n")
	io.WriteString(w, "Flags:\n")
	io.WriteString(w, "  -r, --recurse     Remove descendant states and instances\n")
	io.WriteString(w, "  -f, --force       Ignore active connections\n")
	io.WriteString(w, "  --dry-run         Show intended actions only\n")
	io.WriteString(w, "  -h, --help        Show help\n")
}
