package xsd

import (
	"strconv"
	"strings"
	"testing"
)

func TestValidateSequenceAttributesAndFacets(t *testing.T) {
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="SKU">
    <xs:restriction base="xs:string">
      <xs:pattern value="\d{3}-[A-Z]{2}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="order">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="qty">
                <xs:simpleType>
                  <xs:restriction base="xs:positiveInteger">
                    <xs:maxExclusive value="100"/>
                  </xs:restriction>
                </xs:simpleType>
              </xs:element>
            </xs:sequence>
            <xs:attribute name="sku" type="SKU" use="required"/>
            <xs:attribute name="country" type="xs:string" fixed="US"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompile(t, schema)
	mustValidate(t, engine, `<order><item sku="123-AA" country="US"><qty>2</qty></item></order>`)
	mustNotValidate(t, engine, `<order><item sku="123-AA"><qty>100</qty></item></order>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<order><item sku="bad"><qty>2</qty></item></order>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<order><item><qty>2</qty></item></order>`, ErrValidationAttribute)
}

func TestPatternFacetGroupsOrWithinStepAndAcrossDerivation(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:pattern value="red"/>
      <xs:pattern value="green"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:pattern value="gr.*"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="base" type="Base"/>
  <xs:element name="derived" type="Derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<base>red</base>`)
	mustValidate(t, engine, `<base>green</base>`)
	mustValidate(t, engine, `<derived>green</derived>`)
	mustNotValidate(t, engine, `<derived>red</derived>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<base>blue</base>`, ErrValidationFacet)
}

func TestCompatibleLengthFacetBoundsAreAllowed(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="v">
    <xs:simpleType>
      <xs:restriction base="xs:NMTOKENS">
        <xs:length value="2"/>
        <xs:minLength value="1"/>
        <xs:maxLength value="2"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<v>a b</v>`)
	mustNotValidate(t, engine, `<v>a</v>`, ErrValidationFacet)
}

func TestAttributeRestrictionMustRespectBaseWildcard(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:attribute name="local"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:anyAttribute namespace="##other"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:anyAttribute namespace="##any"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestAttributeRestrictionDoesNotInheritBaseWildcard(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other" processContents="skip"/></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent><xs:restriction base="base"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<root/>`)
	mustNotValidate(t, engine, `<root xmlns:f="urn:f" f:a="x"/>`, ErrValidationAttribute)
}

