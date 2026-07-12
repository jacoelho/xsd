package compile_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
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
	engine := mustCompileRuntime(t, schema)
	mustValidateRuntime(t, engine, `<order><item sku="123-AA" country="US"><qty>2</qty></item></order>`)
	mustNotValidateRuntime(t, engine, `<order><item sku="123-AA"><qty>100</qty></item></order>`, xsderrors.CodeValidationFacet)
	mustNotValidateRuntime(t, engine, `<order><item sku="bad"><qty>2</qty></item></order>`, xsderrors.CodeValidationFacet)
	mustNotValidateRuntime(t, engine, `<order><item><qty>2</qty></item></order>`, xsderrors.CodeValidationAttribute)
}

func TestPatternFacetGroupsOrWithinStepAndAcrossDerivation(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<base>red</base>`)
	mustValidateRuntime(t, engine, `<base>green</base>`)
	mustValidateRuntime(t, engine, `<derived>green</derived>`)
	mustNotValidateRuntime(t, engine, `<derived>red</derived>`, xsderrors.CodeValidationFacet)
	mustNotValidateRuntime(t, engine, `<base>blue</base>`, xsderrors.CodeValidationFacet)
}

func TestCompatibleLengthFacetBoundsAreAllowed(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<v>a b</v>`)
	mustNotValidateRuntime(t, engine, `<v>a</v>`, xsderrors.CodeValidationFacet)
}

func TestAttributeRestrictionMustRespectBaseWildcard(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:attribute name="local"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:anyAttribute namespace="##other"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:anyAttribute namespace="##any"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeRestrictionDoesNotInheritBaseWildcard(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:anyAttribute namespace="##other" processContents="skip"/></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent><xs:restriction base="base"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root/>`)
	mustNotValidateRuntime(t, engine, `<root xmlns:f="urn:f" f:a="x"/>`, xsderrors.CodeValidationAttribute)
}

func TestComplexContentCannotExtendAllWithParticles(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:all><xs:element name="a"/></xs:all></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestAnyTypeAllowsAttributesAndChildren(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:anyType"/></xs:schema>`)
	mustValidateRuntime(t, engine, `<root custom="1"><anything other="2">text<nested/></anything></root>`)
}

func TestComplexExtensionInheritsAttributeWildcard(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root xmlns="urn:test" xmlns:foo="urn:foo" foo:attr="1"><child>ok</child></root>`)
}

func TestSchemaNamesAllowXML10FifthEditionNameStartChars(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="〇10" type="xs:int"/>
      <xs:attribute name="〡20" type="xs:int"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root 〇10="1" 〡20="2"/>`)
}

func TestUnionRestrictionAllowsOnlyPatternAndEnumerationFacets(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="base">
    <xs:union memberTypes="xs:NMTOKEN xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="bad">
    <xs:restriction base="base">
      <xs:length value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaFacet)
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
	choiceEngine := mustCompileRuntime(t, choice)
	mustValidateRuntime(t, choiceEngine, `<root><b>x</b></root>`)
	mustNotValidateRuntime(t, choiceEngine, `<root><a>x</a><b>y</b></root>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, choiceEngine, `<root/>`, xsderrors.CodeValidationContent)

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
	allEngine := mustCompileRuntime(t, all)
	mustValidateRuntime(t, allEngine, `<root><d>y</d><c>z</c></root>`)
	mustNotValidateRuntime(t, allEngine, `<root><d>y</d></root>`, xsderrors.CodeValidationContent)

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
	engine := mustCompileRuntime(t, schema)
	mustValidateRuntime(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><foreign xmlns="urn:f"/><empty xsi:nil="true"/></root>`)
	mustNotValidateRuntime(t, engine, `<root><local/><empty/></root>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><empty xsi:nil="true">x</empty></root>`, xsderrors.CodeValidationNil)
	mustNotValidateRuntime(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><empty xsi:nil="true"> </empty></root>`, xsderrors.CodeValidationNil)

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
	skipEngine := mustCompileRuntime(t, skip)
	mustValidateRuntime(t, skipEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><declared xsi:nil="true">text<child/></declared></root>`)

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
	nilledEngine := mustCompileRuntime(t, nilledRequired)
	mustValidateRuntime(t, nilledEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`)
	mustNotValidateRuntime(t, nilledEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"><required/></root>`, xsderrors.CodeValidationNil)

	notNillable := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string" nillable="false"/></xs:schema>`)
	mustNotValidateRuntime(t, notNillable, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="false">abc</root>`, xsderrors.CodeValidationNil)

	other := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:any namespace="##other" processContents="lax"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidateRuntime(t, other, `<root xmlns="urn:test"><local xmlns=""/></root>`, xsderrors.CodeValidationElement)
}

func TestStrictWildcardRequiresGlobalElementDeclaration(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="strict"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidateRuntime(t, engine, `<root><unknown/></root>`, xsderrors.CodeValidationElement)
	mustValidateRuntime(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><unknown xsi:type="xs:string" xmlns:xs="http://www.w3.org/2001/XMLSchema">x</unknown></root>`)
}

