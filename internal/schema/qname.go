package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

const invalidQNameMessagePrefix = "invalid QName "

// qNameParts is a parsed lexical QName.
type qNameParts struct {
	Prefix   string
	Local    string
	Prefixed bool
}

// parseQNameParts parses and validates a lexical QName.
func parseQNameParts(lexical string) (qNameParts, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	parts := lex.SplitQName(lexical)
	if !parts.Valid {
		return qNameParts{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+diagnostictext.Quote(lexical))
	}
	return qNameParts{Prefix: parts.Prefix, Local: parts.Local, Prefixed: parts.Prefixed}, nil
}