func TestComplexContentCannotExtendAllWithParticles(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:all><xs:element name="a"/></xs:all></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestAnyTypeAllowsAttributesAndChildren(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:anyType"/></xs:schema>`)
	mustValidate(t, engine, `<root custom="1"><anything other="2">text<nested/></anything></root>`)
}

func TestComplexExtensionInheritsAttributeWildcard(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="tns:Base"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test" xmlns:foo="urn:foo" foo:attr="1"><child>ok</child></root>`)
}

func TestSchemaNamesAllowXML10FifthEditionNameStartChars(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="〇10" type="xs:int"/>
      <xs:attribute name="〡20" type="xs:int"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root 〇10="1" 〡20="2"/>`)
}

func TestUnionRestrictionAllowsOnlyPatternAndEnumerationFacets(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="base">
    <xs:union memberTypes="xs:NMTOKEN xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="bad">
    <xs:restriction base="base">
      <xs:length value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)
}

func TestChoiceAllWildcardAndNil(t *testing.T) {
	choice := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	choiceEngine := mustCompile(t, choice)
	mustValidate(t, choiceEngine, `<root><b>x</b></root>`)
	mustNotValidate(t, choiceEngine, `<root><a>x</a><b>y</b></root>`, ErrValidationElement)
	mustNotValidate(t, choiceEngine, `<root/>`, ErrValidationContent)

	all := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="c" type="xs:string"/>
        <xs:element name="d" type="xs:string" minOccurs="0"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	allEngine := mustCompile(t, all)
	mustValidate(t, allEngine, `<root><d>y</d><c>z</c></root>`)
	mustNotValidate(t, allEngine, `<root><d>y</d></root>`, ErrValidationContent)

	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##other" processContents="skip" minOccurs="0"/>
        <xs:element name="empty" nillable="true" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompile(t, schema)
	mustValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><foreign xmlns="urn:f"/><empty xsi:nil="true"/></root>`)
	mustNotValidate(t, engine, `<root><local/><empty/></root>`, ErrValidationElement)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><empty xsi:nil="true">x</empty></root>`, ErrValidationNil)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><empty xsi:nil="true"> </empty></root>`, ErrValidationNil)

	skip := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="declared" nillable="false"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	skipEngine := mustCompile(t, skip)
	mustValidate(t, skipEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><declared xsi:nil="true">text<child/></declared></root>`)

	nilledRequired := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" nillable="true">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="required"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	nilledEngine := mustCompile(t, nilledRequired)
	mustValidate(t, nilledEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`)
	mustNotValidate(t, nilledEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"><required/></root>`, ErrValidationNil)

	notNillable := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string" nillable="false"/></xs:schema>`)
	mustNotValidate(t, notNillable, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="false">abc</root>`, ErrValidationNil)

	other := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:any namespace="##other" processContents="lax"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, other, `<root xmlns="urn:test"><local xmlns=""/></root>`, ErrValidationElement)
}

func TestStrictWildcardRequiresGlobalElementDeclaration(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="strict"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root><unknown/></root>`, ErrValidationElement)
	mustValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><unknown xsi:type="xs:string" xmlns:xs="http://www.w3.org/2001/XMLSchema">x</unknown></root>`)
}

func TestEmptyChoiceWithRequiredOccurrenceRejectsEmptyContent(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:choice/></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root/>`, ErrValidationContent)
}

func TestAnyAttributeRejectsOccurrenceAttributes(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:anyAttribute minOccurs="2"/></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:anyAttribute maxOccurs="2"/></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestChoiceWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:any namespace="##other"/>
        <xs:any namespace="urn:foreign"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestAllModelBeyondBitsetWidth(t *testing.T) {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:all>`)
	for i := range 70 {
		schema.WriteString(`<xs:element name="e`)
		schema.WriteString(strconv.Itoa(i))
		schema.WriteString(`" type="xs:string"/>`)
	}
	schema.WriteString(`</xs:all></xs:complexType></xs:element></xs:schema>`)
	engine := mustCompile(t, schema.String())

	var doc strings.Builder
	doc.WriteString(`<root>`)
	for i := 69; i >= 0; i-- {
		doc.WriteString(`<e`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`>x</e`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`>`)
	}
	doc.WriteString(`</root>`)
	mustValidate(t, engine, doc.String())

	var missing strings.Builder
	missing.WriteString(`<root>`)
	for i := range 69 {
		missing.WriteString(`<e`)
		missing.WriteString(strconv.Itoa(i))
		missing.WriteString(`>x</e`)
		missing.WriteString(strconv.Itoa(i))
		missing.WriteString(`>`)
	}
	missing.WriteString(`</root>`)
	mustNotValidate(t, engine, missing.String(), ErrValidationContent)
	mustNotValidate(t, engine, `<root><e0>x</e0><e0>y</e0></root>`, ErrValidationElement)
}

func TestAllCannotBeNestedInSequence(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:all><xs:element name="a"/></xs:all>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestModelGroupSyntaxIsValidated(t *testing.T) {
	tests := []struct {
		body string
		code ErrorCode
	}{
		{`<xs:complexType name="t"><xs:all name="bad"><xs:element name="a"/></xs:all></xs:complexType>`, ErrSchemaInvalidAttribute},
		{`<xs:complexType name="t"><xs:all minOccurs="0" maxOccurs="0"><xs:element name="a"/></xs:all></xs:complexType>`, ErrSchemaOccurrence},
		{`<xs:complexType name="t"><xs:all><xs:element name="a"/><xs:annotation/></xs:all></xs:complexType>`, ErrSchemaContentModel},
		{`<xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group><xs:complexType name="t"><xs:sequence><xs:group ref="g"/></xs:sequence></xs:complexType>`, ErrSchemaContentModel},
	}
	for _, test := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.body+`</xs:schema>`)))
		expectCode(t, err, test.code)
	}
}

func TestComplexContentExtensionFromEmptyAllBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group>
  <xs:complexType name="base"><xs:all/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="base"><xs:group ref="g"/></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestComplexContentExtensionCanUseAllWithEmptyBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"><xs:all minOccurs="0" maxOccurs="1"><xs:element name="a"/></xs:all></xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestRestrictionParticleOccurrenceMustBeSubset(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:sequence><xs:element name="a" minOccurs="0" maxOccurs="0"/></xs:sequence></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRestrictionParticleCountRangeMustBeSubset(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:sequence><xs:any/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="e"/><xs:element name="e"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a"/><xs:element name="a"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionChoiceCanMapToWildcardRange(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:any namespace="##any" minOccurs="2" maxOccurs="3"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:choice>
            <xs:element name="e1" minOccurs="2" maxOccurs="3"/>
            <xs:element name="e2" minOccurs="2" maxOccurs="2"/>
            <xs:any namespace="##other" minOccurs="2" maxOccurs="3"/>
          </xs:choice>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)
}

func TestRestrictionChoiceWildcardBranchAllowsElementSubset(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" xmlns:o="urn:other" elementFormDefault="qualified">
  <xs:import namespace="urn:other"/>
  <xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="local"/><xs:any namespace="##other" processContents="strict"/></xs:choice></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element ref="o:foreign"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="root" type="t:derived"/>
</xs:schema>`)),
		sourceBytes("other.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other" elementFormDefault="qualified"><xs:element name="foreign"/></xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns="urn:test" xmlns:o="urn:other"><o:foreign/></root>`)

	_, err = Compile(
		sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="local"/><xs:any namespace="##other"/></xs:choice></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element name="otherLocal"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)),
	)
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRestrictionRejectsWildcardForElementAndNillableLoosening(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" minOccurs="2" maxOccurs="10"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:any minOccurs="3" maxOccurs="9"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="e1" nillable="false"/><xs:element name="e2"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="e1" nillable="true"/><xs:element name="e2"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionChoiceBranchesMustMapToBaseBranches(t *testing.T) {
	tests := []string{
		`<xs:group name="G"><xs:choice><xs:element name="foo"/><xs:element name="bar"/></xs:choice></xs:group>
		 <xs:complexType name="base"><xs:choice><xs:element name="foo"/><xs:element name="test"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:group ref="G"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:choice maxOccurs="2"><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:any namespace="##any"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="c1"/><xs:element name="c2"/></xs:choice><xs:element name="d1"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="other"/><xs:element name="d1"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="c1"/><xs:element name="c2"/></xs:choice><xs:element name="foo"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:choice><xs:element name="c2"/><xs:element name="c1"/></xs:choice><xs:element name="foo"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for i, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionRepeatedSequenceCanMapToRepeatedChoiceBranch(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice maxOccurs="2"><xs:element name="a"/><xs:element name="b"/></xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:sequence maxOccurs="2"><xs:element name="a"/></xs:sequence></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)
}

func TestRestrictionChoiceBranchOccurrenceIsPreserved(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice>
        <xs:element name="c1" minOccurs="2" maxOccurs="4"/>
        <xs:element name="c2" minOccurs="0"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:choice>
            <xs:element name="c1" minOccurs="2" maxOccurs="2"/>
          </xs:choice>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)
}

func TestRestrictionOptionalElementCannotRestrictOptionalChoice(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice minOccurs="0">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a" type="xs:string" minOccurs="0"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRestrictionRepeatedOptionalElementCanRestrictRepeatedOptionalChoice(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="annotation" minOccurs="0"/>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="element"/>
        <xs:element name="any"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="t:base">
        <xs:sequence>
          <xs:element name="annotation" minOccurs="0"/>
          <xs:element name="element" minOccurs="0" maxOccurs="unbounded"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="t:derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><annotation/><element/></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><annotation/><element/><element/></root>`, ErrValidationContent)
}

