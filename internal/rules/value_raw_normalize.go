package rules

import "strings"

// normalizePythonLiteral strips structural whitespace from a Python literal
// string so that two representations that differ only by whitespace inserted
// by Checkmk's server-side re-serialization (e.g. Python's str(dict), which
// adds a space after ':' and ',') compare as equal.
//
// Whitespace INSIDE quoted string literals is preserved verbatim, because
// rule values (e.g. PromQL queries) contain meaningful spaces.
//
// Limitation: it does not reorder dict keys or reconcile Python operators,
// which is intentional — Checkmk preserves insertion order and only changes
// whitespace, so a whitespace-insensitive comparison is sufficient and safe.
func normalizePythonLiteral(s string) string {
	var out strings.Builder
	out.Grow(len(s))

	var quote byte // 0 when not inside a string literal
	for i := 0; i < len(s); i++ {
		c := s[i]

		if quote != 0 {
			// Inside a string literal: copy everything, honoring backslash escapes.
			out.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				out.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
			continue
		}

		switch c {
		case '\'', '"':
			quote = c
			out.WriteByte(c)
		case ' ', '\t', '\n', '\r':
			// Drop structural whitespace outside string literals.
		default:
			out.WriteByte(c)
		}
	}

	return out.String()
}

// valueRawSemanticEquals reports whether two value_raw strings are semantically
// identical for Checkmk purposes, ignoring whitespace that Checkmk inserts or
// removes when it re-serializes Python literals. This mirrors the approach used
// for notification rules (jsonSemanticEquals) but for Python-literal payloads.
func valueRawSemanticEquals(a, b string) bool {
	return normalizePythonLiteral(a) == normalizePythonLiteral(b)
}
