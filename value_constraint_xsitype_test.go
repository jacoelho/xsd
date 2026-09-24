package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestChangedTypeValueConstraintUsesCanonicalLexicalForm(t *testing.T) {
	t.Parallel()

	// XSD 1.0 Structures, Element Locally Valid (Element), 5.1.2 applies
	// the constraint's canonical lexical form to the actual type definition.
	for _, constraint := range []string{"default", "fixed"} {
		for _, test := range []struct {
			name    string
			base    string
			lexical string
			pattern string
			valid   bool
		}{
			{name: "integer", base: "xs:int", lexical: "01", pattern: "1", valid: true},
			{name: "boolean", base: "xs:boolean", lexical: "1", pattern: "true", valid: true},
			{name: "decimal", base: "xs:decimal", lexical: "+01.00", pattern: "1\\.0", valid: true},
			{name: "source-only pattern", base: "xs:int", lexical: "01", pattern: "01", valid: false},
		} {
			t.Run(constraint+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				source := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Actual"><xs:restriction base="%s"><xs:pattern value="%s"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="%s" %s="%s"/>
</xs:schema>`, test.base, test.pattern, test.base, constraint, test.lexical)
				engine, err := xsd.Compile(xsd.Bytes("constraint.xsd", []byte(source)))
				if err != nil {
					t.Fatal(err)
				}
				err = engine.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`))
				if (err == nil) != test.valid {
					t.Fatalf("empty %s=%q with actual pattern %q: error = %v, want valid %v", constraint, test.lexical, test.pattern, err, test.valid)
				}
			})
		}
	}
}

func TestChangedComplexSimpleContentValueConstraint(t *testing.T) {
	t.Parallel()

	for _, constraint := range []string{"default", "fixed"} {
		t.Run(constraint, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Declared"><xs:simpleContent><xs:extension base="xs:int"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Actual"><xs:simpleContent><xs:restriction base="Declared"><xs:pattern value="1"/></xs:restriction></xs:simpleContent></xs:complexType>
  <xs:element name="root" type="Declared" %s="01"/>
</xs:schema>`, constraint)
			engine, err := xsd.Compile(xsd.Bytes("constraint.xsd", []byte(source)))
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, test := range []struct {
				content string
				valid   bool
			}{
				{content: "", valid: true},
				{content: "01", valid: false},
				{content: "1", valid: true},
				{content: "", valid: true},
			} {
				document := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual">` + test.content + `</root>`
				err := session.Validate(strings.NewReader(document))
				if (err == nil) != test.valid {
					t.Fatalf("%s=%q with content %q: error = %v, want valid %v", constraint, "01", test.content, err, test.valid)
				}
			}
		})
	}
}

func TestChangedTypeSchemaSuppliedStringConstraintUsesActualFacets(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Accept"><xs:restriction base="xs:token"><xs:pattern value="a b"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Reject"><xs:restriction base="xs:token"><xs:pattern value="a c"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="xs:string" fixed="a  b"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("constraint.xsd", []byte(source)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{
			name: "schema supplied value is validated as the actual type",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Accept"/>`,
			want: true,
		},
		{
			name: "explicit content still compares against fixed value",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Accept">a b</root>`,
		},
		{
			name: "actual type facets reject schema supplied value",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Reject"/>`,
		},
		{
			name: "whitespace only content is not an absent value",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Accept">   </root>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.doc))
			if (err == nil) != test.want {
				t.Fatalf("Validate() error = %v, want valid %v", err, test.want)
			}
		})
	}
}

func TestChangedTypeConstraintUsesTheAcceptedUnionMember(t *testing.T) {
	t.Parallel()

	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="QNameNope"><xs:restriction base="xs:QName"><xs:pattern value="nope"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Declared"><xs:union memberTypes="QNameNope xs:boolean"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="Declared"><xs:pattern value="true"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Declared" default=" true "/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("constraint.xsd", []byte(source)))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`)); err != nil {
		t.Fatalf("failed QName attempt changed the accepted boolean default: %v", err)
	}
}
