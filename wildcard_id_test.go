package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestWildcardIDUsesEffectiveDeclaredTypes(t *testing.T) {
	t.Parallel()

	const declarations = `
  <xs:simpleType name="DerivedID"><xs:restriction base="xs:ID"/></xs:simpleType>
  <xs:simpleType name="IDOrInt"><xs:union memberTypes="xs:ID xs:int"/></xs:simpleType>
  <xs:attribute name="a" type="DerivedID"/>
  <xs:attributeGroup name="IDs"><xs:attribute ref="a"/></xs:attributeGroup>
  <xs:complexType name="Base"><xs:attribute name="a" type="DerivedID"/><xs:anyAttribute processContents="strict"/></xs:complexType>`
	for _, test := range []struct {
		name      string
		wildType  string
		rootType  string
		attribute string
		wantError bool
	}{
		{
			name: "strict absent declared ID", wildType: "xs:ID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "lax absent declared ID", wildType: "xs:ID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="lax"/>`,
		},
		{
			name: "derived declared ID", wildType: "xs:ID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:attribute name="a" type="DerivedID"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "derived wildcard ID", wildType: "DerivedID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "group reference", wildType: "xs:ID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:attributeGroup ref="IDs"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "inherited use", wildType: "xs:ID", attribute: `b="B"`, wantError: true,
			rootType: `<xs:complexContent><xs:extension base="Base"/></xs:complexContent>`,
		},
		{
			name: "restriction removes use", wildType: "xs:ID", attribute: `b="B"`,
			rootType: `<xs:complexContent><xs:restriction base="Base"><xs:attribute name="a" use="prohibited"/><xs:anyAttribute processContents="strict"/></xs:restriction></xs:complexContent>`,
		},
		{
			name: "prohibited source has no use", wildType: "xs:ID", attribute: `b="B"`,
			rootType: `<xs:attribute name="a" type="xs:ID" use="prohibited"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "no declared ID use", wildType: "xs:ID", attribute: `b="B"`,
			rootType: `<xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "skip does not assess wildcard declaration", wildType: "xs:ID", attribute: `b="B"`,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="skip"/>`,
		},
		{
			name: "unresolved lax has no ID declaration", wildType: "xs:ID", attribute: `unknown="B"`,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="lax"/>`,
		},
		{
			name: "declared union is not ID derived", wildType: "xs:ID", attribute: `b="B"`,
			rootType: `<xs:attribute name="a" type="IDOrInt"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "wildcard union is not ID derived", wildType: "IDOrInt", attribute: `b="B"`,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="strict"/>`,
		},
		{
			name: "known non-ID wildcard declaration", wildType: "xs:int", attribute: `b="7"`,
			rootType: `<xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="strict"/>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + declarations +
				`<xs:attribute name="b" type="` + test.wildType + `"/>` +
				`<xs:element name="root"><xs:complexType>` + test.rootType + `</xs:complexType></xs:element></xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes("wildcard-id.xsd", []byte(schema)))
			if err != nil {
				t.Fatal(err)
			}
			err = engine.Validate(strings.NewReader(`<root ` + test.attribute + `/>`))
			if !test.wantError {
				if err != nil {
					t.Fatalf("valid wildcard attribute rejected: %v", err)
				}
				return
			}
			var diagnostic *xsderrors.Error
			if !errors.As(err, &diagnostic) || diagnostic.Code() != xsderrors.CodeValidationAttribute {
				t.Fatalf("Validate() = %v, want validation.attribute for wildcard ID conflicting with declared use", err)
			}
		})
	}
}

func TestRejectedWildcardIDDoesNotPublishIdentity(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="b" type="xs:ID"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="bad"><xs:complexType><xs:attribute name="a" type="xs:ID"/><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
      <xs:element name="good"><xs:complexType><xs:attribute name="b" type="xs:ID" use="required"/></xs:complexType></xs:element>
      <xs:element name="ref"><xs:complexType><xs:attribute name="to" type="xs:IDREF" use="required"/></xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
    <xs:unique name="ids"><xs:selector xpath="bad | good"/><xs:field xpath="@b"/></xs:unique>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("wildcard-id-recovery.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 10})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		err = session.Validate(strings.NewReader(`<root><bad b="B"/><good b="B"/><ref to="B"/></root>`))
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) != 1 {
			t.Fatalf("Validate() diagnostics = %v, want only the wildcard ID conflict", diagnostics)
		}
		var diagnostic *xsderrors.Error
		if !errors.As(diagnostics[0], &diagnostic) || diagnostic.Code() != xsderrors.CodeValidationAttribute {
			t.Fatalf("Validate() = %v, want validation.attribute without leaked ID or unique-field registration", err)
		}
		if err := session.Validate(strings.NewReader(`<root><bad/><good b="B"/><ref to="B"/></root>`)); err != nil {
			t.Fatalf("valid document after wildcard rejection: %v", err)
		}
	}
}