func TestRestrictionSequenceToChoiceRequiresValidBranchMap(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1"/><xs:element name="e2"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="other"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for i, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionSequenceMappingSkipsOnlyEmptiableBaseParticles(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)

	_, err := Compile(
		sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" xmlns:i="urn:imported">
  <xs:import namespace="urn:imported"/>
  <xs:complexType name="base"><xs:sequence><xs:element name="e1" maxOccurs="3"/><xs:element name="e2" minOccurs="0" maxOccurs="3"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element ref="i:e1"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="doc" type="t:derived"/>
</xs:schema>`)),
		sourceBytes("imp.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:imported"><xs:element name="e1"/></xs:schema>`)),
	)
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRestrictionSequenceToAllRequiresValidParticleMap(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:all>
      <xs:element name="a0" minOccurs="0"/>
      <xs:element name="a1"/>
      <xs:element name="a2" minOccurs="0"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:sequence><xs:element name="a1"/></xs:sequence></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	tests := []string{
		`<xs:complexType name="base"><xs:all><xs:element name="a1"/><xs:element name="a2" minOccurs="0"/></xs:all></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="a1" minOccurs="2" maxOccurs="2"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:all minOccurs="0"><xs:element name="a1"/><xs:element name="a2" minOccurs="0"/></xs:all></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="a1" minOccurs="0"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for i, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionAllCannotRestrictMultiParticleSequence(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="e1"/><xs:element name="e2"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:all><xs:element name="e1"/><xs:element name="e2"/></xs:all></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestNestedChoiceGroupOccurrenceIsNotDoubleCounted(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:element name="foo"/>
  <xs:group name="G"><xs:choice><xs:element ref="t:foo" maxOccurs="3"/></xs:choice></xs:group>
  <xs:element name="doc">
    <xs:complexType>
      <xs:choice><xs:group ref="t:G"/></xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<doc xmlns="urn:test"><foo/><foo/><foo/></doc>`)
}

func TestSingleParticleNestedSequenceOccurrenceIsFlattened(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:element name="doc">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="foo"/>
        <xs:sequence minOccurs="0" maxOccurs="1"><xs:element name="foo" maxOccurs="2"/></xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<doc xmlns="urn:test"><foo/><foo/><foo/></doc>`)
}

func TestSingleBranchNestedChoiceOccurrenceIsFlattened(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:element name="doc">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="foo"/>
        <xs:choice><xs:element name="e1" maxOccurs="3"/></xs:choice>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<doc xmlns="urn:test"><foo/><e1/><e1/><e1/></doc>`)
}

func TestNestedChoiceBranchOccurrencesInsideSequence(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="foo"/>
        <xs:choice>
          <xs:element name="a" minOccurs="2" maxOccurs="3"/>
          <xs:element name="b"/>
        </xs:choice>
        <xs:element name="bar"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><foo/><a/><a/><bar/></root>`)
	mustValidate(t, engine, `<root><foo/><a/><a/><a/><bar/></root>`)
	mustValidate(t, engine, `<root><foo/><b/><bar/></root>`)
	mustNotValidate(t, engine, `<root><foo/><a/><bar/></root>`, ErrValidationElement)
}

func TestExtensionUPAChecksRepeatableModelRefTerms(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base"><xs:choice><xs:any namespace="##targetNamespace" maxOccurs="3"/></xs:choice></xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="t:base"><xs:choice><xs:element name="c1"/><xs:element name="c2"/></xs:choice></xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestAllCanContainOnlyElementParticles(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:all><xs:sequence><xs:element name="a"/></xs:sequence></xs:all></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestChoiceUPAChecksGroupSequenceFirstElement(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
  <xs:complexType name="t"><xs:choice><xs:element name="a"/><xs:group ref="g"/></xs:choice></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRequiredGroupRefWithEmptiableChoiceCanBeAbsent(t *testing.T) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test" elementFormDefault="qualified">
  <xs:element name="Root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="A"/>
        <xs:group ref="B" maxOccurs="5"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:group name="B">
    <xs:choice>
      <xs:element name="B1" minOccurs="0"/>
      <xs:element name="B2" minOccurs="0"/>
    </xs:choice>
  </xs:group>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
	mustValidate(t, engine, `<Root xmlns="urn:test"><A/></Root>`)
}

func TestTopLevelGroupCompositorCannotHaveOccurs(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:choice maxOccurs="2"><xs:element name="a"/></xs:choice></xs:group>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:choice><xs:element name="a"/></xs:choice><xs:sequence><xs:element name="b"/></xs:sequence></xs:group>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="a" maxOccurs="2"/></xs:all></xs:group>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)
}

func TestTopLevelGroupDeclarationAttributesAreValidated(t *testing.T) {
	tests := []struct {
		group string
		code  ErrorCode
	}{
		{`<xs:group ref="g"/>`, ErrSchemaInvalidAttribute},
		{`<xs:group><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, ErrSchemaReference},
		{`<xs:group name="g" minOccurs="1"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, ErrSchemaInvalidAttribute},
		{`<xs:group name="g" maxOccurs="1"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, ErrSchemaInvalidAttribute},
	}
	for _, test := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.group+`</xs:schema>`)))
		expectCode(t, err, test.code)
	}
}

