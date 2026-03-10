package cli

import "io"

func PrintStatusUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs status [--cache]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --cache         Show full cache diagnostics\n")
	io.WriteString(w, "  -h, --help      Show help\n")
}
