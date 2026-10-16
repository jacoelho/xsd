package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestImportedAttributeGroupsRejectUnexpressibleWildcardIntersection(t *testing.T) {
	t.Parallel()

	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:a" xmlns:b="urn:b">
  <xs:import namespace="urn:a"/>
  <xs:import namespace="urn:b"/>
  <xs:element name="r">
    <xs:complexType>
      <xs:attributeGroup ref="a:G"/>
      <xs:attributeGroup ref="b:G"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	const groupA = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a">
  <xs:attributeGroup name="G"><xs:anyAttribute namespace="##other" processContents="lax"/></xs:attributeGroup>
</xs:schema>`
	const groupB = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b">
  <xs:attributeGroup name="G"><xs:anyAttribute namespace="##other" processContents="lax"/></xs:attributeGroup>
</xs:schema>`
	compile := func(second string) (*xsd.Engine, error) {
		return xsd.Compile(xsd.Bytes("root.xsd", []byte(root)), xsd.Bytes("a.xsd", []byte(groupA)), xsd.Bytes("b.xsd", []byte(second)))
	}
	engine, err := compile(groupB)
	if engine != nil {
		t.Fatal("invalid wildcard intersection published a schema")
	}
	assertAttributeGroupCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel,
		"attribute wildcard intersection is not expressible", "root.xsd", [2]int{7, 7})
	if _, err := compile(strings.Replace(groupB, "##other", "##any", 1)); err != nil {
		t.Fatalf("expressible wildcard intersection failed: %v", err)
	}
}

func TestUnusedAttributeGroupRejectsInvalidDefaultOverride(t *testing.T) {
	t.Parallel()

	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:int" default="1"/>
  <xs:attributeGroup name="G">
    <xs:attribute ref="a" default="bad"/>
  </xs:attributeGroup>
  <xs:element name="r" type="xs:string"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("unused.xsd", []byte(source)))
	if engine != nil {
		t.Fatal("invalid unused attribute group published a schema")
	}
	assertAttributeGroupCompileDiagnostic(t, err, xsderrors.CodeSchemaFacet,
		"invalid attribute default value for a", "unused.xsd", [2]int{4, 5})
	if _, err := xsd.Compile(xsd.Bytes("unused.xsd", []byte(strings.Replace(source, `default="bad"`, `default="3"`, 1)))); err != nil {
		t.Fatalf("valid unused attribute group default failed: %v", err)
	}
}

func assertAttributeGroupCompileDiagnostic(t *testing.T, err error, code xsderrors.Code, message, path string, location [2]int) {
	t.Helper()
	line, column := location[0], location[1]
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("Compile() error = %v, want structured diagnostic", err)
	}
	if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != code || diagnostic.Message() != message || diagnostic.Path() != path || diagnostic.Line() != line || diagnostic.Column() != column || diagnostic.Cause() != nil {
		t.Fatalf("Compile() diagnostic = %s/%s %q at %s:%d:%d cause=%v, want schema compile/%s %q at %s:%d:%d with no cause",
			diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), diagnostic.Cause(), code, message, path, line, column)
	}
}
