package cli

import "testing"

func TestIDPrefixErrorMessage(t *testing.T) {
	err := &IDPrefixError{Prefix: "abc", Reason: "must be hex"}
	if err.Error() == "" || err.Kind != "" && err.Error() == "" {
		t.Fatalf("expected error message")
	}
	kindErr := &IDPrefixError{Kind: "instance", Prefix: "abc", Reason: "must be hex"}
	if kindErr.Error() == "" {
		t.Fatalf("expected kind error message")
	}
}

func TestAmbiguousPrefixErrorMessage(t *testing.T) {
	err := &AmbiguousPrefixError{Prefix: "deadbeef"}
	if err.Error() == "" {
		t.Fatalf("expected error message")
	}
	kindErr := &AmbiguousPrefixError{Kind: "state", Prefix: "deadbeef"}
	if kindErr.Error() == "" {
		t.Fatalf("expected kind error message")
	}
}
