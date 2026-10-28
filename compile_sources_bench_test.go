package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func BenchmarkCompileSourceDocuments(b *testing.B) {
	for _, shape := range []string{"distinct", "identity_constraints"} {
		b.Run(shape, func(b *testing.B) {
			sources := make([]xsd.SchemaSource, 128)
			for i := range sources {
				schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r%d" type="xs:string"/></xs:schema>`, i)
				if shape == "identity_constraints" {
					schema = fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root%d"><xs:complexType><xs:sequence><xs:element name="item" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType><xs:unique name="values%d"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique></xs:element></xs:schema>`, i, i)
				}
				sources[i] = xsd.Bytes(fmt.Sprintf("schema-%03d.xsd", i), []byte(schema))
			}
			if shape == "identity_constraints" {
				requireCompiledUniqueConstraints(b, sources)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(sources...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// requireCompiledUniqueConstraints proves the measured schema set compiles a
// live unique constraint per source, so the benchmark cannot silently measure
// a set that dropped or merged its identity constraints.
func requireCompiledUniqueConstraints(b *testing.B, sources []xsd.SchemaSource) {
	b.Helper()
	engine, err := xsd.Compile(sources...)
	if err != nil {
		b.Fatal(err)
	}
	for _, i := range []int{0, len(sources) / 2, len(sources) - 1} {
		duplicate := fmt.Sprintf("<root%d><item>a</item><item>a</item></root%d>", i, i)
		err := engine.Validate(strings.NewReader(duplicate))
		if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code() != xsderrors.CodeValidationIdentity {
			b.Fatalf("root%d duplicate items = %v; want %s", i, err, xsderrors.CodeValidationIdentity)
		}
		distinct := fmt.Sprintf("<root%d><item>a</item><item>b</item></root%d>", i, i)
		if err := engine.Validate(strings.NewReader(distinct)); err != nil {
			b.Fatalf("root%d distinct items = %v", i, err)
		}
	}
}
