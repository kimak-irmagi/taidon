package cli

import "io"

func PrintDiscoverUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --aliases         Run the aliases analyzer\n")
	io.WriteString(w, "  --gitignore       Run the gitignore hygiene analyzer\n")
	io.WriteString(w, "  --vscode          Run the VS Code guidance analyzer\n")
	io.WriteString(w, "  --prepare-shaping Run the prepare-shaping analyzer\n")
	io.WriteString(w, "  -h, --help        Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  discover is advisory and read-only.\n")
	io.WriteString(w, "  Bare discover runs all stable analyzers in canonical order.\n")
	io.WriteString(w, "  Some analyzers print copy-paste follow-up commands, but discover itself does not write files.\n")
}
