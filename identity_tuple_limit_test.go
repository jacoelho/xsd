package xsd_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestMaxIdentityTupleBytesMeasuresCanonicalText(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		types []string
		attrs string
		limit int64
		// size is the canonical text length XSD Part 2 gives the entry, set
		// when the limit must reject it.
		size int
		id   bool
	}{
		{name: "string at limit", types: []string{"xs:string"}, attrs: `a="abc"`, limit: 3},
		{name: "string over limit", types: []string{"xs:string"}, attrs: `a="abcd"`, limit: 3, size: 4},
		{name: "ID at limit", types: []string{"xs:ID"}, attrs: `a="abc"`, limit: 3, id: true},
		{name: "ID over limit", types: []string{"xs:ID"}, attrs: `a="abcd"`, limit: 3, size: 4, id: true},
		{name: "two fields at limit", types: []string{"xs:string", "xs:string"}, attrs: `a="a" b="b"`, limit: 3},
		{name: "two fields over limit", types: []string{"xs:string", "xs:string"}, attrs: `a="ab" b="b"`, limit: 3, size: 4},
		{name: "list at limit", types: []string{"L"}, attrs: `a="a b"`, limit: 3},
		{name: "empty list", types: []string{"L"}, attrs: `a=""`, limit: 3},
		{name: "list collapses whitespace", types: []string{"L"}, attrs: `a="a  b"`, limit: 3},
		{name: "list over limit", types: []string{"L"}, attrs: `a="a bc"`, limit: 3, size: 4},
		{name: "decimal canonical", types: []string{"xs:decimal"}, attrs: `a="01.50"`, limit: 3},
		{name: "duration canonical", types: []string{"xs:duration"}, attrs: `a="P1D"`, limit: 3},
		{name: "integer canonical", types: []string{"xs:integer"}, attrs: `a="007"`, limit: 1},
		{name: "integer over limit", types: []string{"xs:integer"}, attrs: `a="0010"`, limit: 1, size: 2},
		{name: "gYear at limit", types: []string{"xs:gYear"}, attrs: `a="2020"`, limit: 4},
		{name: "dateTime canonical at limit", types: []string{"xs:dateTime"}, attrs: `a="2020-01-01T00:00:00+01:00"`, limit: 20},
		{name: "dateTime canonical over limit", types: []string{"xs:dateTime"}, attrs: `a="2020-01-01T00:00:00+01:00"`, limit: 19, size: 20},
		{name: "double canonical at limit", types: []string{"xs:double"}, attrs: `a="1"`, limit: 5},
		{name: "double canonical over limit", types: []string{"xs:double"}, attrs: `a="1"`, limit: 4, size: 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var schema strings.Builder
			schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="L"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="item"><xs:complexType>`)
			for i, typ := range tt.types {
				schema.WriteString(`<xs:attribute name="` + string(rune('a'+i)) + `" type="` + typ + `"/>`)
			}
			schema.WriteString(`</xs:complexType></xs:element></xs:sequence></xs:complexType>`)
			if !tt.id {
				schema.WriteString(`<xs:unique name="U"><xs:selector xpath="item"/>`)
				for i := range tt.types {
					schema.WriteString(`<xs:field xpath="@` + string(rune('a'+i)) + `"/>`)
				}
				schema.WriteString(`</xs:unique>`)
			}
			schema.WriteString(`</xs:element></xs:schema>`)
			engine, err := xsd.Compile(xsd.Bytes("tuple.xsd", []byte(schema.String())))
			if err != nil {
				t.Fatal(err)
			}
			err = engine.ValidateWithOptions(strings.NewReader(`<root><item `+tt.attrs+`/></root>`), xsd.ValidateOptions{MaxIdentityTupleBytes: tt.limit})
			if tt.size == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want valid", err)
				}
				return
			}
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			want := ": " + strconv.Itoa(tt.size) + " exceeds MaxIdentityTupleBytes=" + strconv.FormatInt(tt.limit, 10)
			if !ok || xerr.Code() != xsderrors.CodeValidationLimit || !strings.Contains(xerr.Error(), want) {
				t.Fatalf("Validate() = %v, want %s containing %q", err, xsderrors.CodeValidationLimit, want)
			}
		})
	}
}
