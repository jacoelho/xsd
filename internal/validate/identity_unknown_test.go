package validate

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

// A field that selects an attribute lax assessment left without a declaration
// holds an unknown value: it satisfies key presence, equals no value, adds no
// field cardinality, and yields to a typed value on another selected node. A
// keyref row holding one matches no key and reports only the keyref failure.
func TestIdentityUnknownFieldValues(t *testing.T) {
	t.Parallel()
	const (
		duplicate   = "validation.identity /root/row: duplicate value for identity constraint \"values\"; first seen at /root/row"
		multiple    = "validation.identity /root/row: identity constraint \"values\", field 1: selects multiple nodes; at most one is allowed"
		nillableKey = "validation.identity /root/row: identity constraint \"values\", field 1: a key field must not select a nillable element declaration"
		unmatched   = "validation.identity /root/row: keyref value does not match any value of referenced identity constraint "
	)
	keyMissing := func(field string) string {
		return "validation.identity /root/row: identity constraint \"values\", field " + field + ": a key requires a value, but the selected node is absent or nilled"
	}
	tests := []struct {
		name   string
		kind   string
		refer  string
		fields []string
		body   string
		want   []string
	}{
		{name: "unique equal unknown values", kind: "unique", fields: []string{"@l:a"}, body: `<row l:a="1"/><row l:a="1"/>`},
		{name: "unique empty unknown values beside a typed empty value", kind: "unique", fields: []string{"@a|@l:a"}, body: `<row a=""/><row l:a=""/><row l:a=""/>`},
		{name: "typed duplicate around an unknown row", kind: "unique", fields: []string{"@a|@l:a"}, body: `<row a="1"/><row l:a="1"/><row a="1"/>`, want: []string{duplicate}},
		{name: "typed value beside unknown on one node supersedes it", kind: "unique", fields: []string{"@a|@l:a"}, body: `<row a="1" l:a="2"/><row a="1"/>`, want: []string{duplicate}},
		{name: "two unknown values on one node", kind: "unique", fields: []string{"@l:a|@l:b"}, body: `<row l:a="1" l:b="2"/>`},
		{name: "two typed values on one node", kind: "unique", fields: []string{"@a|@b"}, body: `<row a="1" b="2"/>`, want: []string{multiple}},
		{name: "two-field unique with one unknown field", kind: "unique", fields: []string{"@a", "@l:b"}, body: `<row a="1" l:b="1"/><row a="1" l:b="1"/>`},
		{name: "key unknown value present", kind: "key", fields: []string{"@l:a"}, body: `<row l:a="1"/>`},
		{name: "key equal unknown values", kind: "key", fields: []string{"@l:a"}, body: `<row l:a="1"/><row l:a="1"/>`},
		{name: "key unknown attribute missing", kind: "key", fields: []string{"@l:a"}, body: `<row l:a="1"/><row/>`, want: []string{keyMissing("1")}},
		{name: "key absent field outranks unknown field", kind: "key", fields: []string{"@l:a", "@b"}, body: `<row l:a="1"/>`, want: []string{keyMissing("2")}},
		{name: "nilled value beside unknown on one row supersedes it", kind: "key", fields: []string{"@l:a|n"}, body: `<row xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" l:a="1"><n xsi:nil="true"/></row>`, want: []string{keyMissing("1")}},
		{name: "key nillable field beside an unknown field", kind: "key", fields: []string{"n", "@l:a"}, body: `<row l:a="1"><n>x</n></row>`, want: []string{nillableKey}},
		{name: "keyref unknown value equal to a key value", kind: "keyref", refer: "targets", fields: []string{"@l:a"}, body: `<target id="1"/><row l:a="1"/>`, want: []string{unmatched + `"targets"`}},
		{name: "keyref unknown value beside a typed field", kind: "keyref", refer: "pairs", fields: []string{"@a", "@l:b"}, body: `<target id="1" id2="2"/><row a="1" l:b="2"/>`, want: []string{unmatched + `"pairs"`}},
		{name: "keyref unknown value and absent field", kind: "keyref", refer: "pairs", fields: []string{"@a", "@l:b"}, body: `<target id="1" id2="2"/><row l:b="2"/>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			refer := ""
			if test.refer != "" {
				refer = ` refer="` + test.refer + `"`
			}
			var fields strings.Builder
			for _, field := range test.fields {
				fields.WriteString(`<xs:field xpath="` + field + `"/>`)
			}
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:l="urn:lax">
<xs:element name="root"><xs:complexType><xs:sequence>
<xs:element name="target" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:string"/><xs:attribute name="id2" type="xs:string"/></xs:complexType></xs:element>
<xs:element name="row" minOccurs="0" maxOccurs="unbounded"><xs:complexType>
<xs:sequence><xs:element name="n" type="xs:string" nillable="true" minOccurs="0"/></xs:sequence>
<xs:attribute name="a" type="xs:string"/><xs:attribute name="b" type="xs:string"/><xs:anyAttribute namespace="urn:lax" processContents="lax"/>
</xs:complexType></xs:element></xs:sequence></xs:complexType>
<xs:key name="targets"><xs:selector xpath="target"/><xs:field xpath="@id"/></xs:key>
<xs:unique name="pairs"><xs:selector xpath="target"/><xs:field xpath="@id"/><xs:field xpath="@id2"/></xs:unique>
<xs:`+test.kind+` name="values"`+refer+`><xs:selector xpath="row"/>`+fields.String()+`</xs:`+test.kind+`>
</xs:element></xs:schema>`)
			session, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			doc := `<root xmlns:l="urn:lax">` + test.body + `</root>`
			// The second document reverses whether unknown or typed values
			// come first, so a reused session must not carry field state.
			other := `<root xmlns:l="urn:lax"><row a="1" b="1"/><row l:a="1" l:b="1"/><row/></root>`
			for _, sequence := range [][]string{{doc, other, doc}, {other, doc}} {
				var got []string
				seen := false
				for _, input := range sequence {
					diagnostics := identityDiagnostics(t, session.Validate(strings.NewReader(input)))
					if input == doc {
						if seen && !slices.Equal(diagnostics, got) {
							t.Fatalf("reused session diagnostics = %q, want %q", diagnostics, got)
						}
						got, seen = diagnostics, true
					}
				}
				if !slices.Equal(got, test.want) {
					t.Fatalf("diagnostics = %q, want %q", got, test.want)
				}
			}
		})
	}
}

func identityDiagnostics(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var out []string
	for _, item := range xsderrors.Flatten(err) {
		var diagnostic *xsderrors.Error
		if !errors.As(item, &diagnostic) {
			t.Fatalf("diagnostic %v is not *xsderrors.Error", item)
		}
		out = append(out, string(diagnostic.Code())+" "+diagnostic.Path()+": "+diagnostic.Message())
	}
	return out
}
