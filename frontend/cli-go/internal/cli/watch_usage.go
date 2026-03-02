package cli

import "io"

func PrintWatchUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs watch <job-id>\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  -h, --help  Show help\n")
}
