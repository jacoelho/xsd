package schema

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// schemaParseState is retained only for the parser-admission unit test that
// verifies schema text is recorded on the active semantic admission fact. The
// production parser is typedSchemaParseState in ast_typed.go.
type schemaParseFrame struct {
	node      *schemaSyntaxNode
	textBytes int64
}

type schemaParseState struct {
	stack  []schemaParseFrame
	limits Limits
}

func (s *schemaParseState) chars(t []byte, _ xmlstream.CharacterDataKind, line, col int) error {
	if err := checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		return nil
	}
	last := len(s.stack) - 1
	s.stack[last].textBytes += int64(len(t))
	if err := checkSchemaTokenLimit(s.stack[last].textBytes, s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if s.stack[last].node != nil && !lex.IsXMLWhitespaceBytes(t) {
		s.stack[last].node.hasNonWhitespaceText = true
	}
	return nil
}