func TestStrictWildcardRecoveryConsumesOccurrenceBeforeRequiredSibling(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="strict"/>
        <xs:element name="after" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	session, err := validate.NewSession(engine, validate.Options{MaxErrors: 10})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	err = session.Validate(strings.NewReader(`<root><unknown/><after>ok</after></root>`))
	codes := validationErrorCodes(err)
	want := []xsderrors.Code{xsderrors.CodeValidationElement}
	if !slices.Equal(codes, want) {
		t.Fatalf("validation codes = %v, want %v; err=%v", codes, want, err)
	}
}

func validationErrorCodes(err error) []xsderrors.Code {
	if err == nil {
		return nil
	}
	if errs, ok := errors.AsType[xsderrors.Errors](err); ok {
		codes := make([]xsderrors.Code, 0, len(errs))
		for _, item := range errs {
			if x, ok := errors.AsType[*xsderrors.Error](item); ok {
				codes = append(codes, x.Code)
			}
		}
		return codes
	}
	if x, ok := errors.AsType[*xsderrors.Error](err); ok {
		return []xsderrors.Code{x.Code}
	}
	return nil
}

func TestEmptyChoiceWithRequiredOccurrenceRejectsEmptyContent(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:choice/></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidateRuntime(t, engine, `<root/>`, xsderrors.CodeValidationContent)
}

func TestAnyAttributeRejectsOccurrenceAttributes(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:anyAttribute minOccurs="2"/></xs:complexType></xs:element></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:anyAttribute maxOccurs="2"/></xs:complexType></xs:element></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
}

func TestInvalidWildcardAttributesAreSchemaErrors(t *testing.T) {
	tests := []string{
		`<xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="##bogus"/></xs:sequence></xs:complexType></xs:element>`,
		`<xs:element name="root"><xs:complexType><xs:anyAttribute processContents="open"/></xs:complexType></xs:element>`,
	}
	for _, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
	}
}

func TestDirectSequenceContentModel(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root><a>x</a><b>y</b></root>`)
	mustNotValidateRuntime(t, engine, `<root><b>y</b><a>x</a></root>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<root><a>x</a></root>`, xsderrors.CodeValidationContent)
}

func TestChoiceWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:any namespace="##other"/>
        <xs:any namespace="urn:foreign"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
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
	engine := mustCompileRuntime(t, schema.String())

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
	mustValidateRuntime(t, engine, doc.String())

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
	mustNotValidateRuntime(t, engine, missing.String(), xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<root><e0>x</e0><e0>y</e0></root>`, xsderrors.CodeValidationElement)
}

func TestAllCannotBeNestedInSequence(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:all><xs:element name="a"/></xs:all>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestModelGroupSyntaxIsValidated(t *testing.T) {
	tests := []struct {
		body string
		code xsderrors.Code
	}{
		{`<xs:complexType name="t"><xs:all name="bad"><xs:element name="a"/></xs:all></xs:complexType>`, xsderrors.CodeSchemaInvalidAttribute},
		{`<xs:complexType name="t"><xs:all minOccurs="0" maxOccurs="0"><xs:element name="a"/></xs:all></xs:complexType>`, xsderrors.CodeSchemaOccurrence},
		{`<xs:complexType name="t"><xs:all><xs:element name="a"/><xs:annotation/></xs:all></xs:complexType>`, xsderrors.CodeSchemaContentModel},
		{`<xs:complexType name="t"><xs:all><xs:any namespace="##any"/></xs:all></xs:complexType>`, xsderrors.CodeSchemaContentModel},
		{`<xs:complexType name="t"><xs:sequence><xs:group/></xs:sequence></xs:complexType>`, xsderrors.CodeSchemaReference},
		{`<xs:complexType name="t"><xs:sequence><xs:group ref="g"><xs:element name="a"/></xs:group></xs:sequence></xs:complexType>`, xsderrors.CodeSchemaContentModel},
		{`<xs:complexType name="t"><xs:sequence><xs:attribute name="a"/></xs:sequence></xs:complexType>`, xsderrors.CodeSchemaContentModel},
		{`<xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group><xs:complexType name="t"><xs:sequence><xs:group ref="g"/></xs:sequence></xs:complexType>`, xsderrors.CodeSchemaContentModel},
	}
	for _, test := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.body+`</xs:schema>`))})
		expectCode(t, err, test.code)
	}
}

