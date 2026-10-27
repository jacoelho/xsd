package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateKnownAttributeWildcard(b *testing.B) {
	for _, fixture := range []struct {
		name       string
		wildType   string
		declaredID bool
		presentID  bool
	}{
		{name: "WildcardID", wildType: "xs:ID"},
		{name: "AbsentDeclaredID", wildType: "xs:int", declaredID: true},
		{name: "PresentDeclaredID", wildType: "xs:int", declaredID: true, presentID: true},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			declared := ""
			if fixture.declaredID {
				declared = `<xs:attribute name="id" type="xs:ID"/>`
			}
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:attribute name="extra" type="` + fixture.wildType + `"/>
<xs:element name="root"><xs:complexType><xs:sequence>
<xs:element name="item" maxOccurs="unbounded"><xs:complexType>` + declared + `
<xs:anyAttribute processContents="strict"/>
</xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`
			var document strings.Builder
			document.WriteString(`<root>`)
			for i := range 100 {
				document.WriteString(`<item`)
				if fixture.presentID {
					fmt.Fprintf(&document, ` id="id%d"`, i)
				}
				if fixture.wildType == "xs:ID" {
					fmt.Fprintf(&document, ` extra="id%d"`, i)
				} else {
					fmt.Fprintf(&document, ` extra="%d"`, i)
				}
				document.WriteString(`/>`)
			}
			document.WriteString(`</root>`)
			benchmarkSessionDocument(b, schema, document.String())
		})
	}
}

func BenchmarkCompileWildcardAttributeUses(b *testing.B) {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:attribute name="extra" type="xs:int"/>`)
	for i := range 64 {
		fmt.Fprintf(&schema, `<xs:complexType name="T%d"><xs:attribute name="id" type="xs:ID"/><xs:anyAttribute processContents="strict"/></xs:complexType><xs:element name="e%d" type="T%d"/>`, i, i, i)
	}
	schema.WriteString(`</xs:schema>`)
	source := xsd.Bytes("wildcard-attribute-uses.xsd", []byte(schema.String()))
	engine, err := xsd.Compile(source)
	if err != nil {
		b.Fatal(err)
	}
	if validationErr := engine.Validate(strings.NewReader(`<e63 id="id63" extra="63"/>`)); validationErr != nil {
		b.Fatal(validationErr)
	}
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil || engine == nil {
			b.Fatalf("Compile() = %v, %v", engine, err)
		}
	}
}
