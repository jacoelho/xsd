package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkIdentityDispatchOpposing(b *testing.B) {
	for _, shape := range []string{"fields256", "union256", "attributeOnly256", "terminalMiss256"} {
		b.Run(shape, func(b *testing.B) {
			schema, document := identityDispatchOpposingFixture(shape)
			benchmarkIdentityCPUSession(b, schema, document, 3)
		})
	}
}

func identityDispatchOpposingFixture(shape string) (schemaText, documentText string) {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">`)
	for i := range 256 {
		fmt.Fprintf(&schema, `<xs:element name="f%d" type="xs:string"/>`, i)
	}
	schema.WriteString(`<xs:element name="other" type="xs:string"/></xs:choice><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element></xs:sequence></xs:complexType>`)
	if shape == "attributeOnly256" || shape == "terminalMiss256" {
		for i := range 256 {
			fmt.Fprintf(&schema, `<xs:unique name="u%d"><xs:selector xpath="row"/><xs:field xpath="`, i)
			if shape == "attributeOnly256" {
				schema.WriteString(`@id`)
			} else {
				schema.WriteString(`f0`)
			}
			schema.WriteString(`"/></xs:unique>`)
		}
	} else {
		schema.WriteString(`<xs:unique name="u"><xs:selector xpath="row"/>`)
		if shape == "union256" {
			schema.WriteString(`<xs:field xpath="`)
			for i := range 256 {
				if i > 0 {
					schema.WriteByte('|')
				}
				fmt.Fprintf(&schema, "f%d", i)
			}
			schema.WriteString(`"/>`)
		} else {
			for i := range 256 {
				fmt.Fprintf(&schema, `<xs:field xpath="f%d"/>`, i)
			}
		}
		schema.WriteString(`</xs:unique>`)
	}
	schema.WriteString(`</xs:element></xs:schema>`)
	var document strings.Builder
	document.WriteString(`<root>`)
	for i := range 128 {
		switch shape {
		case "attributeOnly256":
			fmt.Fprintf(&document, `<row id="v%d"/>`, i)
		case "terminalMiss256":
			fmt.Fprintf(&document, `<row><other>v%d</other></row>`, i)
		default:
			fmt.Fprintf(&document, `<row><f255>v%d</f255></row>`, i)
		}
	}
	document.WriteString(`</root>`)
	return schema.String(), document.String()
}

func BenchmarkIdentityDispatchFieldScaling(b *testing.B) {
	for _, test := range []struct {
		fields, children, attributes int
	}{
		{fields: 4096},
		{fields: 4096, children: 512},
		{fields: 16384, children: 512},
		{fields: 65536},
		{fields: 65536, children: 512},
		{fields: 65536, children: 4096},
		{fields: 65536, attributes: 4096},
	} {
		b.Run(fmt.Sprintf("fields_%d/children_%d/attributes_%d", test.fields, test.children, test.attributes), func(b *testing.B) {
			schema, document := identityDispatchFieldScalingFixture(test.fields, test.children, test.attributes)
			engine, err := xsd.CompileWithOptions(xsd.CompileOptions{
				MaxSchemaTokenBytes:        16 << 20,
				MaxSchemaSourceBytes:       16 << 20,
				MaxSchemaTotalBytes:        16 << 20,
				MaxSchemaInstantiatedNodes: 200_000,
			}, xsd.Bytes("field-scaling.xsd", []byte(schema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{
				MaxIdentityEntries:    test.fields + 1,
				MaxInstanceAttributes: max(1, test.attributes),
			})
			if err != nil {
				b.Fatal(err)
			}
			reader := strings.NewReader(document)
			if validateErr := session.Validate(reader); validateErr != nil {
				b.Fatalf("incomplete unique tuple must be valid: %v", validateErr)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(document)))
			b.ResetTimer()
			for b.Loop() {
				reader.Reset(document)
				if validateErr := session.Validate(reader); validateErr != nil {
					b.Fatal(validateErr)
				}
			}
		})
	}
}

func identityDispatchFieldScalingFixture(fields, children, attributes int) (schemaText, documentText string) {
	var schema, document strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range children {
		fmt.Fprintf(&schema, `<xs:element name="f%d" type="xs:string"/>`, i)
	}
	schema.WriteString(`<xs:element name="root"><xs:complexType><xs:sequence><xs:any processContents="strict" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>`)
	document.WriteString(`<root`)
	for i := range attributes {
		fmt.Fprintf(&schema, `<xs:attribute name="a%d" type="xs:string"/>`, i)
		fmt.Fprintf(&document, ` a%d="v"`, i)
	}
	document.WriteByte('>')
	schema.WriteString(`</xs:complexType><xs:unique name="u"><xs:selector xpath="."/>`)
	for i := range fields {
		fmt.Fprintf(&schema, `<xs:field xpath="f%d"/>`, i)
	}
	schema.WriteString(`</xs:unique></xs:element></xs:schema>`)
	for i := range children {
		fmt.Fprintf(&document, `<f%d>x</f%d>`, i, i)
	}
	document.WriteString(`</root>`)
	return schema.String(), document.String()
}

func BenchmarkIdentityDispatchFirstUse(b *testing.B) {
	for _, count := range []int{0, 1, 2, 128, 4096} {
		b.Run(fmt.Sprintf("constraints_%d", count), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("first-use.xsd", []byte(identityCPUWideSchema(count))))
			if err != nil {
				b.Fatal(err)
			}
			document := identityCPUWideDocument(1)
			reader := strings.NewReader(document)
			warm, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			if validateErr := warm.Validate(reader); validateErr != nil {
				b.Fatal(validateErr)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fresh, sessionErr := engine.NewSession(xsd.ValidateOptions{})
				if sessionErr != nil {
					b.Fatal(sessionErr)
				}
				reader.Reset(document)
				if validateErr := fresh.Validate(reader); validateErr != nil {
					b.Fatal(validateErr)
				}
			}
		})
	}
}

func BenchmarkIdentityDispatchNoMatchGrowth(b *testing.B) {
	for _, depth := range []int{1, 64} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			var schema, document strings.Builder
			schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="root" minOccurs="0"/></xs:sequence><xs:attribute name="marker" type="xs:string"/></xs:complexType><xs:unique name="u"><xs:selector xpath="."/>`)
			for field := range 1024 {
				fmt.Fprintf(&schema, `<xs:field xpath="f%d"/>`, field)
			}
			schema.WriteString(`</xs:unique></xs:element></xs:schema>`)
			for range depth {
				document.WriteString(`<root marker="x">`)
			}
			for range depth {
				document.WriteString(`</root>`)
			}
			engine, err := xsd.Compile(xsd.Bytes("no-match-growth.xsd", []byte(schema.String())))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth, MaxIdentityEntries: 65537})
			if err != nil {
				b.Fatal(err)
			}
			doc := document.String()
			reader := strings.NewReader(doc)
			if validateErr := session.Validate(reader); validateErr != nil {
				b.Fatal(validateErr)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			b.ResetTimer()
			for b.Loop() {
				reader.Reset(doc)
				if validateErr := session.Validate(reader); validateErr != nil {
					b.Fatal(validateErr)
				}
			}
		})
	}
}
