package main

import (
	"errors"
	"fmt"
	"os"

	"sqlrs/cli/internal/app"
)

var runApp = app.Run

var exitFn = os.Exit

func run(args []string) (int, error) {
	if err := runApp(args); err != nil {
		var exitErr *app.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Code, exitErr
		}
		return 1, err
	}
	return 0, nil
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	if code != 0 {
		exitFn(code)
	}
}
