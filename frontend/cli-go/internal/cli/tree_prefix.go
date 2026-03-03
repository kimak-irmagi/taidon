package cli

import "strings"

// compactTreePrefix renders a compact tree branch prefix using one character per depth level.
// See docs/user-guides/sqlrs-ls.md and docs/user-guides/sqlrs-rm.md.

func compactTreePrefix(ancestorsHasNext []bool, isLast bool) string {
	var b strings.Builder
	b.Grow(len(ancestorsHasNext) + 1)
	for _, hasNext := range ancestorsHasNext {
		if hasNext {
			b.WriteByte('|')
		} else {
			b.WriteByte(' ')
		}
	}
	if isLast {
		b.WriteByte('`')
	} else {
		b.WriteByte('+')
	}
	return b.String()
}

func compactTreeNextAncestors(ancestorsHasNext []bool, parentIsLast bool) []bool {
	next := make([]bool, 0, len(ancestorsHasNext)+1)
	next = append(next, ancestorsHasNext...)
	next = append(next, !parentIsLast)
	return next
}
