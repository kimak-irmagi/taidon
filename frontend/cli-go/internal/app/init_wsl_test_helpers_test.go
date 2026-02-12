package app

import (
	"context"
	"testing"
)

func withRunWSLCommandStub(t *testing.T, fn func(context.Context, string, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandFn
	runWSLCommandFn = fn
	t.Cleanup(func() { runWSLCommandFn = prev })
}

func withRunWSLCommandAllowFailureStub(t *testing.T, fn func(context.Context, string, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandAllowFailureFn
	runWSLCommandAllowFailureFn = fn
	t.Cleanup(func() { runWSLCommandAllowFailureFn = prev })
}

func withRunWSLCommandWithInputStub(t *testing.T, fn func(context.Context, string, bool, string, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandWithInputFn
	runWSLCommandWithInputFn = fn
	t.Cleanup(func() { runWSLCommandWithInputFn = prev })
}

func withRunHostCommandStub(t *testing.T, fn func(context.Context, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runHostCommandFn
	runHostCommandFn = fn
	t.Cleanup(func() { runHostCommandFn = prev })
}

func withIsElevatedStub(t *testing.T, fn func(bool) (bool, error)) {
	t.Helper()
	prev := isElevatedFn
	isElevatedFn = fn
	t.Cleanup(func() { isElevatedFn = prev })
}

func containsArgs(args []string, flag string, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
