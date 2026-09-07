package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestUnionEnumerationPreservesStringMemberWhitespace(t *testing.T) {
	t.Parallel()

	// XSD 1.0 Datatypes 4.3.6 delegates union whitespace normalization to
	// its members; the first member here is xs:string, which preserves it.
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Members"><xs:union memberTypes="xs:string xs:int"/></xs:simpleType>
  <xs:simpleType name="Spaced"><xs:restriction base="Members"><xs:enumeration value="  x  "/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Spaced"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("union.xsd", []byte(source)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		content string
		valid   bool
	}{
		{content: "  x  ", valid: true},
		{content: "x", valid: false},
		{content: "  7  ", valid: false},
	} {
		err := engine.Validate(strings.NewReader("<root>" + test.content + "</root>"))
		if (err == nil) != test.valid {
			t.Fatalf("union enumeration with %q: error = %v, want valid %v", test.content, err, test.valid)
		}
	}
}