func TestComplexContentExtensionFromEmptyAllBase(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group>
  <xs:complexType name="base"><xs:all/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="base"><xs:group ref="g"/></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestComplexContentExtensionCannotUseAllGroupWithNonEmptyBase(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="b"/></xs:all></xs:group>
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="base"><xs:group ref="g"/></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestComplexContentExtensionCanUseAllWithEmptyBase(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"><xs:all minOccurs="0" maxOccurs="1"><xs:element name="a"/></xs:all></xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestRestrictionParticleOccurrenceMustBeSubset(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:sequence><xs:element name="a" minOccurs="0" maxOccurs="0"/></xs:sequence></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionParticleCountRangeMustBeSubset(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:sequence><xs:any/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="e"/><xs:element name="e"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a"/><xs:element name="a"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionChoiceCanMapToWildcardRange(t *testing.T) {
	mustCompileRuntime(t, `
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
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" xmlns:o="urn:other" elementFormDefault="qualified">
  <xs:import namespace="urn:other"/>
  <xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="local"/><xs:any namespace="##other" processContents="strict"/></xs:choice></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element ref="o:foreign"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="root" type="t:derived"/>
</xs:schema>`)),
		source.Bytes("other.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other" elementFormDefault="qualified"><xs:element name="foreign"/></xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root xmlns="urn:test" xmlns:o="urn:other"><o:foreign/></root>`)

	_, err = compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base"><xs:sequence><xs:choice><xs:element name="local"/><xs:any namespace="##other"/></xs:choice></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element name="otherLocal"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionRejectsWildcardForElementAndNillableLoosening(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" minOccurs="2" maxOccurs="10"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:any minOccurs="3" maxOccurs="9"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="e1" nillable="false"/><xs:element name="e2"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="e1" nillable="true"/><xs:element name="e2"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
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
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionRepeatedSequenceCanMapToRepeatedChoiceBranch(t *testing.T) {
	mustCompileRuntime(t, `
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

func TestRestrictionRepeatedElementCanRestrictRepeatedChoice(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="a"/>
        <xs:element name="b"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a" minOccurs="2" maxOccurs="2"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root><a/><a/></root>`)
	mustNotValidateRuntime(t, engine, `<root><a/></root>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<root><b/><b/></root>`, xsderrors.CodeValidationElement)
}

func TestRestrictionOptionalBoundedElementCanRestrictRepeatedChoice(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a"/>
        <xs:element name="b"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a" minOccurs="0" maxOccurs="2"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root/>`)
	mustValidateRuntime(t, engine, `<root><a/></root>`)
	mustValidateRuntime(t, engine, `<root><a/><a/></root>`)
	mustNotValidateRuntime(t, engine, `<root><a/><a/><a/></root>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<root><b/></root>`, xsderrors.CodeValidationElement)
}

func TestRestrictionChoiceBranchOccurrenceIsPreserved(t *testing.T) {
	mustCompileRuntime(t, `
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
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionRepeatedOptionalElementCanRestrictRepeatedOptionalChoice(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root xmlns="urn:test"><annotation/><element/></root>`)
	mustNotValidateRuntime(t, engine, `<root xmlns="urn:test"><annotation/><element/><element/></root>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<root xmlns="urn:test"><annotation/><any/></root>`, xsderrors.CodeValidationElement)
}

func TestRestrictionRepeatedChoiceLimitDoesNotApplyToNestedGroup(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a"/>
        <xs:element name="any"/>
      </xs:choice>
      <xs:sequence>
        <xs:element name="c" minOccurs="0" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a" minOccurs="0" maxOccurs="unbounded"/>
          <xs:sequence>
            <xs:element name="c" minOccurs="0" maxOccurs="unbounded"/>
          </xs:sequence>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root><a/><c/><c/></root>`)
	mustNotValidateRuntime(t, engine, `<root><a/><a/></root>`, xsderrors.CodeValidationElement)
}

func TestRestrictionParticleLimitDoesNotLeakToSharedGroup(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="annotation" minOccurs="0"/>
      <xs:element name="element" minOccurs="0" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:group>
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
      <xs:restriction base="base">
        <xs:group ref="G"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="direct">
    <xs:complexType><xs:group ref="G"/></xs:complexType>
  </xs:element>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<direct><element/><element/></direct>`)
	mustValidateRuntime(t, engine, `<root><element/></root>`)
	mustNotValidateRuntime(t, engine, `<root><element/><element/></root>`, xsderrors.CodeValidationElement)
}

func TestRestrictionSequenceToChoiceRequiresValidBranchMap(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1"/><xs:element name="e2"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="other"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice minOccurs="0"><xs:element name="c"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice minOccurs="0" maxOccurs="2"><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice minOccurs="0" maxOccurs="2"><xs:element name="a"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for i, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionChoiceCanRestrictSequenceWithEmptiableRemainder(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="e1" minOccurs="0"/><xs:element name="e2" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" minOccurs="0"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)

	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="e1" minOccurs="0"/><xs:element name="e2" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice minOccurs="0"><xs:element name="e1"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)
}

func TestRestrictionSequenceMappingSkipsOnlyEmptiableBaseParticles(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a" minOccurs="0"/><xs:element name="b"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)

	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" xmlns:i="urn:imported">
  <xs:import namespace="urn:imported"/>
  <xs:complexType name="base"><xs:sequence><xs:element name="e1" maxOccurs="3"/><xs:element name="e2" minOccurs="0" maxOccurs="3"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="t:base"><xs:sequence><xs:element ref="i:e1"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="doc" type="t:derived"/>
</xs:schema>`)),
		source.Bytes("imp.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:imported"><xs:element name="e1"/></xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionSequenceToAllRequiresValidParticleMap(t *testing.T) {
	mustCompileRuntime(t, `
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
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		if err == nil {
			t.Fatalf("case %d: Compile() succeeded unexpectedly", i)
		}
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionAllCannotRestrictMultiParticleSequence(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="e1"/><xs:element name="e2"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:all><xs:element name="e1"/><xs:element name="e2"/></xs:all></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestNestedChoiceGroupOccurrenceIsNotDoubleCounted(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:element name="foo"/>
  <xs:group name="G"><xs:choice><xs:element ref="t:foo" maxOccurs="3"/></xs:choice></xs:group>
  <xs:element name="doc">
    <xs:complexType>
      <xs:choice><xs:group ref="t:G"/></xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<doc xmlns="urn:test"><foo/><foo/><foo/></doc>`)
}

func TestSingleParticleNestedSequenceOccurrenceIsFlattened(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<doc xmlns="urn:test"><foo/><foo/><foo/></doc>`)
}

func TestSingleBranchNestedChoiceOccurrenceIsFlattened(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<doc xmlns="urn:test"><foo/><e1/><e1/><e1/></doc>`)
}

func TestNestedChoiceBranchOccurrencesInsideSequence(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root><foo/><a/><a/><bar/></root>`)
	mustValidateRuntime(t, engine, `<root><foo/><a/><a/><a/><bar/></root>`)
	mustValidateRuntime(t, engine, `<root><foo/><b/><bar/></root>`)
	mustNotValidateRuntime(t, engine, `<root><foo/><a/><bar/></root>`, xsderrors.CodeValidationElement)
}

func TestExtensionUPAChecksRepeatableModelRefTerms(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="base"><xs:choice><xs:any namespace="##targetNamespace" maxOccurs="3"/></xs:choice></xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="t:base"><xs:choice><xs:element name="c1"/><xs:element name="c2"/></xs:choice></xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestAllCanContainOnlyElementParticles(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:all><xs:sequence><xs:element name="a"/></xs:sequence></xs:all></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestChoiceUPAChecksGroupSequenceFirstElement(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
  <xs:complexType name="t"><xs:choice><xs:element name="a"/><xs:group ref="g"/></xs:choice></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRequiredGroupRefWithEmptiableChoiceCanBeAbsent(t *testing.T) {
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
	mustValidateRuntime(t, engine, `<Root xmlns="urn:test"><A/></Root>`)
}

func TestTopLevelGroupCompositorCannotHaveOccurs(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:choice maxOccurs="2"><xs:element name="a"/></xs:choice></xs:group>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaOccurrence)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:choice><xs:element name="a"/></xs:choice><xs:sequence><xs:element name="b"/></xs:sequence></xs:group>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="a" maxOccurs="2"/></xs:all></xs:group>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaOccurrence)
}

func TestTopLevelGroupDeclarationAttributesAreValidated(t *testing.T) {
	tests := []struct {
		group string
		code  xsderrors.Code
	}{
		{`<xs:group ref="g"/>`, xsderrors.CodeSchemaInvalidAttribute},
		{`<xs:group><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, xsderrors.CodeSchemaReference},
		{`<xs:group name="g" minOccurs="1"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, xsderrors.CodeSchemaInvalidAttribute},
		{`<xs:group name="g" maxOccurs="1"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>`, xsderrors.CodeSchemaInvalidAttribute},
	}
	for _, test := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.group+`</xs:schema>`))})
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
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+group+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestAnyParticleCanContainOnlyAnnotation(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:sequence>
      <xs:any><xs:group ref="other"/></xs:any>
    </xs:sequence>
  </xs:group>
  <xs:group name="other"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestDirectGroupRefOccurrenceIsValidated(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/><xs:element name="b"/></xs:sequence></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="2" maxOccurs="4"/></xs:complexType></xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<r><a/><b/><a/><b/></r>`)
	mustNotValidateRuntime(t, engine, `<r><a/><b/></r>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<r><a/><b/><a/></r>`, xsderrors.CodeValidationContent)
}

func TestExtensionGroupRefOccurrenceIsPreserved(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><x/></r>`)
	mustValidateRuntime(t, engine, `<r><x/><a/><b/><a/></r>`)
	mustNotValidateRuntime(t, engine, `<r><x/><a/><b/><a/><b/></r>`, xsderrors.CodeValidationElement)

	engine = mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><x/></r>`)
	mustNotValidateRuntime(t, engine, `<r><x/><a/></r>`, xsderrors.CodeValidationElement)
}

func TestRepeatedSequenceCanStartNextOccurrenceAfterMinimumSatisfied(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><a/><a/><b/></r>`)
}

