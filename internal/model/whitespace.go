package model

import (
	"iter"

	"github.com/jacoelho/xsd/internal/value"
)

// WhiteSpace represents whitespace normalization
type WhiteSpace int

const (
	WhiteSpacePreserve WhiteSpace = iota
	WhiteSpaceReplace
	WhiteSpaceCollapse
)

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

// TrimXMLWhitespace trims XML whitespace according to the active normalization mode.
var TrimXMLWhitespace = value.TrimXMLWhitespaceString

// FieldsXMLWhitespaceSeq yields fields split on XML whitespace (space, tab, CR, LF).
// It is equivalent to strings.FieldsSeq for XML whitespace only.
func FieldsXMLWhitespaceSeq(lexical string) iter.Seq[string] {
	return value.FieldsXMLWhitespaceStringSeq(lexical)
}
