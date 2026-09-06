package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateQualifiedSimpleValues(b *testing.B) {
	for _, test := range []struct {
		name        string
		typ         string
		declaration string
	}{
		{name: "QName", typ: "xs:QName"},
		{
			name: "NOTATION", typ: "notationType",
			declaration: `<xs:notation name="value" public="value"/>
<xs:simpleType name="notationType"><xs:restriction base="xs:NOTATION">
  <xs:enumeration value="value"/>
</xs:restriction></xs:simpleType>`,
		},
	} {
		b.Run(test.name, func(b *testing.B) {
			schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
%s
<xs:element name="root"><xs:complexType><xs:sequence>
  <xs:element name="item" type="%s" maxOccurs="unbounded"/>
</xs:sequence></xs:complexType></xs:element>
</xs:schema>`, test.declaration, test.typ)
			engine, err := xsd.Compile(xsd.Bytes("qualified-values.xsd", []byte(schema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			document := "<root>" + strings.Repeat("<item>value</item>", 128) + "</root>"
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