func TestChoiceKeepsSelectedBranch(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root><a>x</a><a>y</a></root>`)
	mustValidateRuntime(t, engine, `<root><b>x</b></root>`)
	mustNotValidateRuntime(t, engine, `<root><a>x</a></root>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<root><a>x</a><b>y</b></root>`, xsderrors.CodeValidationElement)

	repeating := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, repeating, `<root><a>x</a><b>y</b><a>z</a></root>`)
	mustNotValidateRuntime(t, repeating, `<root/>`, xsderrors.CodeValidationContent)
}

func TestInvalidOccurrenceIsSchemaCompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" minOccurs="2" maxOccurs="1"/></xs:sequence></xs:complexType></xs:element></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaOccurrence)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="1" maxOccurs="0"/></xs:complexType></xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaOccurrence)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g"><xs:all><xs:element name="a"/></xs:all></xs:group>
  <xs:element name="r"><xs:complexType><xs:group ref="g" minOccurs="0" maxOccurs="0"/></xs:complexType></xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaOccurrence)
}

func TestCompileOptionsNameAndOccurrenceLimits(t *testing.T) {
	_, err := compile.Compile(compile.Options{MaxSchemaNames: 1}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" maxOccurs="11"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxFiniteOccurs: 10}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	boundary := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a" maxOccurs="10"/><xs:element name="b" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxFiniteOccurs: 10}, []source.Source{source.Bytes("schema.xsd", []byte(boundary))})
	if err != nil {
		t.Fatalf("Compile() maxOccurs boundary error = %v", err)
	}

	_, err = compile.Compile(compile.Options{MaxContentModelStates: 1}, []source.Source{source.Bytes("schema.xsd", []byte(boundary))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	directSequence := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a"/><xs:element name="b"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxContentModelStates: 1}, []source.Source{source.Bytes("schema.xsd", []byte(directSequence))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	directChoice := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:choice><xs:element name="a"/><xs:element name="b"/></xs:choice></xs:complexType></xs:element></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxContentModelStates: 1}, []source.Source{source.Bytes("schema.xsd", []byte(directChoice))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	_, err = compile.Compile(compile.Options{MaxContentModelStates: 32}, []source.Source{source.Bytes("schema.xsd", []byte(boundary))})
	if err != nil {
		t.Fatalf("Compile() content model state boundary error = %v", err)
	}
}

func TestNestedChoiceModelGroup(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><a>x</a><b>y</b><a>z</a></r>`)
}

