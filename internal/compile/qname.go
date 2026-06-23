package compile

import (
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

const invalidQNameMessagePrefix = "invalid QName "

// QNameParts is a parsed lexical QName.
type QNameParts struct {
	Prefix   string
	Local    string
	Prefixed bool
}

// ParseQNameParts parses and validates a lexical QName.
func ParseQNameParts(lexical string) (QNameParts, error) {
	lexical = trimCompileXMLWhitespace(lexical)
	if lexical == "" {
		return QNameParts{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+lexical)
	}
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok {
		if !lex.IsNCName(lexical) {
			return QNameParts{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+lexical)
		}
		return QNameParts{Local: lexical}, nil
	}
	if prefix == "" || local == "" || strings.Contains(local, ":") || !lex.IsNCName(prefix) || !lex.IsNCName(local) {
		return QNameParts{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+lexical)
	}
	return QNameParts{Prefix: prefix, Local: local, Prefixed: true}, nil
}
