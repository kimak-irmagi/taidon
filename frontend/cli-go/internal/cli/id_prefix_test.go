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

func TestIDPrefixErrorNil(t *testing.T) {
	var err *IDPrefixError
	if err.Error() != "" {
		t.Fatalf("expected empty error message")
	}
}

func TestAmbiguousPrefixErrorNil(t *testing.T) {
	var err *AmbiguousPrefixError
	if err.Error() != "" {
		t.Fatalf("expected empty error message")
	}
}

func TestNormalizeIDPrefixValid(t *testing.T) {
	value, err := normalizeIDPrefix("instance", "AbCdEf12")
	if err != nil {
		t.Fatalf("normalizeIDPrefix: %v", err)
	}
	if value != "abcdef12" {
		t.Fatalf("expected lowercase prefix, got %q", value)
	}
}

func TestNormalizeIDPrefixErrors(t *testing.T) {
	cases := []string{"", "abc", "zzzzzzzz"}
	for _, value := range cases {
		if _, err := normalizeIDPrefix("state", value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}
