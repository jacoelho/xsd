package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestLiveTypesBesideZeroCountElementTypesValidate(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("zero-count.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="b" type="xs:int"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="hollow" minOccurs="0" maxOccurs="0"><xs:complexType/></xs:element>
    <xs:element name="restricted" minOccurs="0" maxOccurs="0"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
    <xs:element name="all" minOccurs="0" maxOccurs="0"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
    <xs:element name="live" type="B"/>
    <xs:element name="empty"><xs:complexType/></xs:element>
    <xs:element name="pair"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, test := range []struct {
		name     string
		document string
		valid    bool
	}{
		{name: "live siblings", document: `<root><live b="1">long text</live><empty/><pair><d/><c/></pair></root>`, valid: true},
		{name: "shared base attribute type", document: `<root><live b="x">t</live><empty/><pair><c/><d/></pair></root>`},
		{name: "live empty content", document: `<root><live>t</live><empty>text</empty><pair><c/><d/></pair></root>`},
		{name: "live all group incomplete", document: `<root><live>t</live><empty/><pair><c/></pair></root>`},
		{name: "zero-count element present", document: `<root><hollow/><live>t</live><empty/><pair><c/><d/></pair></root>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.document))
			if test.valid && err != nil {
				t.Fatalf("Validate(%s) error = %v, want valid", test.document, err)
			}
			if test.valid {
				return
			}
			if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Category() != xsderrors.CategoryValidation {
				t.Fatalf("Validate(%s) error = %v, want a validation diagnostic", test.document, err)
			}
		})
	}
}

func TestLiveSimpleTypesBesideZeroCountSimpleTypesReportAsWithoutThem(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction><xs:simpleType><xs:restriction base="xs:int"><xs:maxInclusive value="50"/></xs:restriction></xs:simpleType><xs:minInclusive value="10"/></xs:restriction></xs:simpleType>
  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    UNREACHED
    <xs:element name="t" type="T"/>
    <xs:element name="l"><xs:simpleType><xs:list><xs:simpleType><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/></xs:restriction></xs:simpleType></xs:list></xs:simpleType></xs:element>
    <xs:element name="u"><xs:simpleType><xs:union memberTypes="xs:boolean"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[a-c]+"/></xs:restriction></xs:simpleType></xs:union></xs:simpleType></xs:element>
    <xs:element name="c"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:enumeration value="yes"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
  </xs:sequence><xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"><xs:length value="2"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element>
</xs:schema>`
	const unreached = `<xs:element name="z1" minOccurs="0" maxOccurs="0"><xs:simpleType><xs:restriction base="xs:int"><xs:maxInclusive value="5"/></xs:restriction></xs:simpleType></xs:element>
    <xs:element name="z2" minOccurs="0" maxOccurs="0"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:enumeration value="no"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
    <xs:element name="z3" minOccurs="0" maxOccurs="0"><xs:complexType><xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[0-9]"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element>`
	compile := func(t *testing.T, unreached string) *xsd.Engine {
		t.Helper()
		engine, err := xsd.Compile(xsd.Bytes("zero-count-simple.xsd", []byte(strings.Replace(schema, "UNREACHED", unreached, 1))))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		return engine
	}
	with, without := compile(t, unreached), compile(t, "")
	for _, test := range []struct {
		name     string
		document string
		valid    bool
	}{
		{name: "valid", document: `<root a="ab"><t>20</t><l>1 2.5</l><u>abc</u><c>yes</c></root>`, valid: true},
		{name: "above anonymous base bound", document: `<root a="ab"><t>60</t><l>1</l><u>true</u><c>yes</c></root>`},
		{name: "below own bound", document: `<root a="ab"><t>5</t><l>1</l><u>true</u><c>yes</c></root>`},
		{name: "list item digits", document: `<root a="ab"><t>20</t><l>1 123</l><u>true</u><c>yes</c></root>`},
		{name: "no union member", document: `<root a="ab"><t>20</t><l>1</l><u>xyz</u><c>yes</c></root>`},
		{name: "text enumeration", document: `<root a="ab"><t>20</t><l>1</l><u>true</u><c>no</c></root>`},
		{name: "attribute length", document: `<root a="abc"><t>20</t><l>1</l><u>true</u><c>yes</c></root>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, want := with.Validate(strings.NewReader(test.document)), without.Validate(strings.NewReader(test.document))
			if (want == nil) != test.valid {
				t.Fatalf("without zero-count elements: Validate(%s) error = %v, want valid %v", test.document, want, test.valid)
			}
			if (got == nil) != (want == nil) || got != nil && got.Error() != want.Error() {
				t.Fatalf("Validate(%s) error = %v, want %v as without the zero-count elements", test.document, got, want)
			}
		})
	}
}
