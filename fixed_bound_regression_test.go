package xsd_test

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestCompileAllowsFixedValueFromNearestInheritedOrderedFacet(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		facet string
		older string
		fixed string
	}{
		{name: "minInclusive", facet: "minInclusive", older: "1", fixed: "2"},
		{name: "maxInclusive", facet: "maxInclusive", older: "10", fixed: "9"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:%s value="%s"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Middle"><xs:restriction base="Base"><xs:%s value="%s" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Leaf"><xs:restriction base="Middle"><xs:%s value="%s"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="Leaf"/>
</xs:schema>`, test.facet, test.older, test.facet, test.fixed, test.facet, test.fixed)
			if _, err := xsd.Compile(xsd.Bytes("fixed-bound.xsd", []byte(schema))); err != nil {
				t.Fatalf("same value as nearest fixed declaration was rejected: %v", err)
			}
		})
	}
}
