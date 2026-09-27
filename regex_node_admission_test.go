package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicRegexNodeLimitRejectsFacetWithLocation(t *testing.T) {
	const prefix = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="TooMany"><xs:restriction base="xs:string"><xs:pattern value="`
	const suffix = `"/></xs:restriction></xs:simpleType>
</xs:schema>`
	schema := prefix + strings.Repeat("a", 100_001) + suffix

	_, err := xsd.Compile(xsd.Bytes("regex-node-limit.xsd", []byte(schema)))
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("Compile() error = %v, want schema diagnostic", err)
	}
	if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaFacet {
		t.Fatalf("Compile() diagnostic = %s/%s, want schema-compile/schema-facet", diagnostic.Category(), diagnostic.Code())
	}
	if diagnostic.Path() != "regex-node-limit.xsd" || diagnostic.Line() != 2 || diagnostic.Column() == 0 {
		t.Fatalf("Compile() location = %q:%d:%d, want regex-node-limit.xsd at line 2", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
	}
}
