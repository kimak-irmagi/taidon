package alias

import "fmt"

// UserError marks user-facing alias-definition validation failures so the app
// layer can preserve historical exit-code behavior while sharing one loader.
type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

func userErrorf(format string, args ...any) *UserError {
	return &UserError{Message: fmt.Sprintf(format, args...)}
}
