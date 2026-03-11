package cli

import "io"

func PrintLsUsage(w io.Writer) {
	io.WriteString(w, "Usage:\n")
	io.WriteString(w, "  sqlrs ls [flags]\n\n")
	io.WriteString(w, "Selectors:\n")
	io.WriteString(w, "  -n, --names       List names\n")
	io.WriteString(w, "  -i, --instances   List instances\n")
	io.WriteString(w, "  -s, --states      List states\n")
	io.WriteString(w, "  -j, --jobs        List jobs\n")
	io.WriteString(w, "  -t, --tasks       List tasks\n")
	io.WriteString(w, "  --all             List all objects\n\n")
	io.WriteString(w, "Filters:\n")
	io.WriteString(w, "  --name <name>         Filter by name\n")
	io.WriteString(w, "  --instance <id>       Filter by instance id\n")
	io.WriteString(w, "  --state <id>          Filter by state id\n")
	io.WriteString(w, "  --job <id>            Filter by job id\n")
	io.WriteString(w, "  --kind <prepareKind>  Filter by prepare kind\n")
	io.WriteString(w, "  --image <imageId>     Filter by base image id\n\n")
	io.WriteString(w, "Output:\n")
	io.WriteString(w, "  --quiet           Suppress section titles\n")
	io.WriteString(w, "  --no-header       Suppress table header\n")
	io.WriteString(w, "  --long            Show full ids and absolute timestamps\n")
	io.WriteString(w, "  --wide            Disable PREPARE_ARGS and ARGS truncation\n")
	io.WriteString(w, "  --signature       Show diagnostic job signature for jobs\n")
	io.WriteString(w, "  --cache-details   Show additional cache metadata for state rows\n")
	io.WriteString(w, "  -h, --help        Show help\n")
}
