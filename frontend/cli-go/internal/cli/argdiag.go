package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// UnicodeDashFlagMessage returns a human-readable hint when a flag-like token
// uses a Unicode dash instead of ASCII '-'. Returns an empty string otherwise.
func UnicodeDashFlagMessage(args []string) string {
	for _, arg := range args {
		if !isFlagLikeToken(arg) {
			continue
		}
		bad, ok := firstUnicodeDash(arg)
		if !ok {
			continue
		}
		suggested := normalizeUnicodeDashes(arg)
		return fmt.Sprintf("argument %q contains Unicode dash %q (U+%04X). Use ASCII '-' instead, e.g. %q", arg, string(bad), bad, suggested)
	}
	return ""
}

func isFlagLikeToken(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(arg)
	return first == '-' || isUnicodeDash(first)
}

func firstUnicodeDash(arg string) (rune, bool) {
	for _, r := range arg {
		if isUnicodeDash(r) {
			return r, true
		}
	}
	return 0, false
}

func normalizeUnicodeDashes(arg string) string {
	var b strings.Builder
	b.Grow(len(arg))
	for _, r := range arg {
		if isUnicodeDash(r) {
			b.WriteByte('-')
			continue
		}
		b.WriteRune(r)
	}
	normalized := b.String()
	first, _ := utf8.DecodeRuneInString(strings.TrimSpace(arg))
	if isUnicodeDash(first) && strings.HasPrefix(normalized, "-") && !strings.HasPrefix(normalized, "--") {
		rest := strings.TrimPrefix(normalized, "-")
		if len(rest) > 1 {
			normalized = "--" + rest
		}
	}
	return normalized
}

func isUnicodeDash(r rune) bool {
	switch r {
	case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212', '\uFE58', '\uFE63', '\uFF0D':
		return true
	default:
		return false
	}
}
