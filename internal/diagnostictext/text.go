// Package diagnostictext bounds and escapes values included in diagnostics.
package diagnostictext

import (
	"strconv"
	"unicode/utf8"
)

const (
	// MaxAlternatives bounds displayed alternatives in a diagnostic.
	MaxAlternatives = 8
	// MaxValueBytes bounds each UTF-8 prefix before quoting and escaping.
	MaxValueBytes = 128
)

// Quote returns an owned, escaped representation. A suffix outside the quotes
// marks an omitted tail. It never copies or retains more than MaxValueBytes of
// input, and preserves complete UTF-8 characters at the truncation boundary.
func Quote[T ~string | ~[]byte](text T) string {
	end := min(len(text), MaxValueBytes)
	if end < len(text) {
		for end > 0 && !utf8.RuneStart(text[end]) {
			end--
		}
	}
	quoted := strconv.Quote(string(text[:end]))
	if end < len(text) {
		quoted += "..."
	}
	return quoted
}

// ExpandedName quotes {namespace}local without first allocating the full name.
// An empty namespace uses the local name alone.
func ExpandedName(namespace, local string) string {
	if namespace == "" {
		return Quote(local)
	}
	return QuoteParts("{", namespace, "}", local)
}

// QuoteParts quotes concatenated parts without allocating their complete
// concatenation. It reads at most MaxValueBytes+1 bytes across all parts.
func QuoteParts(parts ...string) string {
	prefix := make([]byte, 0, MaxValueBytes+1)
	for _, part := range parts {
		prefix = append(prefix, part[:min(len(part), MaxValueBytes+1-len(prefix))]...)
		if len(prefix) > MaxValueBytes {
			break
		}
	}
	return Quote(prefix)
}
