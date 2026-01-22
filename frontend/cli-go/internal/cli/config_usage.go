package cli

import (
	"fmt"
	"io"
)

func PrintConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs config get <path> [--effective]")
	fmt.Fprintln(w, "  sqlrs config set <path> <json_value>")
	fmt.Fprintln(w, "  sqlrs config rm <path>")
	fmt.Fprintln(w, "  sqlrs config schema")
}
