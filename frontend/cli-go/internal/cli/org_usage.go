package cli

import (
	"fmt"
	"io"
)

func PrintOrgUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs org create <slug> [--name <display-name>]")
	fmt.Fprintln(w, "  sqlrs org ls")
	fmt.Fprintln(w, "  sqlrs org get <org-ref>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Organization commands require --mode=remote and an explicit remote endpoint.")
}
