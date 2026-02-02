package types

import (
	"iter"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// WhiteSpace represents whitespace normalization
type WhiteSpace int

const (
	WhiteSpacePreserve WhiteSpace = iota
	WhiteSpaceReplace
	WhiteSpaceCollapse
)

type whiteSpaceNormalizer struct{}

func (n whiteSpaceNormalizer) Normalize(value string, typ Type) (string, error) {
	if typ == nil {
		return value, nil
	}
	return ApplyWhiteSpace(value, typ.WhiteSpace()), nil
}

// ApplyWhiteSpace applies whitespace normalization
func ApplyWhiteSpace(value string, ws WhiteSpace) string {
	if value == "" {
		return value
	}
	mode := runtime.WS_Preserve
	switch ws {
	case WhiteSpaceReplace:
		mode = runtime.WS_Replace
	case WhiteSpaceCollapse:
		mode = runtime.WS_Collapse
	}
	if mode == runtime.WS_Preserve {
		return value
	}
	normalized := valuepkg.NormalizeWhitespace(mode, []byte(value), nil)
	return string(normalized)
}

// NormalizeWhiteSpace applies whitespace normalization for simple types.
// Non-simple types are returned unchanged.
func NormalizeWhiteSpace(value string, typ Type) string {
	if typ == nil {
		return value
	}
	switch typ.(type) {
	case *SimpleType, *BuiltinType:
		return ApplyWhiteSpace(value, typ.WhiteSpace())
	default:
		return value
	}
}

func splitXMLWhitespaceFields(value string) []string {
	return strings.FieldsFunc(value, isXMLWhitespaceRune)
}

// SplitXMLWhitespaceFields splits a string on XML whitespace (space, tab, CR, LF).
// It returns nil for empty input.
func SplitXMLWhitespaceFields(value string) []string {
	if value == "" {
		return nil
	}
	return splitXMLWhitespaceFields(value)
}

// TrimXMLWhitespace removes leading and trailing XML whitespace (space, tab, CR, LF).
func TrimXMLWhitespace(value string) string {
	start := 0
	end := len(value)
	for start < end && IsXMLWhitespaceByte(value[start]) {
		start++
	}
	for end > start && IsXMLWhitespaceByte(value[end-1]) {
		end--
	}
	if start == 0 && end == len(value) {
		return value
	}
	return value[start:end]
}

// FieldsXMLWhitespaceSeq yields fields split on XML whitespace (space, tab, CR, LF).
// It is equivalent to strings.FieldsSeq for XML whitespace only.
func FieldsXMLWhitespaceSeq(value string) iter.Seq[string] {
	return func(yield func(string) bool) {
		i := 0
		for i < len(value) {
			for i < len(value) && IsXMLWhitespaceByte(value[i]) {
				i++
			}
			if i >= len(value) {
				return
			}
			start := i
			for i < len(value) && !IsXMLWhitespaceByte(value[i]) {
				i++
			}
			if !yield(value[start:i]) {
				return
			}
		}
	}
}

func isXMLWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// IsXMLWhitespaceByte reports whether the byte is XML whitespace.
func IsXMLWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
