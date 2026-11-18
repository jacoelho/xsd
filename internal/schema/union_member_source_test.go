package schema

import (
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

func TestUnionMemberSourceAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		memberTypes        string
		hasMemberTypes     bool
		hasSimpleTypeChild bool
		wantValue          string
		wantNames          []xml.Name
	}{
		{
			name:               "anonymous child",
			hasSimpleTypeChild: true,
		},
		{
			name:               "empty attribute with anonymous child",
			memberTypes:        " \t\n\r ",
			hasMemberTypes:     true,
			hasSimpleTypeChild: true,
		},
		{
			name:           "tokens",
			memberTypes:    " xs:string\tp:local\nother\r\nlast ",
			hasMemberTypes: true,
			wantValue:      "xs:string p:local other last",
			wantNames: []xml.Name{
				{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"},
				{Space: "urn:member", Local: "local"},
				{Local: "other"},
				{Local: "last"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:member">` + "\n  " + `<xs:simpleType name="union">` + "\n    <xs:union"
			if tt.hasMemberTypes {
				input += ` memberTypes="` + tt.memberTypes + `"`
			}
			input += ">"
			if tt.hasSimpleTypeChild {
				input += `<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`
			}
			input += "</xs:union>\n  </xs:simpleType>\n</xs:schema>"
			doc, err := parseSchemaDocument("union.xsd", "union.xsd", strings.NewReader(input), limits, new(xmlstream.Reader), testSchemaAdmission(limits))
			if err != nil {
				t.Fatalf("parseSchemaDocument() error = %v", err)
			}
			union := doc.root.firstXS("simpleType").firstXS("union")
			members, ok := union.semantic.derivationMemberTypes()
			if !ok {
				t.Fatal("union has no admitted member source")
			}
			if members.Lexical.Present != tt.hasMemberTypes || members.Lexical.Value != tt.wantValue || !slices.Equal(members.Names, tt.wantNames) {
				t.Fatalf("admitted members = %+v, want present %v value %q names %#v", members, tt.hasMemberTypes, tt.wantValue, tt.wantNames)
			}
		})
	}
}

func TestCompiledListItemQueryRejectsInvalidRoots(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.loadOwned([]source.Source{source.Bytes("query.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:string xs:int"/></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="inner inner xs:boolean"/></xs:simpleType>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.index(); err != nil {
		t.Fatal(err)
	}
	if err := c.compileGlobals(); err != nil {
		t.Fatal(err)
	}
	q, ok := c.rt.Names.LookupQName("", "outer")
	if !ok {
		t.Fatal("compiled outer type missing")
	}
	id, ok := c.rt.GlobalTypes[q].Simple()
	if !ok {
		t.Fatal("outer is not a simple type")
	}
	if c.simpleListItemReachesList(id) {
		t.Fatal("compiled scalar union unexpectedly contains a list")
	}
	for _, invalid := range []SimpleTypeID{NoSimpleType, SimpleTypeID(1_000_000)} {
		if c.simpleListItemReachesList(invalid) {
			t.Fatalf("list-item query(%d) = true for invalid root", invalid)
		}
	}
}
