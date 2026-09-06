package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateFixedSimpleValues(b *testing.B) {
	for _, test := range []struct {
		typ     string
		lexical string
	}{
		{typ: "string", lexical: "value"},
		{typ: "integer", lexical: "42"},
		{typ: "decimal", lexical: "42.5"},
		{typ: "duration", lexical: "P1D"},
		{typ: "date", lexical: "2026-09-06Z"},
		{typ: "gDay", lexical: "---02+14:00"},
	} {
		b.Run(test.typ, func(b *testing.B) {
			schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="item" type="xs:%s" fixed="%s" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`, test.typ, test.lexical)
			engine, err := xsd.Compile(xsd.Bytes("fixed-values.xsd", []byte(schema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			document := "<root>" + strings.Repeat("<item>"+test.lexical+"</item>", 128) + "</root>"
			reader := strings.NewReader(document)
			b.ReportAllocs()
			b.SetBytes(int64(len(document)))
			for b.Loop() {
				reader.Reset(document)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
