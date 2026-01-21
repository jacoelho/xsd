package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestListDerivedIDREFsTracking(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="RefList">
    <xs:list itemType="xs:IDREF"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="id" type="xs:ID"/>
        <xs:element name="refs" type="RefList"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, `<root><id>good</id><refs>missing</refs></root>`)
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
