package cli

import "io"

func PrintDiscoverUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs discover [--aliases]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --aliases         Run the aliases analyzer (default)\n")
	io.WriteString(w, "  -h, --help        Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  discover is advisory and read-only.\n")
	io.WriteString(w, "  The current slice behaves like discover --aliases.\n")
	io.WriteString(w, "  It prints copy-paste sqlrs alias create commands for each finding.\n")
}
