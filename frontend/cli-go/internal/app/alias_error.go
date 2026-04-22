package app

import (
	"errors"
	"strings"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
)

func wrapAliasLoadError(err error) error {
	if err == nil {
		return nil
	}
	var userErr *aliaspkg.UserError
	if errors.As(err, &userErr) {
		return &ExitError{Code: 2, Err: userErr}
	}
	return err
}

func wrapAliasResolveError(class aliaspkg.Class, err error) error {
	if err == nil {
		return nil
	}

	label := string(class)
	message := err.Error()
	switch {
	case strings.Contains(message, "workspace root is required to resolve aliases"):
		return ExitErrorf(2, "workspace root is required to resolve %s aliases", label)
	case strings.Contains(message, "alias ref is empty"):
		return ExitErrorf(2, "%s alias ref is empty", label)
	case strings.HasPrefix(message, "alias file not found: "):
		return ExitErrorf(2, "%s alias file not found: %s", label, strings.TrimPrefix(message, "alias file not found: "))
	case strings.Contains(message, "alias file not found"):
		return ExitErrorf(2, "%s alias file not found", label)
	default:
		return &ExitError{Code: 2, Err: err}
	}
}
