package app

import (
	"errors"
	"testing"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
)

func TestWrapAliasLoadErrorBranches(t *testing.T) {
	if wrapAliasLoadError(nil) != nil {
		t.Fatalf("expected nil error to stay nil")
	}

	generic := errors.New("boom")
	if got := wrapAliasLoadError(generic); !errors.Is(got, generic) {
		t.Fatalf("expected generic error passthrough, got %v", got)
	}

	got := wrapAliasLoadError(&aliaspkg.UserError{Message: "bad alias"})
	exitErr, ok := got.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if exitErr.Code != 2 || got.Error() != "bad alias" {
		t.Fatalf("unexpected wrapped error: %+v", exitErr)
	}
}
