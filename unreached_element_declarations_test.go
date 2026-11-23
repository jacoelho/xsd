package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

const unreachedDeclarationsSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  %s
  <xs:element name="a"><xs:complexType><xs:sequence>
    %s
    <xs:element name="x" type="xs:string" minOccurs="0"/>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
  <xs:element name="other" type="xs:string" substitutionGroup="head"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="pair"><xs:complexType><xs:all>
      <xs:element name="c" type="xs:int"/>
      <xs:element name="d" type="xs:int" fixed="7"/>
    </xs:all></xs:complexType></xs:element>
    <xs:element ref="head" maxOccurs="unbounded"/>
    <xs:element name="items"><xs:complexType><xs:sequence>
      <xs:element name="item" maxOccurs="unbounded" nillable="true"><xs:complexType><xs:simpleContent>
        <xs:extension base="xs:int"><xs:attribute name="id" type="xs:string"/></xs:extension>
      </xs:simpleContent></xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
      <xs:key name="itemKey"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:key>
    </xs:element>
    <xs:element name="dflt" type="xs:int" default="3"/>
    <xs:element name="strict"><xs:complexType><xs:sequence><xs:any processContents="strict"/></xs:sequence></xs:complexType></xs:element>
    <xs:element name="lax"><xs:complexType><xs:sequence><xs:any processContents="lax"/></xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const unreachedGroup = `<xs:group name="dead"><xs:sequence>
    <xs:element name="head" type="xs:int" nillable="true"/>
    <xs:element name="g0" type="xs:int" default="1"/>
  </xs:sequence></xs:group>`

const unreachedDeclarations = `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
      <xs:element name="g1" type="xs:string" default="z"/>
      <xs:element name="g2" nillable="true"/>
      <xs:element name="g3"><xs:complexType><xs:all><xs:element name="g4"/></xs:all></xs:complexType>
        <xs:key name="deadKey"><xs:selector xpath="g4"/><xs:field xpath="."/></xs:key>
      </xs:element>
    </xs:sequence></xs:complexType></xs:element>
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="g5" type="xs:int" fixed="1"/></xs:sequence>`

// The declarations of a named group no type references compile before every
// global declaration, so publishing only reached declarations renumbers every
// live one; the schema without unreached declarations is the oracle.
func TestUnreachedElementDeclarationsDoNotChangeValidation(t *testing.T) {
	compile := func(t *testing.T, group, declarations string) *xsd.Engine {
		t.Helper()
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(fmt.Sprintf(unreachedDeclarationsSchema, group, declarations))))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		return engine
	}
	withUnreached, without := compile(t, unreachedGroup, unreachedDeclarations), compile(t, "", "")
	const xsi = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	live := func(replace ...string) string {
		document := `<root ` + xsi + `><pair><d>7</d><c>1</c></pair><member>m</member><head>h</head><other>o</other>` +
			`<items><item id="a">1</item><item id="b" xsi:nil="true"/></items><dflt/><strict><head>s</head></strict><lax><unknown/></lax></root>`
		return strings.NewReplacer(replace...).Replace(document)
	}
	for _, test := range []struct {
		name     string
		document string
		valid    bool
	}{
		{name: "live content", document: live(), valid: true},
		{name: "global sibling of unreached locals", document: `<a><x>t</x></a>`, valid: true},
		{name: "zero-count element present", document: `<a><gone/></a>`},
		{name: "fixed value in xs:all", document: live("<d>7</d>", "<d>8</d>")},
		{name: "xs:all term missing", document: live("<c>1</c>", "")},
		{name: "non-member at substitution head", document: live("<other>o</other>", "<gone>o</gone>")},
		{name: "duplicate key", document: live(`id="b"`, `id="a"`)},
		{name: "nil on non-nillable element", document: live("<dflt/>", `<dflt xsi:nil="true"/>`)},
		{name: "default value type", document: live("<dflt/>", "<dflt>x</dflt>")},
		{name: "strict wildcard naming an unreached local", document: live("<strict><head>s</head></strict>", "<strict><g1>s</g1></strict>")},
		{name: "strict wildcard global member", document: live("<strict><head>s</head></strict>", "<strict><member><x/></member></strict>")},
		{name: "lax wildcard global head", document: live("<lax><unknown/></lax>", "<lax><head><x/></head></lax>")},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := without.Validate(strings.NewReader(test.document))
			if (want == nil) != test.valid {
				t.Fatalf("Validate(%s) without unreached declarations error = %v, want valid %v", test.document, want, test.valid)
			}
			got := withUnreached.Validate(strings.NewReader(test.document))
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("Validate(%s) error = %v, want %v as without unreached declarations", test.document, got, want)
			}
		})
	}
}
