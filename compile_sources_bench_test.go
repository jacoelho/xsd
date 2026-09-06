package xsd_test

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkCompileSourceDocuments(b *testing.B) {
	for _, shape := range []string{"distinct", "duplicate_identity"} {
		b.Run(shape, func(b *testing.B) {
			sources := make([]xsd.SchemaSource, 128)
			for i := range sources {
				schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r%d" type="xs:string"/></xs:schema>`, i)
				if shape == "duplicate_identity" {
					schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="item" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType><xs:unique name="values"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique></xs:element></xs:schema>`
				}
				sources[i] = xsd.Bytes(fmt.Sprintf("schema-%03d.xsd", i), []byte(schema))
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