func TestTopLevelGroupDeclarationChildrenAreValidated(t *testing.T) {
	tests := []string{
		`<xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence><xs:annotation/></xs:group>`,
		`<xs:group name="g"><xs:annotation/><xs:annotation/><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`,
		`<xs:group name="g"><xs:group ref="other"/></xs:group><xs:group name="other"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`,
	}
	for _, group := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+group+`</xs:schema>`)))
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestAnyParticleCanContainOnlyAnnotation(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:any>
          <xs:group ref="g"/>
        </xs:any>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:sequence>
      <xs:any><xs:group ref="other"/></xs:any>
    </xs:sequence>
  </xs:group>
  <xs:group name="other"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestDirectGroupRefOccurrenceIsValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/><xs:element name="b"/></xs:sequence></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="2" maxOccurs="4"/></xs:complexType></xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a/><b/><a/><b/></r>`)
	mustNotValidate(t, engine, `<r><a/><b/></r>`, ErrValidationContent)
	mustNotValidate(t, engine, `<r><a/><b/><a/></r>`, ErrValidationContent)
}

func TestExtensionGroupRefOccurrenceIsPreserved(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="x"/></xs:sequence></xs:complexType>
  <xs:group name="choices"><xs:choice><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:group>
  <xs:element name="r">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="base">
          <xs:group ref="choices" minOccurs="0" maxOccurs="3"/>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><x/></r>`)
	mustValidate(t, engine, `<r><x/><a/><b/><a/></r>`)
	mustNotValidate(t, engine, `<r><x/><a/><b/><a/><b/></r>`, ErrValidationElement)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="x"/></xs:sequence></xs:complexType>
  <xs:group name="choices"><xs:choice><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:group>
  <xs:element name="r">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="base">
          <xs:group ref="choices" minOccurs="0" maxOccurs="0"/>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><x/></r>`)
	mustNotValidate(t, engine, `<r><x/><a/></r>`, ErrValidationElement)
}

func TestRepeatedSequenceCanStartNextOccurrenceAfterMinimumSatisfied(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence minOccurs="2" maxOccurs="2">
        <xs:element name="a" minOccurs="1" maxOccurs="2"/>
        <xs:element name="b" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a/><a/><b/></r>`)
}

func TestChoiceKeepsSelectedBranch(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string" minOccurs="2" maxOccurs="2"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><a>x</a><a>y</a></root>`)
	mustValidate(t, engine, `<root><b>x</b></root>`)
	mustNotValidate(t, engine, `<root><a>x</a></root>`, ErrValidationContent)
	mustNotValidate(t, engine, `<root><a>x</a><b>y</b></root>`, ErrValidationElement)

	repeating := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, repeating, `<root><a>x</a><b>y</b><a>z</a></root>`)
	mustNotValidate(t, repeating, `<root/>`, ErrValidationContent)
}

func TestInvalidOccurrenceIsSchemaCompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" minOccurs="2" maxOccurs="1"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="1" maxOccurs="0"/></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)
}

func TestLargeOccurrenceValuesAreSchemaValid(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:choice><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="12678967543233" maxOccurs="12678967543234"/></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestCompileOptionsNameAndOccurrenceLimits(t *testing.T) {
	_, err := CompileWithOptions(CompileOptions{MaxSchemaNames: 1}, sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)

	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" maxOccurs="11"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	_, err = CompileWithOptions(CompileOptions{MaxFiniteOccurs: 10}, sourceBytes("schema.xsd", []byte(schema)))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)

	boundary := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" maxOccurs="10"/><xs:element name="b" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	if _, err := CompileWithOptions(CompileOptions{MaxFiniteOccurs: 10}, sourceBytes("schema.xsd", []byte(boundary))); err != nil {
		t.Fatalf("CompileWithOptions() maxOccurs boundary error = %v", err)
	}
}

func TestNestedChoiceModelGroup(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:choice maxOccurs="unbounded">
          <xs:element name="a" type="xs:string"/>
          <xs:element name="b" type="xs:string"/>
        </xs:choice>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a>x</a><b>y</b><a>z</a></r>`)
}

func TestNestedSequenceModelGroupFlattens(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a>x</a><b>y</b><c>z</c></r>`)
	mustNotValidate(t, engine, `<r><a>x</a><c>z</c></r>`, ErrValidationElement)
}

func TestNestedSequenceModelGroupOccurrenceIsValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence minOccurs="0" maxOccurs="2">
          <xs:element name="a" type="xs:string"/>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><c>z</c></r>`)
	mustValidate(t, engine, `<r><a>x</a><b>y</b><c>z</c></r>`)
	mustValidate(t, engine, `<r><a>x</a><b>y</b><a>x</a><b>y</b><c>z</c></r>`)
	mustNotValidate(t, engine, `<r><a>x</a><c>z</c></r>`, ErrValidationElement)
	mustNotValidate(t, engine, `<r><b>y</b><c>z</c></r>`, ErrValidationElement)
	mustNotValidate(t, engine, `<r><a>x</a><b>y</b><a>x</a><b>y</b><a>x</a><b>y</b><c>z</c></r>`, ErrValidationElement)
}

func TestChoiceBranchCanBeSequenceModelGroup(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
        <xs:element name="c" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a>x</a><b>y</b></r>`)
	mustValidate(t, engine, `<r><c>z</c></r>`)
	mustNotValidate(t, engine, `<r><a>x</a></r>`, ErrValidationContent)
	mustNotValidate(t, engine, `<r><a>x</a><c>z</c></r>`, ErrValidationElement)
}

func TestDirectRecursiveModelGroupsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:sequence>
      <xs:group ref="g"/>
    </xs:sequence>
  </xs:group>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence><xs:group ref="g"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="a">
    <xs:choice>
      <xs:group ref="b"/>
    </xs:choice>
  </xs:group>
  <xs:group name="b">
    <xs:sequence>
      <xs:group ref="a"/>
    </xs:sequence>
  </xs:group>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence><xs:group ref="a"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestRecursiveModelGroupsThroughElements(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:sequence>
      <xs:element name="a">
        <xs:complexType>
          <xs:sequence>
            <xs:group ref="g" minOccurs="0"/>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:group>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence><xs:group ref="g"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a/></r>`)
	mustValidate(t, engine, `<r><a><a/></a></r>`)
	mustNotValidate(t, engine, `<r/>`, ErrValidationContent)
	mustNotValidate(t, engine, `<r><a><b/></a></r>`, ErrValidationElement)
}

func TestRecursiveAttributeGroupsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="a">
    <xs:attributeGroup ref="a"/>
  </xs:attributeGroup>
  <xs:element name="r">
    <xs:complexType>
      <xs:attributeGroup ref="a"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestRepeatingChoiceWithRepeatedBranchPartitionsAtClose(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="a" minOccurs="3" maxOccurs="5"/>
        <xs:element name="b" minOccurs="3" maxOccurs="5"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r><a/><a/><a/><a/><a/><a/></r>`)
	mustValidate(t, engine, `<r><a/><a/><a/><b/><b/><b/></r>`)
	mustNotValidate(t, engine, `<r><a/><a/></r>`, ErrValidationContent)
}

