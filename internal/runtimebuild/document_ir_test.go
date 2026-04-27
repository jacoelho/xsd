package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/schemair"
)

func TestBuildFromDocumentSetIRCoreSchema(t *testing.T) {
	result, err := schemaast.ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:maxLength value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="code" type="tns:Code"/>
    </xs:sequence>
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:complexType>
  <xs:element name="root" type="tns:RootType"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	ir, err := schemair.Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*result.Document}}, schemair.ResolveConfig{})
	if err != nil {
		t.Fatalf("resolve document set: %v", err)
	}
	rt, err := Build(Input{Schema: ir})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if rt == nil || rt.ElementCount() < 2 {
		t.Fatalf("runtime elements = %d, want root element", rt.ElementCount())
	}
}
