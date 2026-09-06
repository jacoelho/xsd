package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicRegexCaretRangeEndpoint(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="caretRange">
    <xs:restriction base="xs:string"><xs:pattern value="[A-^]"/></xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="caretRange"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("caret-range.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, input := range []string{"A", "^"} {
		if err := engine.Validate(strings.NewReader("<root>" + input + "</root>")); err != nil {
			t.Errorf("Validate(%q) error = %v, want nil", input, err)
		}
	}
	if err := engine.Validate(strings.NewReader("<root>@</root>")); err == nil {
		t.Fatal("Validate(\"@\") succeeded, want rejected adjacent nonmember")
	}
}

func TestPublicRegexSurrogateBlocksAreSchemaFacets(t *testing.T) {
	for _, name := range []string{"HighSurrogates", "LowSurrogates", "HighPrivateUseSurrogates"} {
		for _, polarity := range []string{"p", "P"} {
			for _, pattern := range []string{
				"\\" + polarity + "{Is" + name + "}",
				"[\\" + polarity + "{Is" + name + "}]",
			} {
				t.Run(pattern, func(t *testing.T) {
					schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="excluded"><xs:restriction base="xs:string"><xs:pattern value="` + pattern + `"/></xs:restriction></xs:simpleType>
</xs:schema>`
					_, err := xsd.Compile(xsd.Bytes("excluded-block.xsd", []byte(schema)))
					expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet)
				})
			}
		}
	}
}
