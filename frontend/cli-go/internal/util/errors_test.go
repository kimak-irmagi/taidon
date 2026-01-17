package util

import (
	"errors"
	"testing"
)

func TestWrap(t *testing.T) {
	if Wrap(nil, "msg") != nil {
		t.Fatalf("expected nil for nil error")
	}
	base := errors.New("base")
	err := Wrap(base, "msg")
	if err == nil || !errors.Is(err, base) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
