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
	end := valueEnd(text)
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

// AppendEscaped appends text escaped as Quote escapes it, without the
// surrounding quotes and without escaping '"'. It reads at most MaxValueBytes
// of text, keeps complete UTF-8 characters at the cut, and appends "..." when
// the tail was omitted. One value appends at most 4*MaxValueBytes+3 bytes.
func AppendEscaped(dst []byte, text string) []byte {
	end := valueEnd(text)
	var scratch [32]byte
	for i := 0; i < end; {
		// Printable ASCII other than backslash is appended as one run, so
		// ordinary names cost one copy.
		if run := plainASCIIEnd(text, i, end); run > i {
			dst = append(dst, text[i:run]...)
			i = run
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:end])
		switch {
		case r == utf8.RuneError && size == 1, r == '\\', !strconv.IsPrint(r):
			quoted := strconv.AppendQuote(scratch[:0], text[i:i+size])
			dst = append(dst, quoted[1:len(quoted)-1]...)
		default:
			dst = append(dst, text[i:i+size]...)
		}
		i += size
	}
	if end < len(text) {
		dst = append(dst, "..."...)
	}
	return dst
}

// plainASCIIEnd returns the end of the run in text[start:end] of printable
// ASCII bytes that escape to themselves.
func plainASCIIEnd(text string, start, end int) int {
	for start < end && text[start] >= ' ' && text[start] < 0x7f && text[start] != '\\' {
		start++
	}
	return start
}

// valueEnd returns the length of the displayed prefix of text: at most
// MaxValueBytes, backed off to a UTF-8 character boundary.
func valueEnd[T ~string | ~[]byte](text T) int {
	end := min(len(text), MaxValueBytes)
	if end < len(text) {
		for end > 0 && !utf8.RuneStart(text[end]) {
			end--
		}
	}
	return end
}
