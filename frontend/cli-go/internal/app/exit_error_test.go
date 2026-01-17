package app

import (
	"errors"
	"testing"
)

func TestExitErrorWraps(t *testing.T) {
	err := ExitErrorf(2, "bad %s", "input")
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected code 2, got %d", exitErr.Code)
	}
	if exitErr.Error() != "bad input" {
		t.Fatalf("unexpected error message: %q", exitErr.Error())
	}
	if !errors.Is(err, exitErr.Err) {
		t.Fatalf("expected unwrap to underlying error")
	}
}
