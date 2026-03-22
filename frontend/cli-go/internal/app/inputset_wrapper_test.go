package app

import (
	"errors"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
)

func TestWrapInputsetErrorBranches(t *testing.T) {
	if wrapInputsetError(nil) != nil {
		t.Fatalf("expected nil error to stay nil")
	}

	generic := errors.New("boom")
	if got := wrapInputsetError(generic); !errors.Is(got, generic) {
		t.Fatalf("expected generic error passthrough, got %v", got)
	}

	got := wrapInputsetError(inputset.Errorf("bad_arg", "bad arg"))
	exitErr, ok := got.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if exitErr.Code != 2 || got.Error() != "bad arg" {
		t.Fatalf("unexpected wrapped error: %+v", exitErr)
	}
}
