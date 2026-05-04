package xsd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestInstanceUTF8BOMBeforeRootIsIgnored(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	if err := engine.Validate(strings.NewReader("\ufeff<root/>")); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

type zeroReadThenStringReader struct {
	s        string
	off      int
	zeroRead bool
}

func (r *zeroReadThenStringReader) Read(p []byte) (int, error) {
	if !r.zeroRead {
		r.zeroRead = true
		return 0, nil
	}
	if r.off >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.off:])
	r.off += n
	return n, nil
}

func TestRequiredAttributesBeyondBitsetWidth(t *testing.T) {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType>`)
	for i := range 70 {
		schema.WriteString(`<xs:attribute name="a`)
		schema.WriteString(strconv.Itoa(i))
		schema.WriteString(`" type="xs:string" use="required"/>`)
	}
	schema.WriteString(`</xs:complexType></xs:element></xs:schema>`)
	engine := mustCompile(t, schema.String())
	var doc strings.Builder
	doc.WriteString(`<r`)
	for i := range 69 {
		doc.WriteString(` a`)
		doc.WriteString(strconv.Itoa(i))
		doc.WriteString(`="x"`)
	}
	doc.WriteString(`/>`)
	mustNotValidate(t, engine, doc.String(), ErrValidationAttribute)
}

func TestInstanceAttributeCharacterReferencesUseSeparateParserScratch(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:attribute name="a" type="xs:NMTOKENS" fixed="&#xA;&#xA;A&#xA;&#xA;B&#xA;"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<r a="&#xA;&#xA;A&#xA;&#xA;B&#xA;"/>`)
}

func TestInstanceAttributeCRLFMatchesSchemaLineEndingNormalization(t *testing.T) {
	engine := mustCompile(t, "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\"><xs:element name=\"r\"><xs:complexType><xs:attribute name=\"a\" type=\"xs:anySimpleType\" fixed=\"x\ny\"/></xs:complexType></xs:element></xs:schema>")
	mustValidate(t, engine, "<r a=\"x\r\ny\"/>")
}

func TestInvalidSchemaAttributeCombinations(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r"><xs:complexType><xs:simpleContent id="dup"><xs:extension id="dup" base="xs:string"/></xs:simpleContent></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" form="qualified" type="xs:string"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head"/>
  <xs:element name="r"><xs:complexType><xs:sequence><xs:element ref="head" form="qualified"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xs:targetNamespace="urn:bad">
  <xs:element name="r"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r"><xs:complexType><xs:attribute name="id" type="xs:ID" fixed="a"/></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r"><xs:complexType><xs:attribute name="a" type="xs:string"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:attribute></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" fixed="base"/>
  <xs:element name="r"><xs:complexType><xs:attribute ref="a" fixed="local"/></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="0"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g" id=":bad"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g"><xs:attributeGroup name="nested"/></xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup ref="g"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g"><xs:attributeGroup ref=""/></xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"/>
  <xs:attributeGroup name="g"><xs:attribute ref="a"/><xs:attribute ref="a"/></xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaDuplicate)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"/>
  <xs:attributeGroup name="g"><xs:attribute ref="a" type="xs:string"/></xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g"><xs:element name="bad"/></xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g"/>
  <xs:complexType name="t"><xs:attributeGroup ref="g"><xs:attribute name="bad"/></xs:attributeGroup></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:attribute name="a" form="Qualified"/></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:attribute name="a" form=""/></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" use="required"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:attribute name="a" use="required" default="abc"/></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:attribute name="a" value="x"/></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:annotation/></xs:attribute>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:attribute>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"/>
  <xs:complexType name="t"><xs:attribute ref="a"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:attribute></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="t"><xs:attribute name="xmlns"/></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/2001/XMLSchema-instance">
  <xs:attribute name="bad"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a"/>
  <xs:attribute ref="a"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad" abstract="TRUE"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" block="foo"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" block="#All"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" final="substitution"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" final="#All"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" minOccurs="0"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="bad" final="restriction"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="c"><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType>
  <xs:element name="bad" type="c" default="x"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="Qualified">
  <xs:element name="bad"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" nullable="true"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="e" type="xs:string"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="e" form="Qualified"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e" type="Self"/>
  <xs:simpleType name="Self"><xs:restriction base="Self"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence><xs:any processContents="strict"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="bad">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:any processContents="lax"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:anyAttribute processContents="strict"/>
  </xs:complexType>
  <xs:complexType name="bad">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:anyAttribute processContents="skip"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestAttributeRestrictionMustPreserveRequiredAndFixed(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:attribute name="a" use="required"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:attribute name="a"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:attribute name="a" fixed="x"/></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:attribute name="a" fixed="y"/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestProhibitedFixedAttributeIsValidatedAsFixedForXSD10(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" use="prohibited" fixed="37"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root/>`)
	mustValidate(t, engine, `<root a="37"/>`)
	mustNotValidate(t, engine, `<root a="38"/>`, ErrValidationAttribute)
}

