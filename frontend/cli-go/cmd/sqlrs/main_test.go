package main

import (
	"errors"
	"os"
	"testing"

	"sqlrs/cli/internal/app"
)

func TestRunSuccess(t *testing.T) {
	previous := runApp
	runApp = func(args []string) error {
		return nil
	}
	t.Cleanup(func() { runApp = previous })

	code, err := run([]string{"status"})
	if err != nil || code != 0 {
		t.Fatalf("expected success, got code=%d err=%v", code, err)
	}
}

func TestRunExitError(t *testing.T) {
	previous := runApp
	runApp = func(args []string) error {
		return app.ExitErrorf(2, "bad args")
	}
	t.Cleanup(func() { runApp = previous })

	code, err := run([]string{"status"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	var exitErr *app.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
}

func TestRunOtherError(t *testing.T) {
	previous := runApp
	runApp = func(args []string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { runApp = previous })

	code, err := run([]string{"status"})
	if code != 1 || err == nil || err.Error() != "boom" {
		t.Fatalf("expected error, got code=%d err=%v", code, err)
	}
}

func TestMainUsesExitCode(t *testing.T) {
	prevExit := exitFn
	exitCode := 0
	exitFn = func(code int) {
		exitCode = code
	}
	t.Cleanup(func() { exitFn = prevExit })

	prevRun := runApp
	runApp = func(args []string) error {
		return app.ExitErrorf(2, "bad args")
	}
	t.Cleanup(func() { runApp = prevRun })

	prevArgs := os.Args
	os.Args = []string{"sqlrs", "status"}
	t.Cleanup(func() { os.Args = prevArgs })

	main()
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
}
