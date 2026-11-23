package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func upaSchema(body string) string {
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">` + body + `</xs:schema>`
}

func compileUPAMessage(t *testing.T, sources ...xsd.SchemaSource) string {
	t.Helper()
	_, err := xsd.Compile(sources...)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel {
		t.Fatalf("Compile() error = %v, want %s", err, xsderrors.CodeSchemaContentModel)
	}
	return diagnostic.Message()
}

func TestUPADiagnosticNamesTheComplexTypeAndBothParticles(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "choice in a global type",
			schema: `<xs:complexType name="T"><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/><xs:element name="c"/></xs:sequence></xs:choice></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "sequence in a global type",
			schema: `<xs:complexType name="T"><xs:sequence><xs:element name="a" minOccurs="0" maxOccurs="2"/><xs:element name="a"/></xs:sequence></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "all group in a global type",
			schema: `<xs:complexType name="T"><xs:all><xs:element name="a"/><xs:element ref="t:a"/></xs:all></xs:complexType><xs:element name="a"/>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "anonymous type",
			schema: `<xs:element name="root"><xs:complexType><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/></xs:sequence></xs:choice></xs:complexType></xs:element>`,
			want:   `UPA violation in local complex type of element "{urn:t}root": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "anonymous type of a local element",
			schema: `<xs:complexType name="T"><xs:sequence><xs:element name="inner"><xs:complexType><xs:choice><xs:element name="a"/><xs:element name="a"/></xs:choice></xs:complexType></xs:element></xs:sequence></xs:complexType>`,
			want:   `UPA violation in local complex type of element "{urn:t}inner": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "anonymous type of a zero-count element",
			schema: `<xs:complexType name="T"><xs:sequence><xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:choice><xs:element name="a"/><xs:element name="a"/></xs:choice></xs:complexType></xs:element></xs:sequence></xs:complexType>`,
			want:   `UPA violation in local complex type of zero-count element "{urn:t}gone": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "group referenced by a type",
			schema: `<xs:group name="G"><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/><xs:element name="c"/></xs:sequence></xs:choice></xs:group><xs:complexType name="T"><xs:group ref="t:G"/></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "extension",
			schema: `<xs:complexType name="B"><xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType><xs:complexType name="D"><xs:complexContent><xs:extension base="t:B"><xs:sequence><xs:element name="a"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}D": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "wildcard and element",
			schema: `<xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="##targetNamespace" minOccurs="0"/><xs:element name="a"/></xs:sequence></xs:complexType></xs:element>`,
			want:   `UPA violation in local complex type of element "{urn:t}root": element "{urn:t}a" and wildcard for an element in namespace ["urn:t"] overlap`,
		},
		{
			name:   "wildcard and wildcard",
			schema: `<xs:complexType name="T"><xs:choice><xs:any namespace="##other"/><xs:any namespace="urn:u ##local urn:a"/></xs:choice></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": wildcard for an element in a non-empty namespace other than "urn:t" and wildcard for an element in namespace ["", "urn:a", "urn:u"] overlap on namespace "urn:a"`,
		},
		{
			name:   "wildcard and wildcard found by the automaton",
			schema: `<xs:complexType name="T"><xs:sequence maxOccurs="unbounded"><xs:any namespace="##other"/><xs:any namespace="##any" minOccurs="0"/></xs:sequence></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": wildcard for an element in a non-empty namespace other than "urn:t" and wildcard for any element overlap`,
		},
		{
			name:   "wildcard and wildcard found in the compiled rows",
			schema: `<xs:complexType name="T"><xs:sequence><xs:sequence><xs:element name="y"/><xs:any namespace="##other" minOccurs="0"/></xs:sequence><xs:any namespace="##other"/></xs:sequence></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": wildcard for an element in a non-empty namespace other than "urn:t" and wildcard for an element in a non-empty namespace other than "urn:t" overlap`,
		},
		{
			name:   "element found by the automaton",
			schema: `<xs:complexType name="T"><xs:sequence><xs:choice maxOccurs="2"><xs:element name="a" maxOccurs="2"/><xs:element name="b"/></xs:choice><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and element "{urn:t}a" overlap`,
		},
		{
			name:   "substitution group head and member",
			schema: `<xs:element name="head"/><xs:element name="member" substitutionGroup="t:head"/><xs:complexType name="T"><xs:choice><xs:element ref="t:head"/><xs:element ref="t:member"/></xs:choice></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}head" and element "{urn:t}member" overlap`,
		},
		{
			name:   "substitution group head and wildcard accepting the head",
			schema: `<xs:element name="head"/><xs:element name="member" substitutionGroup="t:head"/><xs:complexType name="T"><xs:choice><xs:any namespace="##targetNamespace"/><xs:element ref="t:head"/></xs:choice></xs:complexType>`,
			want:   `UPA violation in complex type "{urn:t}T": element "{urn:t}head" and wildcard for an element in namespace ["urn:t"] overlap`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := compileUPAMessage(t, xsd.Bytes("schema.xsd", []byte(upaSchema(test.schema))))
			if got != test.want {
				t.Fatalf("message = %s\nwant      %s", got, test.want)
			}
		})
	}
}