func TestComplexContentCannotExtendSimpleContent(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="simple"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="simple"><xs:sequence><xs:element name="child"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestSimpleContentInvalidChildrenAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:simpleContent><xs:extension base="xs:string"/><xs:annotation/></xs:simpleContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:simpleContent><xs:restriction base="xs:string"/></xs:simpleContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="bad"><xs:simpleContent><xs:restriction base="base"><xs:sequence/></xs:restriction></xs:simpleContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestElementDefaultAndFixedApplyToEmptySimpleContent(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="defaulted" type="xs:int" default="5"/>
        <xs:element name="fixed" type="xs:int" fixed="7"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><defaulted/><fixed/></root>`)
	mustValidate(t, engine, `<root><defaulted>9</defaulted><fixed>7</fixed></root>`)
	mustNotValidate(t, engine, `<root><defaulted/><fixed>8</fixed></root>`, ErrValidationElement)
	mustNotValidate(t, engine, `<root><defaulted> </defaulted><fixed/></root>`, ErrValidationFacet)
}

func TestInvalidDefaultAndFixedValuesAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="n" type="xs:int" default="nope"/></xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="n"><xs:complexType><xs:attribute name="a" type="xs:int" fixed="nope"/></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="digits"><xs:restriction base="xs:int"><xs:enumeration value="1"/></xs:restriction></xs:simpleType>
  <xs:element name="n"><xs:complexType><xs:attribute name="a" type="digits" fixed=""/></xs:complexType></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaFacet)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="n" type="xs:int" default="1" fixed="1"/></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="n"><xs:complexType><xs:attribute name="a" type="xs:int" default="1" fixed="1"/></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestEmptyFixedValuesAreEnforced(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="empty" type="xs:string" fixed=""/>
      </xs:sequence>
      <xs:attribute name="a" type="xs:string" fixed=""/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root a=""><empty/></root>`)
	mustNotValidate(t, engine, `<root a="x"><empty/></root>`, ErrValidationAttribute)
	mustNotValidate(t, engine, `<root a=""><empty>x</empty></root>`, ErrValidationElement)
}

func TestFixedValueOnAnyTypeAndMixedContentIsValidated(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="any" fixed="abc"/>
  <xs:element name="mixed" fixed="abc">
    <xs:complexType mixed="true"><xs:sequence minOccurs="0"><xs:element name="child"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<any>abc</any>`)
	mustNotValidate(t, engine, `<any>def</any>`, ErrValidationElement)
	mustNotValidate(t, engine, `<mixed>def</mixed>`, ErrValidationElement)
}

func TestXMLBuiltInAttributesCanBeReferenced(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="http://www.w3.org/XML/1998/namespace"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="xml:base" use="required"/>
      <xs:attribute ref="xml:space"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root xml:base="a" xml:space="preserve"/>`)
	mustNotValidate(t, engine, `<root/>`, ErrValidationAttribute)
}

