package schema_test

import (
	"errors"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestGlobalAttributeTypeRequiresSimpleType(t *testing.T) {
	t.Parallel()

	const invalid = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="C"/>
  <xs:complexType name="T">
    <xs:attribute name="a" type="C"/>
  </xs:complexType>
  <xs:element name="r" type="T"/>
</xs:schema>`
	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("kind.xsd", []byte(invalid))})
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != "unknown simple type C" {
		t.Fatalf("Compile(complex attribute type) = %v; want schema reference error for unknown simple type C", err)
	}
	if diagnostic.Path() != "kind.xsd" || diagnostic.Line() != 4 || diagnostic.Column() != 5 {
		t.Fatalf("diagnostic location = %s:%d:%d; want kind.xsd:4:5", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
	}

	const valid = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="C"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="T">
    <xs:attribute name="a" type="C"/>
  </xs:complexType>
  <xs:element name="r" type="T"/>
</xs:schema>`
	compiled, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("kind.xsd", []byte(valid))})
	if err != nil {
		t.Fatal(err)
	}
	mustValidate(t, compiled, `<r a="value"/>`)
}
