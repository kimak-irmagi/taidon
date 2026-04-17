package app

import (
	"errors"

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
