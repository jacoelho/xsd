package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestDateTimeTimezonePresenceAffectsEnumeration(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:date">
        <xs:enumeration value="2000-01-01Z"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	docXML := `<root>2000-01-01</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrFacetViolation) {
		t.Fatalf("expected facet violation, got: %v", violations)
	}
}
