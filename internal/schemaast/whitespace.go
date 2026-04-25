package schemaast

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

// TrimXMLWhitespace trims XML whitespace according to the active normalization mode.
var TrimXMLWhitespace = value.TrimXMLWhitespaceString

// FieldsXMLWhitespaceSeq yields fields split on XML whitespace (space, tab, CR, LF).
// It is equivalent to strings.FieldsSeq for XML whitespace only.
func FieldsXMLWhitespaceSeq(lexical string) iter.Seq[string] {
	return value.FieldsXMLWhitespaceStringSeq(lexical)
}
