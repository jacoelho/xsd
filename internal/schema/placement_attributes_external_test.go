package schema_test

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

type placementAttributeCase struct {
	name     string
	body     string
	wantCode xsderrors.Code
	wantMsg  string
	wantLine int
	wantCol  int
}

func compilePlacementAttributeCase(t *testing.T, tc placementAttributeCase) {
	t.Helper()
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + "\n" + tc.body + "\n</xs:schema>"
	compiled, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if tc.wantMsg == "" {
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		return
	}
	if compiled != nil {
		t.Fatal("invalid schema was published")
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("Compile() error = %T %[1]v, want structured diagnostic", err)
	}
	if diagnostic.Code() != tc.wantCode || diagnostic.Message() != tc.wantMsg {
		t.Fatalf("diagnostic = (%s, %q), want (%s, %q)", diagnostic.Code(), diagnostic.Message(), tc.wantCode, tc.wantMsg)
	}
	if diagnostic.Line() != tc.wantLine || diagnostic.Column() != tc.wantCol {
		t.Fatalf("location = %d:%d, want %d:%d", diagnostic.Line(), diagnostic.Column(), tc.wantLine, tc.wantCol)
	}
}

func inSequence(particle string) string {
	return `  <xs:complexType name="C">
    <xs:sequence>
      ` + particle + `
    </xs:sequence>
  </xs:complexType>`
}

func inElement(definition string) string {
	return `  <xs:element name="r">
    ` + definition + `
  </xs:element>`
}

func inComplexType(content string) string {
	return `  <xs:complexType name="C">
    ` + content + `
  </xs:complexType>`
}

func inContent(container, derivation string) string {
	return `  <xs:complexType name="C">
    <xs:` + container + `>
      ` + derivation + `
    </xs:` + container + `>
  </xs:complexType>`
}

func TestLocalForbiddenAttributeWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	invalid := func(name, body, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: msg, wantLine: line, wantCol: col}
	}
	var cases []placementAttributeCase
	for _, attr := range []string{`abstract="true"`, `final="#all"`, `substitutionGroup="H"`} {
		local := attr[:strings.IndexByte(attr, '=')]
		msg := "local element cannot have " + local
		cases = append(cases,
			invalid("element "+local+" before child", inSequence(`<xs:element name="e" `+attr+`><xs:complexType foo="1"/></xs:element>`), msg, 4, 7),
			invalid("element "+local+" before unbound type prefix", inSequence(`<xs:element name="e" `+attr+` type="p:T"/>`), msg, 4, 7),
		)
	}
	for _, attr := range []string{`name="T"`, `abstract="true"`, `final="#all"`, `block="#all"`} {
		local := attr[:strings.IndexByte(attr, '=')]
		msg := "local complexType cannot have " + local
		cases = append(cases,
			invalid("complexType "+local+" before child", inElement(`<xs:complexType `+attr+`><xs:sequence foo="1"/></xs:complexType>`), msg, 3, 5),
			invalid("complexType "+local+" before unbound type prefix", inElement(`<xs:complexType `+attr+`><xs:attribute name="a" type="p:T"/></xs:complexType>`), msg, 3, 5),
		)
	}
	for _, attr := range []string{`name="S"`, `final="#all"`} {
		local := attr[:strings.IndexByte(attr, '=')]
		msg := "local simpleType cannot have " + local
		cases = append(cases,
			invalid("simpleType "+local+" before child", inElement(`<xs:simpleType `+attr+`><xs:restriction base="xs:string" foo="1"/></xs:simpleType>`), msg, 3, 5),
			invalid("simpleType "+local+" before unbound base prefix", inElement(`<xs:simpleType `+attr+`><xs:restriction base="p:T"/></xs:simpleType>`), msg, 3, 5),
		)
	}
	for _, attr := range []string{`minOccurs="1"`, `maxOccurs="1"`} {
		local := attr[:strings.IndexByte(attr, '=')]
		msg := "top-level model group cannot have " + local
		cases = append(cases,
			invalid("named group model "+local+" before child", `  <xs:group name="G">
    <xs:sequence `+attr+`><xs:element name="e" foo="1"/></xs:sequence>
  </xs:group>`, msg, 3, 5),
			invalid("named group model "+local+" before unbound ref prefix", `  <xs:group name="G">
    <xs:choice `+attr+`><xs:element ref="p:E"/></xs:choice>
  </xs:group>`, msg, 3, 5),
		)
	}
	cases = append(cases,
		invalid("first forbidden element attribute in start-tag order", inSequence(`<xs:element name="e" substitutionGroup="H" abstract="true"/>`), "local element cannot have substitutionGroup", 4, 7),
		invalid("first forbidden complexType attribute in start-tag order", inElement(`<xs:complexType block="#all" name="T"/>`), "local complexType cannot have block", 3, 5),
		invalid("unknown attribute before forbidden one", inSequence(`<xs:element foo="1" name="e" abstract="true"/>`), `"element" cannot have attribute "foo"`, 4, 7),
		placementAttributeCase{name: "misplaced named complexType reports its parent's content model", wantCode: xsderrors.CodeSchemaContentModel, wantMsg: `invalid model group child "complexType"`, wantLine: 4, wantCol: 7, body: `  <xs:complexType name="C">
    <xs:sequence>
      <xs:complexType name="T"/>
    </xs:sequence>
  </xs:complexType>`},
		placementAttributeCase{name: "local declarations and definitions accept their attributes", body: `  <xs:element name="H" abstract="true"/>
  <xs:complexType name="C">
    <xs:sequence>
      <xs:element id="e1" name="e" type="xs:string" nillable="true" default="x" form="qualified" block="#all" minOccurs="0" maxOccurs="2"/>
      <xs:element name="f">
        <xs:complexType id="t1" mixed="true">
          <xs:attribute id="a1" name="a" form="unqualified" use="optional" default="y">
            <xs:simpleType id="s1"><xs:restriction base="xs:string"/></xs:simpleType>
          </xs:attribute>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:group name="G">
    <xs:sequence id="g1"><xs:element name="g"/></xs:sequence>
  </xs:group>`},
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestPlacementRequiredAttributeWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	missing := func(name, body, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: xsderrors.CodeSchemaReference, wantMsg: msg, wantLine: line, wantCol: col}
	}
	cases := []placementAttributeCase{
		missing("local element name before child", inSequence(`<xs:element minOccurs="0"><xs:complexType foo="1"/></xs:element>`), "local element missing name or ref", 4, 7),
		missing("unbound type prefix before missing local element name", inSequence(`<xs:element type="p:T"/>`), `unbound QName prefix "p"`, 4, 7),
		missing("local attribute name before child", inComplexType(`<xs:attribute><xs:simpleType foo="1"/></xs:attribute>`), "attribute missing name or ref", 3, 5),
		missing("unbound type prefix before missing local attribute name", inComplexType(`<xs:attribute type="p:T"/>`), `unbound QName prefix "p"`, 3, 5),
		missing("group use ref before child", inSequence(`<xs:group minOccurs="0"><xs:annotation foo="1"/></xs:group>`), "group use missing ref", 4, 7),
		missing("attributeGroup use ref before child", inComplexType(`<xs:attributeGroup><xs:annotation foo="1"/></xs:attributeGroup>`), "attributeGroup use missing ref", 3, 5),
		missing("unbound type prefix before missing top-level element name", `  <xs:element type="p:T"/>`, `unbound QName prefix "p"`, 2, 3),
	}
	for _, container := range []string{"simpleContent", "complexContent"} {
		for _, derivation := range []string{"restriction", "extension"} {
			msg := container + " " + derivation + " missing base"
			cases = append(cases,
				missing(msg+" before child", inContent(container, `<xs:`+derivation+`><xs:attribute name="a" foo="1"/></xs:`+derivation+`>`), msg, 4, 7),
				missing(msg+" before unbound type prefix", inContent(container, `<xs:`+derivation+`><xs:attribute name="a" type="p:T"/></xs:`+derivation+`>`), msg, 4, 7),
			)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}
