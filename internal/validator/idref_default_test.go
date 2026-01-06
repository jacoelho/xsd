package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xml"
)

func TestDefaultIDREFSAttributeValidation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="drefs" type="xs:IDREFS" default="abc"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	doc, err := xml.Parse(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatalf("Parse XML: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := v.Validate(doc)
	if len(violations) == 0 {
		t.Fatalf("Expected IDREF violation, got none")
	}

	found := false
	for _, viol := range violations {
		if viol.Code == string(errors.ErrIDRefNotFound) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrIDRefNotFound, violations)
	}
}
