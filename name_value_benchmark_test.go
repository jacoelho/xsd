package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateUnicodeQNameValues(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="value" type="xs:QName" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	doc := `<root xmlns:é="urn:unicode">` + strings.Repeat(`<value>é:aé́</value>`, 100) + `</root>`
	benchmarkSessionDocument(b, schema, doc)
}

func BenchmarkOrdinaryFirstValidation(b *testing.B) {
	for _, fixture := range []struct {
		name, schema, document string
	}{
		{"minimal", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`, `<root>x</root>`},
		{"ordinarySmall", benchmarkSchema, benchmarkDoc()},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("first.xsd", []byte(fixture.schema)))
			if err != nil {
				b.Fatal(err)
			}
			warm, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			if validateErr := warm.Validate(strings.NewReader(fixture.document)); validateErr != nil {
				b.Fatal(validateErr)
			}
			b.SetBytes(int64(len(fixture.document)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
				if sessionErr != nil {
					b.Fatal(sessionErr)
				}
				if validateErr := session.Validate(strings.NewReader(fixture.document)); validateErr != nil {
					b.Fatal(validateErr)
				}
			}
		})
	}
}