func TestNestedSequenceModelGroupFlattens(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><a>x</a><b>y</b><c>z</c></r>`)
	mustNotValidateRuntime(t, engine, `<r><a>x</a><c>z</c></r>`, xsderrors.CodeValidationElement)
}

func TestNestedSequenceModelGroupOccurrenceIsValidated(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><c>z</c></r>`)
	mustValidateRuntime(t, engine, `<r><a>x</a><b>y</b><c>z</c></r>`)
	mustValidateRuntime(t, engine, `<r><a>x</a><b>y</b><a>x</a><b>y</b><c>z</c></r>`)
	mustNotValidateRuntime(t, engine, `<r><a>x</a><c>z</c></r>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<r><b>y</b><c>z</c></r>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<r><a>x</a><b>y</b><a>x</a><b>y</b><a>x</a><b>y</b><c>z</c></r>`, xsderrors.CodeValidationElement)
}

func TestChoiceBranchCanBeSequenceModelGroup(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><a>x</a><b>y</b></r>`)
	mustValidateRuntime(t, engine, `<r><c>z</c></r>`)
	mustNotValidateRuntime(t, engine, `<r><a>x</a></r>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<r><a>x</a><c>z</c></r>`, xsderrors.CodeValidationElement)
}

func TestDirectRecursiveModelGroupsAreSchemaErrors(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestRecursiveModelGroupsThroughElements(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<r><a/></r>`)
	mustValidateRuntime(t, engine, `<r><a><a/></a></r>`)
	mustNotValidateRuntime(t, engine, `<r/>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<r><a><b/></a></r>`, xsderrors.CodeValidationElement)
}

