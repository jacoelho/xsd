package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestEarlyYearFacetWhitespace(t *testing.T) {
	for _, tc := range []struct {
		facet    string
		accepted []string
		rejected string
	}{
		{
			facet:    "minInclusive",
			accepted: []string{"0001-02-03T12:00:00", "0001-02-04T12:00:00"},
			rejected: "0001-02-02T12:00:00",
		},
		{
			facet:    "enumeration",
			accepted: []string{"0001-02-03T12:00:00"},
			rejected: "0001-02-03T12:00:01",
		},
	} {
		t.Run(tc.facet, func(t *testing.T) {
			const literal = " \t\r\n0001-02-03T12:00:00 \t\r\n"
			source := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Early"><xs:restriction base="xs:dateTime">
    <xs:` + tc.facet + ` value="` + literal + `"/>
  </xs:restriction></xs:simpleType>
  <xs:element name="root" type="Early"/>
</xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes("early-year.xsd", []byte(source)))
			if err != nil {
				t.Fatalf("Compile(): %v", err)
			}
			for _, accepted := range tc.accepted {
				if validationErr := engine.Validate(strings.NewReader("<root>" + accepted + "</root>")); validationErr != nil {
					t.Fatalf("Validate(%q): %v", accepted, validationErr)
				}
			}
			err = engine.Validate(strings.NewReader("<root>" + tc.rejected + "</root>"))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
		})
	}
}