func TestRepeatingChoiceRestrictionWithDerivedChoiceValidates(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="c1"/>
        <xs:element name="c2"/>
        <xs:element name="c3" maxOccurs="unbounded"/>
      </xs:choice>
      <xs:element name="tail"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="restricted">
    <xs:complexContent>
      <xs:restriction base="t:base">
        <xs:sequence>
          <xs:choice maxOccurs="unbounded">
            <xs:element name="c1"/>
            <xs:element name="c2"/>
            <xs:element name="c3" maxOccurs="unbounded"/>
          </xs:choice>
          <xs:element name="tail"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="t:restricted"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><c3/><c3/><tail/></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><tail/></root>`, ErrValidationElement)
}

func TestSequenceParticleCannotRestrictElementParticle(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test">
  <xs:complexType name="base">
    <xs:choice minOccurs="2" maxOccurs="unbounded">
      <xs:element name="e1" minOccurs="0" maxOccurs="10"/>
      <xs:element name="e2" minOccurs="0"/>
    </xs:choice>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="t:base">
          <xs:choice minOccurs="2" maxOccurs="unbounded">
            <xs:sequence maxOccurs="2">
              <xs:element name="e1" maxOccurs="2"/>
            </xs:sequence>
            <xs:element name="e2"/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestChoiceDuplicateElementIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="a" type="xs:int"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRepeatingChoiceWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice maxOccurs="10">
        <xs:any namespace="urn:other" processContents="lax"/>
        <xs:any namespace="urn:other" processContents="strict"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestSequenceRepeatedElementCanBeUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:element name="a" maxOccurs="2"/>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestSequenceWildcardElementOverlapIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:any namespace="##any" maxOccurs="unbounded"/>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestSequenceWildcardWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:any namespace="##other" minOccurs="0"/>
      <xs:any namespace="##other"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRepeatingSequenceWildcardProcessOnlyOverlapCompiles(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ok">
    <xs:sequence maxOccurs="10">
      <xs:any namespace="urn:other" processContents="strict"/>
      <xs:any namespace="urn:other" processContents="lax"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestAllParticleCannotRepeat(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string" maxOccurs="2"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaOccurrence)
}

func TestComplexExtensionUnionsAttributeWildcards(t *testing.T) {
	engine, err := Compile(
		sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:f="urn:f">
  <xs:import namespace="urn:f" schemaLocation="foreign.xsd"/>
  <xs:complexType name="base"><xs:anyAttribute namespace="##local"/></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"><xs:anyAttribute namespace="##other"/></xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
  <xs:attribute name="local" type="xs:string"/>
</xs:schema>`)),
		sourceBytes("foreign.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:f">
  <xs:attribute name="foreign" type="xs:string"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xmlns:f="urn:f" local="a" f:foreign="b"/>`)
}

func TestAttributeGroupWildcardsIntersect(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="a">
    <xs:anyAttribute namespace="urn:a" processContents="skip"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="b">
    <xs:anyAttribute namespace="urn:b" processContents="lax"/>
  </xs:attributeGroup>
  <xs:element name="root">
    <xs:complexType>
      <xs:attributeGroup ref="a"/>
      <xs:attributeGroup ref="b"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root/>`)
	mustNotValidate(t, engine, `<root xmlns:a="urn:a" a:x="1"/>`, ErrValidationAttribute)
	mustNotValidate(t, engine, `<root xmlns:b="urn:b" b:x="1"/>`, ErrValidationAttribute)
}

func TestAttributeWildcardUnionMustBeExpressible(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" xmlns:a="urn:a">
  <xs:complexType name="base">
    <xs:anyAttribute namespace="##other"/>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="a:base">
        <xs:anyAttribute namespace="##local urn:b"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}
