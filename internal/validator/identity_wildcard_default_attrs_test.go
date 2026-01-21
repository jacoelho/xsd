package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestIdentityWildcardIncludesDefaultAttributes(t *testing.T) {
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
            <xs:attribute name="code" type="xs:string" default="A"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref">
          <xs:complexType>
            <xs:attribute name="ref" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="codeKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@*"/>
    </xs:key>
    <xs:keyref name="codeRef" refer="tns:codeKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="@ref"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<tns:root xmlns:tns="urn:test">
  <tns:item/>
  <tns:ref ref="A"/>
</tns:root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
