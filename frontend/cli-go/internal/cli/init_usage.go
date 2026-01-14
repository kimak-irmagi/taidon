package cli

import "io"

func PrintInitUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs init [flags]\n\n")
	io.WriteString(w, "Flags:\n")
	io.WriteString(w, "  --workspace <path>   Workspace root (default: cwd)\n")
	io.WriteString(w, "  --force              Allow nested workspaces\n")
	io.WriteString(w, "  --engine <path>      Engine binary path\n")
	io.WriteString(w, "  --shared-cache       Enable shared cache in local config\n")
	io.WriteString(w, "  --dry-run            Show intended actions only\n")
	io.WriteString(w, "  -h, --help           Show help\n")
}
