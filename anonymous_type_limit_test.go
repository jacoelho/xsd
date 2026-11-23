package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// anonymousTypeShape builds a schema holding a given number of anonymous
// types whose interned names do not depend on that number.
type anonymousTypeShape struct {
	schema func(types int) string
	name   string
}

func anonymousTypeShapes() []anonymousTypeShape {
	return []anonymousTypeShape{
		{
			name: "union members",
			schema: func(types int) string {
				var b strings.Builder
				b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:simpleType><xs:union>`)
				for range types - 1 {
					b.WriteString(`<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)
				}
				b.WriteString(`</xs:union></xs:simpleType></xs:element></xs:schema>`)
				return b.String()
			},
		},
		{
			name: "two-name element tree",
			schema: func(types int) string {
				var b strings.Builder
				b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
				writeAnonymousElementTree(&b, "a", types)
				b.WriteString(`</xs:schema>`)
				return b.String()
			},
		},
	}
}

// writeAnonymousElementTree writes an element named name whose anonymous
// complex type holds elements a and b, splitting types anonymous complex types
// among them.
func writeAnonymousElementTree(b *strings.Builder, name string, types int) {
	b.WriteString(`<xs:element name="` + name + `"><xs:complexType>`)
	if rest := types - 1; rest > 0 {
		b.WriteString(`<xs:sequence>`)
		writeAnonymousElementTree(b, "a", (rest+1)/2)
		if rest > 1 {
			writeAnonymousElementTree(b, "b", rest/2)
		}
		b.WriteString(`</xs:sequence>`)
	}
	b.WriteString(`</xs:complexType></xs:element>`)
}

func compileAnonymousTypes(shape anonymousTypeShape, types int, options xsd.CompileOptions) error {
	_, err := xsd.CompileWithOptions(options, xsd.Bytes("schema.xsd", []byte(shape.schema(types))))
	return err
}

func TestAnonymousTypesInternNoNames(t *testing.T) {
	t.Parallel()

	for _, shape := range anonymousTypeShapes() {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			names := 1
			for compileAnonymousTypes(shape, 3, xsd.CompileOptions{MaxSchemaNames: names}) != nil {
				if names > 10_000 {
					t.Fatal("three anonymous types fail to compile at every MaxSchemaNames up to 10000")
				}
				names++
			}
			if err := compileAnonymousTypes(shape, 3_000, xsd.CompileOptions{MaxSchemaNames: names}); err != nil {
				t.Fatalf("3000 anonymous types under MaxSchemaNames %d, enough for 3, error = %v", names, err)
			}
		})
	}
}

func TestAnonymousTypeCountIsBoundedByMaxSchemaInstantiatedNodes(t *testing.T) {
	t.Parallel()

	const types = 3_000
	for _, shape := range anonymousTypeShapes() {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			compile := func(nodes int) error {
				return compileAnonymousTypes(shape, types, xsd.CompileOptions{MaxSchemaInstantiatedNodes: nodes})
			}
			low, high := 1, 1_000_000
			if err := compile(high); err != nil {
				t.Fatalf("Compile(MaxSchemaInstantiatedNodes: %d) error = %v", high, err)
			}
			for low < high {
				mid := low + (high-low)/2
				if compile(mid) == nil {
					high = mid
				} else {
					low = mid + 1
				}
			}
			if low <= types {
				t.Fatalf("%d anonymous types compile under MaxSchemaInstantiatedNodes %d, want each to cost a node", types, low)
			}
			err := compile(low - 1)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit || !strings.Contains(diagnostic.Message(), "MaxSchemaInstantiatedNodes") {
				t.Fatalf("Compile(MaxSchemaInstantiatedNodes: %d) error = %v, want %s naming MaxSchemaInstantiatedNodes", low-1, err, xsderrors.CodeSchemaLimit)
			}
		})
	}
}
