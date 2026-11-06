package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileValueFailuresCarryValueReason(t *testing.T) {
	t.Parallel()

	const head = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	tests := []struct {
		name     string
		schema   string
		code     xsderrors.Code
		contains string
	}{
		{
			name: "enumeration literal outside base",
			schema: head + `<xs:simpleType name="T"><xs:restriction base="xs:int">
<xs:enumeration value="abc"/></xs:restriction></xs:simpleType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet, contains: "invalid integer",
		},
		{
			name:   "element fixed outside type",
			schema: head + `<xs:element name="e" type="xs:int" fixed="abc"/></xs:schema>`,
			code:   xsderrors.CodeSchemaFacet, contains: "invalid",
		},
		{
			name: "whiteSpace loosening of atomic restriction",
			schema: head + `<xs:simpleType name="T"><xs:restriction base="xs:token">
<xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet,
		},
		{
			name: "whiteSpace loosening of replace base",
			schema: head + `<xs:simpleType name="T"><xs:restriction base="xs:normalizedString">
<xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet,
		},
		{
			name: "whiteSpace loosening of list restriction",
			schema: head + `<xs:simpleType name="L"><xs:list itemType="xs:int"/></xs:simpleType>
<xs:simpleType name="T"><xs:restriction base="L">
<xs:whiteSpace value="replace"/></xs:restriction></xs:simpleType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet,
		},
		{
			name: "whiteSpace loosening against fixed base facet",
			schema: head + `<xs:simpleType name="B"><xs:restriction base="xs:string">
<xs:whiteSpace value="collapse" fixed="true"/></xs:restriction></xs:simpleType>
<xs:simpleType name="T"><xs:restriction base="B">
<xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet,
		},
		{
			name: "whiteSpace loosening in simpleContent restriction",
			schema: head + `<xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:token"/></xs:simpleContent></xs:complexType>
<xs:complexType name="T"><xs:simpleContent><xs:restriction base="B">
<xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleContent></xs:complexType></xs:schema>`,
			code: xsderrors.CodeSchemaFacet,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(tt.schema)))
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != tt.code {
				t.Fatalf("Compile() error = %v, want code %s", err, tt.code)
			}
			if !strings.Contains(diagnostic.Message(), tt.contains) {
				t.Fatalf("message = %q, want it to contain %q", diagnostic.Message(), tt.contains)
			}
		})
	}
}

func TestCompileWhitespaceTighteningIsValid(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:simpleType name="T"><xs:restriction base="xs:string"><xs:whiteSpace value="collapse"/></xs:restriction></xs:simpleType>
<xs:simpleType name="U"><xs:restriction base="xs:normalizedString"><xs:whiteSpace value="collapse"/></xs:restriction></xs:simpleType>
</xs:schema>`
	if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}
