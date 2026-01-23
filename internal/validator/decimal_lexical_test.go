package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestDecimalRejectsExponentLexical(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:decimal"/>
</xs:schema>`

	docXML := `<root>1.2e3</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}
