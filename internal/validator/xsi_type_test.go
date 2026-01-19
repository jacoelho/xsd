package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestXsiTypeBlockedByRestrictionInChain(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           xmlns:tns="http://example.com/test">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="b" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="ext">
    <xs:complexContent>
      <xs:extension base="tns:base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="e" block="restriction"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<tns:root xmlns:tns="http://example.com/test"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <e xsi:type="tns:ext"><b/></e>
</tns:root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) == 0 {
		t.Fatalf("Expected xsi:type violation, got none")
	}

	found := false
	for _, viol := range violations {
		if viol.Code == string(errors.ErrXsiTypeInvalid) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrXsiTypeInvalid, violations)
	}
}

func TestXsiTypeInvalidQName(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:type="bad::Type">value</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) == 0 {
		t.Fatalf("Expected xsi:type violation, got none")
	}

	found := false
	for _, viol := range violations {
		if viol.Code == string(errors.ErrXsiTypeInvalid) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrXsiTypeInvalid, violations)
	}
}

func TestXsiTypeUnprefixedUsesDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns="urn:test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:type="T">value</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) > 0 {
		t.Fatalf("Expected no violations, got: %v", violations)
	}
}

func TestXsiTypeBuiltinIDDuplicate(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <a xsi:type="xs:ID">dup</a>
  <b xsi:type="xs:ID">dup</b>
</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrDuplicateID) {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrDuplicateID, violations)
	}
}

func TestXsiTypeBuiltinIDREFSUnresolved(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <ref xsi:type="xs:IDREFS">missing</ref>
</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrIDRefNotFound, violations)
	}
}
