package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestQualifiedAttributeDiagnosticsNameExpandedName(t *testing.T) {
	t.Parallel()
	const body = `<xs:element name="r"><xs:complexType>
    <xs:attribute name="x" type="xs:string" fixed="F"/>
    <xs:attribute name="y" type="xs:int"/>
    <xs:attribute name="z" type="xs:string" use="required"/>
    <xs:attribute name="w" type="xs:int"/>
  </xs:complexType></xs:element>`
	for _, tt := range []struct {
		name   string
		schema string
		doc    string
		want   [3]string
	}{
		{
			name:   "qualified",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" elementFormDefault="qualified" attributeFormDefault="qualified">` + body + `</xs:schema>`,
			doc:    `<a:r xmlns:a="urn:a" a:x="G" a:y="notint" a:w="1"/>`,
			want:   [3]string{`"{urn:a}x"`, `"{urn:a}y"`, `"{urn:a}z"`},
		},
		{
			name:   "unqualified",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + body + `</xs:schema>`,
			doc:    `<r x="G" y="notint" w="1"/>`,
			want:   [3]string{`"x"`, `"y"`, `"z"`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes("names.xsd", []byte(tt.schema)))
			if err != nil {
				t.Fatal(err)
			}
			err = engine.ValidateWithOptions(strings.NewReader(tt.doc), xsd.ValidateOptions{MaxErrors: 10})
			leaves := xsderrors.Flatten(err)
			if len(leaves) != 3 {
				t.Fatalf("got %d diagnostics, want 3: %v", len(leaves), err)
			}
			for i, marker := range []string{"expected fixed value", "invalid attribute", "missing required attribute"} {
				msg := leaves[i].Error()
				if !strings.Contains(msg, marker) || !strings.Contains(msg, tt.want[i]) {
					t.Errorf("diagnostic %d = %q, want %q naming %s", i, msg, marker, tt.want[i])
				}
				if tt.name == "qualified" && strings.Contains(msg, `"`+"xyz"[i:i+1]+`"`) {
					t.Errorf("diagnostic %d names a bare local name: %q", i, msg)
				}
			}
		})
	}
}
