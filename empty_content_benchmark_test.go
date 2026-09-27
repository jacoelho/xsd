package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkCompileEmptyDerivations(b *testing.B) {
	for _, count := range []int{8, 64} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var schema strings.Builder
			schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="T0"/>`)
			for i := 1; i <= count; i++ {
				derivation := "extension"
				if i%2 == 0 {
					derivation = "restriction"
				}
				fmt.Fprintf(&schema, `<xs:complexType name="T%d"><xs:complexContent><xs:%s base="T%d"/></xs:complexContent></xs:complexType>`, i, derivation, i-1)
			}
			fmt.Fprintf(&schema, `<xs:element name="root" type="T%d"/></xs:schema>`, count)
			source := xsd.Bytes("empty.xsd", []byte(schema.String()))
			if _, err := xsd.Compile(source); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(source); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSessionValidateEmptyContent(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("empty.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence minOccurs="0" maxOccurs="0">
    <xs:element name="child"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name, xml string
		invalid   bool
	}{
		{name: "empty", xml: `<root/>`},
		{name: "unexpected-child", xml: `<root><child/></root>`, invalid: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			var reader strings.Reader
			reader.Reset(tc.xml)
			if err := session.Validate(&reader); (err != nil) != tc.invalid {
				b.Fatalf("warm validation error = %v, invalid = %v", err, tc.invalid)
			}
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(tc.xml)
				if err := session.Validate(&reader); (err != nil) != tc.invalid {
					b.Fatalf("validation error = %v, invalid = %v", err, tc.invalid)
				}
			}
		})
	}
}
