package cli

import "io"

func PrintRunUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs run[:kind] [--instance <id|name>] [-- <command> ] [args...]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --instance <id|name>  Target instance id or name\n")
	io.WriteString(w, "  -h, --help            Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  If <command> is omitted, the run kind default command is used.\n")
}
