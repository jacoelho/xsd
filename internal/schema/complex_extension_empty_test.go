package schema_test

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestComplexContentExtensionEmptyModelInheritsBaseContent(t *testing.T) {
	t.Parallel()

	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="MixedBase" mixed="true">
    <xs:sequence><xs:element name="baseChild"/></xs:sequence>
    <xs:attribute name="baseAttr" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="MixedDerived">
    <xs:complexContent><xs:extension base="MixedBase">
      <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="ignored"/></xs:sequence>
      <xs:attribute name="derivedAttr" use="required"/>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="mixed" type="MixedDerived"/>

  <xs:complexType name="AllBase">
    <xs:all><xs:element name="allChild"/></xs:all>
  </xs:complexType>
  <xs:complexType name="AllDerived">
    <xs:complexContent><xs:extension base="AllBase">
      <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="ignored"/></xs:sequence>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="all" type="AllDerived"/>

  <xs:complexType name="SimpleBase">
    <xs:simpleContent><xs:extension base="xs:int"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="SimpleDerived">
    <xs:complexContent><xs:extension base="SimpleBase">
      <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="ignored"/></xs:sequence>
      <xs:attribute name="extra" use="required"/>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="simple" type="SimpleDerived"/>
</xs:schema>`)

	mustValidate(t, engine, `<mixed baseAttr="base" derivedAttr="derived">text<baseChild/></mixed>`)
	mustNotValidate(t, engine, `<mixed baseAttr="base" derivedAttr="derived"><ignored/></mixed>`, xsderrors.CodeValidationElement)
	mustValidate(t, engine, `<all><allChild/></all>`)
	mustNotValidate(t, engine, `<all><ignored/></all>`, xsderrors.CodeValidationElement)
	mustValidate(t, engine, `<simple extra="yes">42</simple>`)
	mustNotValidate(t, engine, `<simple>42</simple>`, xsderrors.CodeValidationAttribute)
	mustNotValidate(t, engine, `<simple extra="yes">bad</simple>`, xsderrors.CodeValidationFacet)
}

func TestComplexContentExtensionEmptyModelPreservesMixedAndRequiredChoice(t *testing.T) {
	t.Parallel()

	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Empty"/>
  <xs:complexType name="MixedSynthetic">
    <xs:complexContent mixed="true"><xs:extension base="Empty">
      <xs:sequence minOccurs="0" maxOccurs="0"/>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="mixed" type="MixedSynthetic"/>
  <xs:complexType name="RequiredChoice">
    <xs:complexContent><xs:extension base="Empty"><xs:choice/></xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="choice" type="RequiredChoice"/>
</xs:schema>`)

	mustValidate(t, engine, `<mixed>text</mixed>`)
	mustValidate(t, engine, `<mixed/>`)
	mustNotValidate(t, engine, `<choice/>`, xsderrors.CodeValidationContent)
	mustNotValidate(t, engine, `<choice>text</choice>`, xsderrors.CodeValidationText)
}

func TestComplexContentExtensionEmptyModelValidatesSyntaxAndReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		code   xsderrors.Code
	}{
		{
			name: "missing element reference",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element ref="missing"/></xs:sequence>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference,
		},
		{
			name: "missing group reference",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:group ref="missing" minOccurs="0" maxOccurs="0"/>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference,
		},
		{
			name: "invalid all occurrence",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:all minOccurs="0" maxOccurs="0"/>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaOccurrence,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
				source.Bytes("schema.xsd", []byte(tt.schema)),
			})
			if err == nil {
				t.Fatal("Compile() unexpectedly succeeded")
			}
			expectCode(t, err, tt.code)
		})
	}
}

func TestComplexContentExtensionEmptyModelKeepsAdmissionAndFinalChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		code   xsderrors.Code
	}{
		{
			name: "all base rejects nonempty extension",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:all><xs:element name="a"/></xs:all></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:sequence><xs:element name="b"/></xs:sequence>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "simple base rejects nonempty extension",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:sequence><xs:element name="b"/></xs:sequence>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "final base rejects empty extension",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base" final="extension"/>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base">
    <xs:sequence minOccurs="0" maxOccurs="0"/>
  </xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`,
			code: xsderrors.CodeSchemaReference,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
				source.Bytes("schema.xsd", []byte(tt.schema)),
			})
			if err == nil {
				t.Fatal("Compile() unexpectedly succeeded")
			}
			expectCode(t, err, tt.code)
		})
	}
}
