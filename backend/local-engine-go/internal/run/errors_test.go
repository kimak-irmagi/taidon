package run

import "testing"

func TestValidationErrorMessage(t *testing.T) {
	err := ValidationError{Message: "bad"}
	if err.Error() != "bad" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
	err = ValidationError{Message: "bad", Details: "info"}
	if err.Error() != "bad: info" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestNotFoundErrorMessage(t *testing.T) {
	err := NotFoundError{Message: "missing"}
	if err.Error() != "missing" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
	err = NotFoundError{Message: "missing", Details: "info"}
	if err.Error() != "missing: info" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestConflictErrorMessage(t *testing.T) {
	err := ConflictError{Message: "conflict"}
	if err.Error() != "conflict" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
	err = ConflictError{Message: "conflict", Details: "info"}
	if err.Error() != "conflict: info" {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
