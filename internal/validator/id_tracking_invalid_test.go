package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestIDREFAttributeInvalidValueNotTracked(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="ref" type="xs:IDREF"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root ref="a b"/>`)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
	if hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("unexpected IDREF violation, got %v", violations)
	}
}

func TestInvalidIDValuesDoNotTriggerDuplicateID(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root><item id="a b"/><item id="a b"/></root>`)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
	if hasViolationCode(violations, errors.ErrDuplicateID) {
		t.Fatalf("unexpected duplicate ID violation, got %v", violations)
	}
}

func TestIDREFSInvalidListItemNotTracked(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:IDREFS"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root>1bad</root>`)
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
	if hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("unexpected IDREF violation, got %v", violations)
	}
}
