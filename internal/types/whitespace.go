package types

import (
	"iter"
	"strings"

	"github.com/jacoelho/xsd/internal/value"
)

// WhiteSpace represents whitespace normalization
type WhiteSpace int

const (
	WhiteSpacePreserve WhiteSpace = iota
	WhiteSpaceReplace
	WhiteSpaceCollapse
)

type whiteSpaceNormalizer struct{}

func (n whiteSpaceNormalizer) Normalize(lexical string, typ Type) (string, error) {
	if typ == nil {
		return lexical, nil
	}
	return ApplyWhiteSpace(lexical, typ.WhiteSpace()), nil
}

// ApplyWhiteSpace applies whitespace normalization
func ApplyWhiteSpace(lexical string, ws WhiteSpace) string {
	if lexical == "" {
		return lexical
	}
	mode := value.WhitespacePreserve
	switch ws {
	case WhiteSpaceReplace:
		mode = value.WhitespaceReplace
	case WhiteSpaceCollapse:
		mode = value.WhitespaceCollapse
	}
	if mode == value.WhitespacePreserve {
		return lexical
	}
	normalized := value.NormalizeWhitespace(mode, []byte(lexical), nil)
	return string(normalized)
}

// NormalizeWhiteSpace applies whitespace normalization for simple types.
// Non-simple types are returned unchanged.
func NormalizeWhiteSpace(lexical string, typ Type) string {
	if typ == nil {
		return lexical
	}
	switch typ.(type) {
	case *SimpleType, *BuiltinType:
		return ApplyWhiteSpace(lexical, typ.WhiteSpace())
	default:
		return lexical
	}
}

func splitXMLWhitespaceFields(lexical string) []string {
	return strings.FieldsFunc(lexical, isXMLWhitespaceRune)
}

// SplitXMLWhitespaceFields splits a string on XML whitespace (space, tab, CR, LF).
// It returns nil for empty input.
func SplitXMLWhitespaceFields(lexical string) []string {
	if lexical == "" {
		return nil
	}
	return splitXMLWhitespaceFields(lexical)
}

// TrimXMLWhitespace removes leading and trailing XML whitespace (space, tab, CR, LF).
func TrimXMLWhitespace(lexical string) string {
	return value.TrimXMLWhitespaceString(lexical)
}

// FieldsXMLWhitespaceSeq yields fields split on XML whitespace (space, tab, CR, LF).
// It is equivalent to strings.FieldsSeq for XML whitespace only.
func FieldsXMLWhitespaceSeq(lexical string) iter.Seq[string] {
	return func(yield func(string) bool) {
		i := 0
		for i < len(lexical) {
			for i < len(lexical) && value.IsXMLWhitespaceByte(lexical[i]) {
				i++
			}
			if i >= len(lexical) {
				return
			}
			start := i
			for i < len(lexical) && !value.IsXMLWhitespaceByte(lexical[i]) {
				i++
			}
			if !yield(lexical[start:i]) {
				return
			}
		}
	}
}

func isXMLWhitespaceRune(r rune) bool {
	if r < 0 || r > 0x7f {
		return false
	}
	return value.IsXMLWhitespaceByte(byte(r))
}
