package prepare

import (
	"errors"
	"testing"
)

func TestValidationErrorMessage(t *testing.T) {
	err := ValidationError{Code: "invalid_argument", Message: "bad input"}
	if err.Error() != "bad input" {
		t.Fatalf("unexpected error string: %q", err.Error())
	}

	err = ValidationError{Code: "invalid_argument", Message: "bad input", Details: "details"}
	if err.Error() != "bad input: details" {
		t.Fatalf("unexpected error string: %q", err.Error())
	}
}

func TestToErrorResponse(t *testing.T) {
	ve := ValidationError{Code: "invalid_argument", Message: "bad", Details: "details"}
	resp := ToErrorResponse(ve)
	if resp == nil || resp.Code != "invalid_argument" || resp.Message != "bad" || resp.Details != "details" {
		t.Fatalf("unexpected validation response: %+v", resp)
	}

	ptr := &ValidationError{Code: "invalid_argument", Message: "bad", Details: "details"}
	resp = ToErrorResponse(ptr)
	if resp == nil || resp.Code != "invalid_argument" || resp.Message != "bad" || resp.Details != "details" {
		t.Fatalf("unexpected pointer response: %+v", resp)
	}

	resp = ToErrorResponse(errors.New("boom"))
	if resp == nil || resp.Code != "internal_error" || resp.Message != "internal error" || resp.Details != "boom" {
		t.Fatalf("unexpected generic response: %+v", resp)
	}

	resp = ToErrorResponse(nil)
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
}