func TestXLinkBuiltInAttributesCanBeReferenced(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xlink="http://www.w3.org/1999/xlink">
  <xs:import namespace="http://www.w3.org/1999/xlink" schemaLocation="http://www.w3.org/XML/2008/06/xlink.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="xlink:type" default="locator"/>
      <xs:attribute ref="xlink:href" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="target.xml"/>`)
	mustNotValidate(t, engine, `<root/>`, ErrValidationAttribute)
}

func TestStandardAttributeSchemasDoNotDuplicateBuiltIns(t *testing.T) {
	engine, err := Compile(
		sourceBytes("xml-a.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/XML/1998/namespace">
  <xs:attribute name="lang" type="xs:string"/>
</xs:schema>`)),
		sourceBytes("xml-b.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/XML/1998/namespace">
  <xs:annotation><xs:documentation>second local copy</xs:documentation></xs:annotation>
  <xs:attribute name="lang" type="xs:string"/>
</xs:schema>`)),
		sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml-a.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="xml:lang" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<root xml:lang="en"/>`)
}

func TestAbstractComplexTypeCannotValidateElement(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="abs" abstract="true"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="direct" type="abs"/>
  <xs:element name="root" type="base"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<direct><a/><b/></direct>`, ErrValidationType)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="abs"><a/><b/></root>`, ErrValidationType)
}

func TestExplicitEmptyFinalOverridesFinalDefault(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" finalDefault="#all">
  <xs:complexType name="base" final=""><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:element name="b"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
	mustValidate(t, engine, `<root><a/><b/></root>`)
}

func TestRestrictionElementFixedUsesCanonicalValue(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="ints"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:complexType name="base"><xs:sequence><xs:element name="e" type="ints" fixed="1 2 3"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:sequence><xs:element name="e" type="ints" fixed="1   2      3"/></xs:sequence></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)
}

func TestRestrictionAttributeTypeMustDeriveFromBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test">
  <xs:simpleType name="unionType"><xs:union memberTypes="xs:float xs:integer"/></xs:simpleType>
  <xs:complexType name="base"><xs:attribute name="att" type="xs:integer"/></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="t:base"><xs:attribute name="att" type="t:unionType"/></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestSimpleContentRestrictionTypeMustDeriveFromBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent>
      <xs:extension base="xs:decimal"><xs:attribute name="foo"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:simpleContent>
      <xs:restriction base="base"><xs:simpleType><xs:list itemType="xs:int"/></xs:simpleType></xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestComplexContentRestrictionCannotUseSimpleContentBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent>
      <xs:extension base="xs:string"><xs:attribute name="attr1" type="xs:string"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base"><xs:sequence/></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestComplexContentExtensionCanAddAttributesToSimpleContentBase(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:simpleContent>
      <xs:extension base="xs:string"><xs:attribute name="field1" type="xs:string"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"><xs:attribute name="field2" type="xs:string"/></xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:element name="e"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestFixedMixedElementCannotContainChildElements(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="mixed" mixed="true">
    <xs:sequence><xs:element name="a" minOccurs="0" type="xs:byte"/></xs:sequence>
  </xs:complexType>
  <xs:element name="r" type="mixed" fixed="abc"/>