func TestUPADiagnosticNamesTheTypeWhoseDefinitionHoldsTheViolation(t *testing.T) {
	t.Parallel()

	const base = `<xs:complexType name="B"><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/></xs:sequence></xs:choice></xs:complexType>`
	const emptyExtension = `<xs:complexType name="A"><xs:complexContent><xs:extension base="t:B"/></xs:complexContent></xs:complexType>`
	const group = `<xs:group name="G"><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/></xs:sequence></xs:choice></xs:group>`
	const groupBase = `<xs:complexType name="B"><xs:sequence><xs:group ref="t:G"/></xs:sequence></xs:complexType>`
	const chainedGroup = `<xs:group name="G2"><xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/></xs:sequence></xs:choice></xs:group>`
	const chainBase = `<xs:complexType name="B"><xs:sequence><xs:element name="x"/><xs:group ref="t:G1"/></xs:sequence></xs:complexType>`
	const want = `UPA violation in complex type "{urn:t}B": element "{urn:t}a" and element "{urn:t}a" overlap`
	for _, test := range []struct {
		name   string
		schema string
	}{
		{name: "empty extension declared after its base", schema: base + emptyExtension},
		{name: "empty extension declared before its base", schema: emptyExtension + base},
		{name: "group under an empty extension declared after its base", schema: groupBase + emptyExtension + group},
		{name: "group under an empty extension declared before its base", schema: emptyExtension + group + groupBase},
		{name: "nested group chain under an empty extension", schema: emptyExtension + `<xs:group name="G1"><xs:sequence><xs:group ref="t:G2"/></xs:sequence></xs:group>` + chainBase + chainedGroup},
		{name: "anonymous type referencing the base first", schema: `<xs:complexType name="C"><xs:sequence><xs:element name="e"><xs:complexType><xs:complexContent><xs:extension base="t:B"/></xs:complexContent></xs:complexType></xs:element></xs:sequence></xs:complexType>` + base},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := compileUPAMessage(t, xsd.Bytes("schema.xsd", []byte(upaSchema(test.schema)))); got != want {
				t.Fatalf("message = %s\nwant      %s", got, want)
			}
		})
	}
}

func TestUPADiagnosticNamesTheNamespaceTwoTruncatedWildcardListsShare(t *testing.T) {
	t.Parallel()

	shared := "urn:a urn:b urn:c urn:d urn:e urn:f urn:g urn:h"
	schema := upaSchema(`<xs:complexType name="T"><xs:choice><xs:any namespace="` + shared + ` urn:x"/><xs:any namespace="` + shared + ` urn:z urn:x"/></xs:choice></xs:complexType>`)
	const lists = `wildcard for an element in namespace ["urn:a", "urn:b", "urn:c", "urn:d", "urn:e", "urn:f", "urn:g", "urn:h", ...]`
	const want = `UPA violation in complex type "{urn:t}T": ` + lists + ` and ` + lists + ` overlap on namespace "urn:a"`
	if got := compileUPAMessage(t, xsd.Bytes("schema.xsd", []byte(schema))); got != want {
		t.Fatalf("message = %s\nwant      %s", got, want)
	}
}

func TestUPADiagnosticNamesAMemberOnlyWhenTheParticlesShareNoDeclarationName(t *testing.T) {
	t.Parallel()

	members := upaSchema(`<xs:import/><xs:element name="ccc" substitutionGroup="head"/><xs:element name="bbb" substitutionGroup="head"/>`)
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:t"/>` +
		`<xs:element name="head"/>` +
		`<xs:complexType name="T"><xs:choice><xs:element ref="head"/><xs:any namespace="##other"/></xs:choice></xs:complexType>` +
		`</xs:schema>`
	const want = `UPA violation in complex type "T": element "head" and wildcard for an element in a non-empty namespace other than "" overlap on substitution group member "{urn:t}bbb"`
	for _, order := range [][]xsd.SchemaSource{
		{xsd.Bytes("root.xsd", []byte(root)), xsd.Bytes("members.xsd", []byte(members))},
		{xsd.Bytes("members.xsd", []byte(members)), xsd.Bytes("root.xsd", []byte(root))},
	} {
		if got := compileUPAMessage(t, order...); got != want {
			t.Fatalf("message = %s\nwant      %s", got, want)
		}
	}
}

func TestUPADiagnosticListsTheParticlesInOneOrderWhateverTheDeclarationOrder(t *testing.T) {
	t.Parallel()

	const want = `UPA violation in complex type "{urn:t}T": element "{urn:t}a" and wildcard for any element overlap`
	for _, choice := range []string{
		`<xs:element name="a"/><xs:any/>`,
		`<xs:any/><xs:element name="a"/>`,
	} {
		schema := upaSchema(`<xs:complexType name="T"><xs:choice>` + choice + `</xs:choice></xs:complexType>`)
		if got := compileUPAMessage(t, xsd.Bytes("schema.xsd", []byte(schema))); got != want {
			t.Fatalf("%s: message = %s\nwant      %s", choice, got, want)
		}
	}
}
