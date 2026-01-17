package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestQNameRequiresNamespaceBinding(t *testing.T) {
	t.Run("element_value", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:qname"
           xmlns:tns="urn:qname"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:QName"/>
</xs:schema>`

		docXML := `<?xml version="1.0"?>
<tns:root xmlns:tns="urn:qname">p:val</tns:root>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse schema: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations := validateStream(t, v, docXML)
		assertHasCode(t, violations, errors.ErrDatatypeInvalid)
	})

	t.Run("attribute_value", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:qname"
           xmlns:tns="urn:qname"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="q" type="xs:QName"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

		docXML := `<?xml version="1.0"?>
<tns:root xmlns:tns="urn:qname" q="p:val"/>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse schema: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations := validateStream(t, v, docXML)
		assertHasCode(t, violations, errors.ErrDatatypeInvalid)
	})

	t.Run("list_item", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:qname"
           xmlns:tns="urn:qname"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameList">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:QNameList"/>
</xs:schema>`

		docXML := `<?xml version="1.0"?>
<tns:root xmlns:tns="urn:qname">p:one p:two</tns:root>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse schema: %v", err)
		}

		v := New(mustCompile(t, schema))
		violations := validateStream(t, v, docXML)
		assertHasCode(t, violations, errors.ErrDatatypeInvalid)
	})
}

func assertHasCode(t *testing.T, violations []errors.Validation, code errors.ErrorCode) {
	t.Helper()
	if len(violations) == 0 {
		t.Fatalf("expected violations, got none")
	}
	for _, viol := range violations {
		if viol.Code == string(code) {
			return
		}
	}
	t.Fatalf("expected violation code %s, got: %v", code, violations)
}
