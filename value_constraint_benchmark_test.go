package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateChangedTypeConstraint(b *testing.B) {
	for _, constraint := range []string{"default", "fixed"} {
		for _, test := range []struct {
			name    string
			lexical string
			facet   string
		}{
			{name: "canonical", lexical: "1"},
			{name: "noncanonical", lexical: "01"},
			{name: "canonical-pattern", lexical: "01", facet: `<xs:pattern value="1"/>`},
			{name: "source-pattern", lexical: "01", facet: `<xs:pattern value="01"/>`},
		} {
			b.Run(constraint+"/"+test.name, func(b *testing.B) {
				source := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Actual"><xs:restriction base="xs:int">%s</xs:restriction></xs:simpleType>
  <xs:element name="root" type="xs:int" %s="%s"/>
</xs:schema>`, test.facet, constraint, test.lexical)
				engine, err := xsd.Compile(xsd.Bytes("constraint.xsd", []byte(source)))
				if err != nil {
					b.Fatal(err)
				}
				session, err := engine.NewSession(xsd.ValidateOptions{})
				if err != nil {
					b.Fatal(err)
				}
				const document = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual"/>`
				reader := strings.NewReader(document)
				failures := 0
				b.ReportAllocs()
				for b.Loop() {
					reader.Reset(document)
					if session.Validate(reader) != nil {
						failures++
					}
				}
				// Both acceptance and rejection are workloads; report the outcome
				// alongside cost so a semantic correction cannot look like a speedup.
				b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
			})
		}
	}
}

func BenchmarkSessionValidateChangedTypeQualifiedConstraint(b *testing.B) {
	for _, constraint := range []string{"default", "fixed"} {
		for _, test := range []struct {
			name     string
			declared string
			binding  string
		}{
			{name: "QName/bound", declared: "xs:QName", binding: `xmlns:p="urn:value"`},
			{name: "QName/missing", declared: "xs:QName"},
			{name: "QName/rebound", declared: "xs:QName", binding: `xmlns:p="urn:other"`},
			{name: "string-union/missing", declared: "StringFirst"},
		} {
			b.Run(constraint+"/"+test.name, func(b *testing.B) {
				source := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:value">
  <xs:simpleType name="StringFirst"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:simpleType name="Actual"><xs:restriction base="xs:QName"><xs:enumeration value="p:x"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="%s" %s="p:x"/>
</xs:schema>`, test.declared, constraint)
				engine, err := xsd.Compile(xsd.Bytes("qualified-constraint.xsd", []byte(source)))
				if err != nil {
					b.Fatal(err)
				}
				session, err := engine.NewSession(xsd.ValidateOptions{})
				if err != nil {
					b.Fatal(err)
				}
				document := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Actual" ` + test.binding + `/>`
				reader := strings.NewReader(document)
				failures := 0
				b.ReportAllocs()
				for b.Loop() {
					reader.Reset(document)
					if session.Validate(reader) != nil {
						failures++
					}
				}
				b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
			})
		}
	}
}
