package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// checkSchemaLocationHint checks the syntax of one XSI schema-location hint
// value: xsi:schemaLocation is a list of anyURI (any item count),
// xsi:noNamespaceSchemaLocation one anyURI. It owns the hint syntax
// diagnostic. Identity capture evaluates the same attributes for their key
// through the anyURI value program, which applies the same uriref.Check rule
// (internal/value/string.go), so the verdicts cannot diverge. Hints never take
// part in component selection and no hint state is retained;
// MaxInstanceTokenBytes bounds each hint attribute. local must be one of the
// two hint names.
func checkSchemaLocationHint(local, value string) error {
	if local == vocab.XSIAttrSchemaLocation {
		for field := range lex.XMLFieldsSeq(value) {
			if _, err := uriref.Check(field); err != nil {
				return xsderrors.Validation(xsderrors.CodeValidationAttribute, invalidURIMessage("xsi:schemaLocation", field), nil)
			}
		}
		return nil
	}
	return checkNoNamespaceSchemaLocation(value)
}

func checkNoNamespaceSchemaLocation(value string) error {
	value = lex.TrimXMLWhitespaceString(value)
	if _, err := uriref.Check(value); err != nil {
		return xsderrors.Validation(xsderrors.CodeValidationAttribute, invalidURIMessage("xsi:noNamespaceSchemaLocation", value), nil)
	}
	return nil
}

// isSchemaLocationHintName reports whether name is an XSI schema-location hint.
func isSchemaLocationHintName(name xml.Name) bool {
	return name.Space == vocab.XSINamespaceURI &&
		(name.Local == vocab.XSIAttrSchemaLocation || name.Local == vocab.XSIAttrNoNamespaceSchemaLocation)
}
