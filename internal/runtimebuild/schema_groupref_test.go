package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestBuildSchema_ResolvesGroupRefs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:group"
           xmlns:tns="urn:group"
           elementFormDefault="qualified">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="tns:G"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := BuildSchema(schema, BuildConfig{}); err != nil {
		t.Fatalf("BuildSchema error = %v", err)
	}
}
