package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
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

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root/>`)
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

func TestListDerivedIDREFTracking(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="RefList">
    <xs:list itemType="xs:IDREF"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="refs" type="RefList"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root refs="missing"/>`)
	if !hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("expected IDREF violation, got %v", violations)
	}
}
