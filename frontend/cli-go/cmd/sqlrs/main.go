package main

import (
	"errors"
	"fmt"
	"os"

	"sqlrs/cli/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		var exitErr *app.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
