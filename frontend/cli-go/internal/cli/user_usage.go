package cli

import (
	"fmt"
	"io"
)

func PrintUserUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs user me")
	fmt.Fprintln(w, "  sqlrs user register [--display-name <name>] [--email <email>]")
	fmt.Fprintln(w, "  sqlrs user create --identity-issuer <issuer> --identity-subject <subject> [--identity-provider <provider>] [--display-name <name>] [--email <email>]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "User commands require --mode=remote and an explicit remote endpoint.")
}