func TestRecursiveAttributeGroupsAreSchemaErrors(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="a">
    <xs:attributeGroup ref="a"/>
  </xs:attributeGroup>
  <xs:element name="r">
    <xs:complexType>
      <xs:attributeGroup ref="a"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestAttributeGroupCanBeReusedByMultipleTypes(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="common">
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:attributeGroup>
  <xs:element name="a">
    <xs:complexType><xs:attributeGroup ref="common"/></xs:complexType>
  </xs:element>
  <xs:element name="b">
    <xs:complexType><xs:attributeGroup ref="common"/></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<a id="a1"/>`)
	mustValidateRuntime(t, engine, `<b id="b1"/>`)
	mustNotValidateRuntime(t, engine, `<a/>`, xsderrors.CodeValidationAttribute)
}

func TestRepeatingChoiceWithRepeatedBranchPartitionsAtClose(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a" minOccurs="3" maxOccurs="5"/>
        <xs:element name="b" minOccurs="3" maxOccurs="5"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<r><a/><a/><a/><a/><a/><a/></r>`)
	mustValidateRuntime(t, engine, `<r><a/><a/><a/><b/><b/><b/></r>`)
	mustNotValidateRuntime(t, engine, `<r><a/><a/></r>`, xsderrors.CodeValidationContent)
}

func TestRepeatedSingleBranchChoicePartitionsByLength(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a" minOccurs="3" maxOccurs="5"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	for _, n := range []int{0, 3, 4, 5, 6, 7, 8} {
		mustValidateRuntime(t, engine, repeatedA(n))
	}
	for _, n := range []int{1, 2} {
		mustNotValidateRuntime(t, engine, repeatedA(n), xsderrors.CodeValidationContent)
	}
}

func TestRepeatedMixedBranchChoicePartitionsByLength(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="a" minOccurs="3" maxOccurs="5"/>
        <xs:element name="b" minOccurs="3" maxOccurs="5"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<r><a/><a/><a/><a/><b/><b/><b/><b/></r>`)
	mustValidateRuntime(t, engine, `<r><a/><a/><a/><a/><a/><a/><b/><b/><b/></r>`)
	mustNotValidateRuntime(t, engine, `<r><a/><a/><a/><b/><b/></r>`, xsderrors.CodeValidationContent)
}

func TestLargeMaxOccursUsesCountedState(t *testing.T) {
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" minOccurs="0" maxOccurs="100000"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	model := compiledRootModel(t, schema)
	if got := len(model.Rows); got > 3 {
		t.Fatalf("compiled rows = %d, want compact counted state", got)
	}
	mustValidateRuntime(t, engine, repeatedA(8))
}

func TestLargeMinOccursInSequenceUsesCountedState(t *testing.T) {
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" minOccurs="10" maxOccurs="10"/>
        <xs:element name="b"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	model := compiledRootModel(t, schema)
	if got := len(model.Rows); got > 4 {
		t.Fatalf("compiled rows = %d, want compact counted state", got)
	}
	mustValidateRuntime(t, engine, repeatedAWithB(10))
	mustNotValidateRuntime(t, engine, repeatedAWithB(9), xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, repeatedAWithB(11), xsderrors.CodeValidationElement)
}

func TestFixedRepeatCanBeFollowedBySameElement(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" minOccurs="2" maxOccurs="2"/>
        <xs:element name="a"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, repeatedA(3))
	mustNotValidateRuntime(t, engine, repeatedA(2), xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, repeatedA(4), xsderrors.CodeValidationElement)
}

func TestLargeFiniteNestedRepeatIsExact(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence minOccurs="18" maxOccurs="18">
          <xs:element name="a"/>
          <xs:element name="b"/>
        </xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, repeatedAB(18))
	mustNotValidateRuntime(t, engine, repeatedAB(17), xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, repeatedAB(19), xsderrors.CodeValidationElement)
}

func TestNullableNestedRepeatPreservesInnerMinimum(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence minOccurs="2" maxOccurs="2">
          <xs:sequence minOccurs="0" maxOccurs="unbounded">
            <xs:element name="b" minOccurs="2" maxOccurs="3"/>
          </xs:sequence>
        </xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, repeatedB(0))
	mustNotValidateRuntime(t, engine, repeatedB(1), xsderrors.CodeValidationContent)
	for _, n := range []int{2, 3, 4, 5, 6} {
		mustValidateRuntime(t, engine, repeatedB(n))
	}
}

func TestNullableExactRepeatCanBeSatisfiedByEmptyOccurrences(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence minOccurs="2" maxOccurs="2">
          <xs:element name="a" minOccurs="0"/>
        </xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	for _, n := range []int{0, 1, 2} {
		mustValidateRuntime(t, engine, repeatedA(n))
	}
	mustNotValidateRuntime(t, engine, repeatedA(3), xsderrors.CodeValidationElement)
}

func TestNullableSingleParticleRepeatCompilesWithoutStateExplosion(t *testing.T) {
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence maxOccurs="100">
          <xs:element name="a" maxOccurs="unbounded"/>
        </xs:sequence>
        <xs:element name="b"/>
        <xs:sequence maxOccurs="100">
          <xs:element name="a" maxOccurs="unbounded"/>
        </xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<r><a/><a/><b/><a/></r>`)
}

