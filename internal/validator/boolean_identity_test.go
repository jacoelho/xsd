package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestIdentityBooleanNormalizationTreatsTrueAndOneEqual(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:boolean" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueBool">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	document := `<root xmlns="urn:test"><item>true</item><item>1</item></root>`
	violations := validateStream(t, v, document)
	if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
		t.Fatalf("expected duplicate identity violation, got %v", violations)
	}
}

func TestIdentityBooleanNormalizationKeyRefMatches(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item">
          <xs:complexType>
            <xs:attribute name="flag" type="xs:boolean" use="required"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref">
          <xs:complexType>
            <xs:attribute name="flag" type="xs:boolean" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="boolKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@flag"/>
    </xs:key>
    <xs:keyref name="boolRef" refer="tns:boolKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="@flag"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	document := `<root xmlns="urn:test"><item flag="true"/><ref flag="1"/></root>`
	violations := validateStream(t, v, document)
	if hasViolationCode(violations, errors.ErrIdentityKeyRefFailed) {
		t.Fatalf("unexpected keyref violation, got %v", violations)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