</xs:schema>`)
	mustValidate(t, engine, `<r/>`)
	mustValidate(t, engine, `<r>abc</r>`)
	mustNotValidate(t, engine, `<r>def</r>`, ErrValidationElement)
	mustNotValidate(t, engine, `<r>abc<a>1</a></r>`, ErrValidationElement)
	mustNotValidate(t, engine, `<r><a>1</a>abc</r>`, ErrValidationElement)
}

func TestRejectDTDAndNonUTF8Instances(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:string"/></xs:schema>`)
	mustNotValidate(t, engine, `<!DOCTYPE r [<!ELEMENT r ANY>]><r/>`, ErrUnsupportedDTD)
	mustNotValidate(t, engine, `<?xml version="1.0" encoding="ISO-8859-1"?><r/>`, ErrUnsupportedNonUTF8)
	mustNotValidate(t, engine, `<?xml version="1.1" encoding="UTF-8"?><r/>`, ErrUnsupportedXML11)
	mustNotValidate(t, engine, `<r>&xxe;</r>`, ErrUnsupportedExternal)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`<?xml version="1.1" encoding="UTF-8"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)))
	expectCode(t, err, ErrUnsupportedXML11)
}

func TestValidateCollectsRecoverableErrors(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
      </xs:sequence>
      <xs:attribute name="code" type="xs:int"/>
      <xs:attribute name="req" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	err := engine.Validate(strings.NewReader(`<root code="x"><a>bad</a><b>bad</b></root>`))
	errs, ok := err.(Errors)
	if !ok {
		t.Fatalf("Validate() error type = %T, want Errors; err=%v", err, err)
	}
	if len(errs) != 4 {
		t.Fatalf("len(Errors) = %d, want 4; err=%v", len(errs), err)
	}
	var xerr *Error
	if !errors.As(err, &xerr) {
		t.Fatalf("errors.As(*Error) failed for %v", err)
	}
	if xerr.Code != ErrValidationFacet {
		t.Fatalf("first error code = %s, want %s", xerr.Code, ErrValidationFacet)
	}
}

func TestValidateWithOptionsLimitsErrors(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	err := engine.ValidateWithOptions(strings.NewReader(`<root><a>x</a><b>y</b></root>`), ValidateOptions{MaxErrors: 1})
	if err == nil {
		t.Fatal("ValidateWithOptions() succeeded")
	}
	if _, ok := err.(Errors); ok {
		t.Fatalf("ValidateWithOptions() returned aggregate despite MaxErrors=1: %v", err)
	}
	expectCode(t, err, ErrValidationFacet)
}

func TestValidateKeepsMalformedXMLFatal(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	err := engine.Validate(strings.NewReader(`<root><a></root>`))
	if err == nil {
		t.Fatal("Validate() succeeded")
	}
	if _, ok := err.(Errors); ok {
		t.Fatalf("Validate() returned aggregate for malformed XML: %v", err)
	}
	expectCode(t, err, ErrValidationXML)
}

func TestValidateCollectsIDREFErrors(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="node" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID"/>
            <xs:attribute name="ref" type="xs:IDREF"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	err := engine.Validate(strings.NewReader(`<root><node ref="missing1"/><node ref="missing2"/></root>`))
	errs, ok := err.(Errors)
	if !ok {
		t.Fatalf("Validate() error type = %T, want Errors; err=%v", err, err)
	}
	if len(errs) != 2 {
		t.Fatalf("len(Errors) = %d, want 2; err=%v", len(errs), err)
	}
	for _, err := range errs {
		expectCode(t, err, ErrValidationType)
	}
}

func TestRequiredFixedIDREFAttributeDoesNotDefaultWhenAbsent(t *testing.T) {
	schema, err := os.ReadFile("tests/corpus/project/attribute-required-fixed-idref-no-default/schema.xsd")
	if err != nil {
		t.Fatalf("ReadFile(schema) error = %v", err)
	}
	engine, err := Compile(sourceBytes("schema.xsd", schema))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	doc, err := os.ReadFile("tests/corpus/project/attribute-required-fixed-idref-no-default/missing-required-fixed-idref.xml")
	if err != nil {
		t.Fatalf("ReadFile(instance) error = %v", err)
	}
	err = engine.Validate(bytes.NewReader(doc))
	if err == nil {
		t.Fatal("Validate() succeeded")
	}
	errs := []error{err}
	if all, ok := err.(Errors); ok {
		errs = all
	}
	if len(errs) != 1 {
		t.Fatalf("len(errors) = %d, want 1; err=%v", len(errs), err)
	}
	expectCode(t, errs[0], ErrValidationAttribute)
}

func TestEngineConcurrentValidation(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:sequence><xs:element name="v" type="xs:int" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				if err := engine.Validate(strings.NewReader(`<r><v>1</v><v>2</v><v>3</v></r>`)); err != nil {
					t.Errorf("Validate() error = %v", err)
					return
				}
			}
		})
	}
	wg.Wait()
}
