package cli

import "io"

func PrintPrepareUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs prepare:psql [--watch|--no-watch] [--image <image-id>] [--] [psql-args...]\n\n")
	io.WriteString(w, "  sqlrs prepare:lb [--watch|--no-watch] [--image <image-id>] [--] [liquibase-args...]\n\n")
	io.WriteString(w, "Options:\n")
	io.WriteString(w, "  --watch             Watch progress until terminal status (default)\n")
	io.WriteString(w, "  --no-watch          Submit job and exit immediately with job references\n")
	io.WriteString(w, "  --image <image-id>  Override base image id\n")
	io.WriteString(w, "  -h, --help          Show help\n\n")
	io.WriteString(w, "Notes:\n")
	io.WriteString(w, "  Use -- to pass flags that would otherwise conflict with sqlrs options.\n")
}