func TestLargeFiniteNestedRepeatReturnsSchemaLimit(t *testing.T) {
	_, err := compile.Compile(compile.Options{MaxContentModelStates: 8}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:sequence minOccurs="18" maxOccurs="18">
          <xs:element name="a"/>
          <xs:element name="b"/>
        </xs:sequence>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func repeatedA(n int) string {
	var b strings.Builder
	b.WriteString("<r>")
	for range n {
		b.WriteString("<a/>")
	}
	b.WriteString("</r>")
	return b.String()
}

func repeatedB(n int) string {
	var b strings.Builder
	b.WriteString("<r>")
	for range n {
		b.WriteString("<b/>")
	}
	b.WriteString("</r>")
	return b.String()
}

func repeatedAWithB(n int) string {
	var b strings.Builder
	b.WriteString("<r>")
	for range n {
		b.WriteString("<a/>")
	}
	b.WriteString("<b/></r>")
	return b.String()
}

func repeatedAB(n int) string {
	var b strings.Builder
	b.WriteString("<r>")
	for range n {
		b.WriteString("<a/><b/>")
	}
	b.WriteString("</r>")
	return b.String()
}

func TestRepeatingChoiceRestrictionWithDerivedChoiceValidates(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root xmlns="urn:test"><c3/><c3/><tail/></root>`)
	mustNotValidateRuntime(t, engine, `<root xmlns="urn:test"><tail/></root>`, xsderrors.CodeValidationElement)
}

func TestSequenceParticleCannotRestrictElementParticle(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestChoiceDuplicateElementIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="a" type="xs:int"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRepeatingChoiceWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice maxOccurs="10">
        <xs:any namespace="urn:other" processContents="lax"/>
        <xs:any namespace="urn:other" processContents="strict"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSequenceRepeatedElementCanBeUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:element name="a" maxOccurs="2"/>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSequenceWildcardElementOverlapIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:any namespace="##any" maxOccurs="unbounded"/>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSequenceWildcardWildcardOverlapIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:any namespace="##other" minOccurs="0"/>
      <xs:any namespace="##other"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSequenceWildcardLocalAndListOverlapIsUPACompileError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:any namespace="##local urn:other" minOccurs="0"/>
      <xs:any namespace="##local"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRepeatingSequenceWildcardProcessOnlyOverlapCompiles(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ok">
    <xs:sequence maxOccurs="10">
      <xs:any namespace="urn:other" processContents="strict"/>
      <xs:any namespace="urn:other" processContents="lax"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestRepeatingSequenceWildcardListOrderIsSetEquivalent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ok">
    <xs:sequence maxOccurs="10">
      <xs:any namespace="urn:b urn:a urn:b"/>
      <xs:any namespace="urn:a urn:b"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestAllParticleCannotRepeat(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string" maxOccurs="2"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaOccurrence)
}

func TestComplexExtensionUnionsAttributeWildcards(t *testing.T) {
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
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
		source.Bytes("foreign.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:f">
  <xs:attribute name="foreign" type="xs:string"/>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root xmlns:f="urn:f" local="a" f:foreign="b"/>`)
}

func TestComplexExtensionWildcardUnionCoversAllNamedNamespaces(t *testing.T) {
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:t"
           xmlns:t="urn:t"
           xmlns:f="urn:f">
  <xs:import namespace="urn:f" schemaLocation="foreign.xsd"/>
  <xs:attribute name="target" type="xs:string"/>
  <xs:complexType name="base"><xs:anyAttribute namespace="##other" processContents="lax"/></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="t:base"><xs:anyAttribute namespace="##targetNamespace" processContents="lax"/></xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="t:derived"/>
</xs:schema>`)),
		source.Bytes("foreign.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:f">
  <xs:attribute name="foreign" type="xs:string"/>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:t" xmlns:f="urn:f" t:target="a" f:foreign="b"/>`)
	mustNotValidateRuntime(t, engine, `<t:root xmlns:t="urn:t" local="x"/>`, xsderrors.CodeValidationAttribute)
}

