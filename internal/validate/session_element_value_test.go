package validate

import (
	"strings"
	"testing"
)

func TestFixedElementDurationUsesValueSpaceEquality(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:duration" fixed="P1D"/>
</xs:schema>`)

	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "equivalent spelling", doc: `<root>PT24H</root>`, want: true},
		{name: "different value", doc: `<root>P2D</root>`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			err = s.Validate(strings.NewReader(test.doc))
			if (err == nil) != test.want {
				t.Fatalf("Validate(%s) error = %v, want valid=%v", test.doc, err, test.want)
			}
		})
	}
}

func TestFixedElementDurationUnionUsesValueSpaceEquality(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="durationOrString">
    <xs:union memberTypes="xs:duration xs:string"/>
  </xs:simpleType>
  <xs:element name="root" type="durationOrString" fixed="P1D"/>
</xs:schema>`)
	s, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(strings.NewReader(`<root>PT24H</root>`)); err != nil {
		t.Fatalf("Validate() error = %v, want valid duration-union value", err)
	}
}

func TestFixedMixedElementRetainsLexicalComparison(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" fixed="a  b">
    <xs:complexType mixed="true"/>
  </xs:element>
</xs:schema>`)

	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "same lexical text", doc: `<root>a  b</root>`, want: true},
		{name: "normalized text differs", doc: `<root>a b</root>`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			err = s.Validate(strings.NewReader(test.doc))
			if (err == nil) != test.want {
				t.Fatalf("Validate(%s) error = %v, want valid=%v", test.doc, err, test.want)
			}
		})
	}
}
