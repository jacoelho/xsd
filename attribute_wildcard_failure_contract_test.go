package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestLocalAttributeWildcardIntersectionDiagnostic(t *testing.T) {
	t.Parallel()

	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:a" targetNamespace="urn:b">
  <xs:import namespace="urn:a"/>
  <xs:element name="r">
    <xs:complexType>
      <xs:attributeGroup ref="a:G"/>
      <xs:anyAttribute namespace="##other" processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	const imported = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a">
  <xs:attributeGroup name="G"><xs:anyAttribute namespace="##other" processContents="lax"/></xs:attributeGroup>
</xs:schema>`
	compile := func(source string) (*xsd.Engine, error) {
		return xsd.Compile(xsd.Bytes("local-wildcard.xsd", []byte(source)), xsd.Bytes("a.xsd", []byte(imported)))
	}
	engine, err := compile(root)
	if engine != nil {
		t.Fatal("invalid local wildcard intersection published a schema")
	}
	assertAttributeGroupCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel,
		"attribute wildcard intersection is not expressible", "local-wildcard.xsd", [2]int{6, 7})
	engine, err = compile(strings.Replace(root, "##other", "##any", 1))
	if err != nil {
		t.Fatalf("expressible local wildcard intersection: %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<b:r xmlns:b="urn:b" xmlns:c="urn:c" c:note="allowed"/>`)); err != nil {
		t.Fatalf("valid local wildcard: %v", err)
	}
}