func TestAttributeGroupWildcardsIntersect(t *testing.T) {
	engine := mustCompileRuntime(t, `
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
	mustValidateRuntime(t, engine, `<root/>`)
	mustNotValidateRuntime(t, engine, `<root xmlns:a="urn:a" a:x="1"/>`, xsderrors.CodeValidationAttribute)
	mustNotValidateRuntime(t, engine, `<root xmlns:b="urn:b" b:x="1"/>`, xsderrors.CodeValidationAttribute)
}

func TestAttributeWildcardUnionMustBeExpressible(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
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
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func wideChoiceSchema(width int, extraParticles string) string {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
`)
	for i := range width {
		sb.WriteString(`        <xs:element name="f` + strconv.Itoa(i) + `" type="xs:string"/>` + "\n")
	}
	sb.WriteString(extraParticles)
	sb.WriteString(`      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	return sb.String()
}

func compiledRootModel(t *testing.T, schema string) runtime.CompiledModel {
	t.Helper()
	build := mutableSchemaBuild(t, schema)
	return build.CompiledModels[rootBuildContentModel(t, build)]
}

func requireIndexedRootModel(t *testing.T, schema string) {
	t.Helper()
	model := compiledRootModel(t, schema)
	if model.Kind != runtime.CompiledModelDFA {
		t.Fatalf("root model kind = %v, want DFA", model.Kind)
	}
	for _, row := range model.Rows {
		if len(row.Edges) >= runtime.CompiledDFARowIndexMinEdges && !row.Index.IsEnabled() {
			t.Fatalf("row with %d edges has no name index", len(row.Edges))
		}
	}
}

func TestWideChoiceIndexedDispatch(t *testing.T) {
	schema := wideChoiceSchema(16, "")
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustValidateRuntime(t, engine, `<r><f0/><f15/><f7/><f7/></r>`)
	mustNotValidateRuntime(t, engine, `<r><f0/><zzz/></r>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<r><f0/><r/></r>`, xsderrors.CodeValidationElement)
}

func TestWideSequenceIndexedDispatch(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
`)
	for i := range 15 {
		sb.WriteString(`        <xs:element name="f` + strconv.Itoa(i) + `" type="xs:string" minOccurs="0"/>` + "\n")
	}
	sb.WriteString(`        <xs:element name="last" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	schema := sb.String()
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustValidateRuntime(t, engine, `<r><f3/><f10/><last/></r>`)
	mustNotValidateRuntime(t, engine, `<r><f3/></r>`, xsderrors.CodeValidationContent)
	mustNotValidateRuntime(t, engine, `<r><last/><f3/></r>`, xsderrors.CodeValidationElement)
}

func TestWideChoiceIndexedSubstitutionGroup(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="head" type="xs:string"/>
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element ref="head"/>
`)
	for i := range 15 {
		sb.WriteString(`        <xs:element name="f` + strconv.Itoa(i) + `" type="xs:string"/>` + "\n")
	}
	sb.WriteString(`      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	schema := sb.String()
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustValidateRuntime(t, engine, `<r><member/><f0/><head/><member/></r>`)
	mustNotValidateRuntime(t, engine, `<r><zzz/></r>`, xsderrors.CodeValidationElement)
}

func TestWideChoiceIndexedWildcardSkip(t *testing.T) {
	schema := wideChoiceSchema(15, `        <xs:any namespace="##other" processContents="skip"/>
`)
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustValidateRuntime(t, engine, `<r><f0/><o:x xmlns:o="urn:o"><o:y/></o:x><f14/></r>`)
	mustNotValidateRuntime(t, engine, `<r><zzz/></r>`, xsderrors.CodeValidationElement)
}

func TestWideChoiceIndexedWildcardLax(t *testing.T) {
	schema := wideChoiceSchema(15, `        <xs:any namespace="##other" processContents="lax"/>
`)
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustValidateRuntime(t, engine, `<r><o:x xmlns:o="urn:o"/><f3/></r>`)
}

func TestWideChoiceIndexedWildcardStrict(t *testing.T) {
	schema := wideChoiceSchema(15, `        <xs:any namespace="##other" processContents="strict"/>
`)
	engine := mustCompileRuntime(t, schema)
	requireIndexedRootModel(t, schema)
	mustNotValidateRuntime(t, engine, `<r><o:x xmlns:o="urn:o"/></r>`, xsderrors.CodeValidationElement)
}

func TestWideCountingExceptionRowKeepsLinearScan(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" minOccurs="2" maxOccurs="2"/>
`)
	for _, name := range []string{"b", "c", "d", "e", "f", "g"} {
		sb.WriteString(`        <xs:element name="` + name + `" minOccurs="0"/>` + "\n")
	}
	sb.WriteString(`        <xs:element name="a"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	schema := sb.String()
	engine := mustCompileRuntime(t, schema)
	model := compiledRootModel(t, schema)
	ambiguousRow := false
	for _, row := range model.Rows {
		if len(row.Edges) >= runtime.CompiledDFARowIndexMinEdges && !row.Index.IsEnabled() {
			ambiguousRow = true
		}
	}
	if !ambiguousRow {
		t.Fatal("expected a wide row without a name index")
	}
	mustValidateRuntime(t, engine, `<r><a/><a/><a/></r>`)
	mustValidateRuntime(t, engine, `<r><a/><a/><b/><g/><a/></r>`)
	mustNotValidateRuntime(t, engine, `<r><a/><a/><a/><a/></r>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<r><a/><b/><a/></r>`, xsderrors.CodeValidationElement)
}
