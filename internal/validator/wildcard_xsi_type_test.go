package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestWildcardLaxXsiTypeAttributes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="HasReq">
    <xs:attribute name="req" type="xs:string" use="required"/>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xmlns:tns="urn:test">
  <item xsi:type="tns:HasReq"/>
</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) == 0 {
		t.Fatalf("expected attribute validation error")
	}

	found := false
	for _, v := range violations {
		if v.Code == string(errors.ErrRequiredAttributeMissing) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s violation, got: %v", errors.ErrRequiredAttributeMissing, violations)
	}
}

func TestWildcardLaxXsiTypeAbstract(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="AbstractType" abstract="true"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xmlns:tns="urn:test">
  <item xsi:type="tns:AbstractType"/>
</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) == 0 {
		t.Fatalf("expected xsi:type validation error")
	}

	found := false
	for _, v := range violations {
		if v.Code == string(errors.ErrXsiTypeInvalid) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s violation, got: %v", errors.ErrXsiTypeInvalid, violations)
	}
}
