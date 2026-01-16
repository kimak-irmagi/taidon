package cli

import (
	"fmt"
	"strings"
)

type IDPrefixError struct {
	Kind   string
	Prefix string
	Reason string
}

func (e *IDPrefixError) Error() string {
	if e == nil {
		return ""
	}
	if e.Kind == "" {
		return fmt.Sprintf("invalid id prefix %q: %s", e.Prefix, e.Reason)
	}
	return fmt.Sprintf("invalid %s id prefix %q: %s", e.Kind, e.Prefix, e.Reason)
}

type AmbiguousPrefixError struct {
	Kind   string
	Prefix string
}

func (e *AmbiguousPrefixError) Error() string {
	if e == nil {
		return ""
	}
	if e.Kind == "" {
		return fmt.Sprintf("ambiguous id prefix: %s", e.Prefix)
	}
	return fmt.Sprintf("ambiguous %s id prefix: %s", e.Kind, e.Prefix)
}

func normalizeIDPrefix(kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &IDPrefixError{Kind: kind, Prefix: value, Reason: "empty prefix"}
	}
	if len(value) < 8 {
		return "", &IDPrefixError{Kind: kind, Prefix: value, Reason: "must be at least 8 hex characters"}
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return "", &IDPrefixError{Kind: kind, Prefix: value, Reason: "must be hex"}
	}
	return strings.ToLower(value), nil
}
