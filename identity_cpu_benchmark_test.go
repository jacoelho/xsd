package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

// These benchmarks keep one public Session warm and reset only the input
// reader between iterations. The schema and instance are built outside the
// timed region so the measurements cover validation and identity lifecycle
// work rather than fixture construction.

func BenchmarkIdentityCPUWideOneRow(b *testing.B) {
	for _, test := range []struct {
		name        string
		constraints int
	}{
		{name: "noidentity", constraints: 0},
		{name: "small", constraints: 1},
		{name: "constraints_128", constraints: 128},
		{name: "constraints_1024", constraints: 1024},
		{name: "constraints_4096", constraints: 4096},
	} {
		b.Run(test.name, func(b *testing.B) {
			schema := identityCPUWideSchema(test.constraints)
			document := identityCPUWideDocument(1)
			benchmarkIdentityCPUSession(b, schema, document, 2)
		})
	}
}

func BenchmarkIdentityCPUWideRows(b *testing.B) {
	for _, constraints := range []int{32, 128, 256} {
		b.Run(fmt.Sprintf("constraints_%d/rows_256", constraints), func(b *testing.B) {
			benchmarkIdentityCPUSession(
				b,
				identityCPUWideSchema(constraints),
				identityCPUWideDocument(256),
				2,
			)
		})
	}
}

func BenchmarkIdentityCPUDisparateFieldNames(b *testing.B) {
	for _, test := range []struct {
		name        string
		constraints int
	}{
		{name: "noidentity", constraints: 0},
		{name: "small", constraints: 1},
		{name: "constraints_256", constraints: 256},
		{name: "constraints_1024", constraints: 1024},
	} {
		b.Run(test.name, func(b *testing.B) {
			schema := identityCPUDisparateFieldSchema(test.constraints)
			document := identityCPUDisparateFieldDocument(256, test.constraints)
			benchmarkIdentityCPUSession(b, schema, document, 3)
		})
	}
}

func BenchmarkIdentityCPUSiblingScopes(b *testing.B) {
	for _, scopes := range []int{32, 128, 512} {
		b.Run(fmt.Sprintf("scopes_%d", scopes), func(b *testing.B) {
			benchmarkIdentityCPUSession(
				b,
				identityCPUSiblingScopeSchema(scopes),
				identityCPUSiblingScopeDocument(scopes),
				2,
			)
		})
	}
}

func BenchmarkIdentityCPUNestedSameConstraint(b *testing.B) {
	for _, depth := range []int{16, 64, 256} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			benchmarkIdentityCPUSession(
				b,
				nestedIdentitySelectionsBenchmarkSchema,
				nestedIdentitySelectionsBenchmarkDoc(depth),
				depth+2,
			)
		})
	}
}

func benchmarkIdentityCPUSession(b *testing.B, schema, document string, maxDepth int) {
	b.Helper()
	engine, err := xsd.Compile(xsd.Bytes("identity-cpu.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: maxDepth})
	if err != nil {
		b.Fatal(err)
	}
	reader := strings.NewReader(document)
	if err := session.Validate(reader); err != nil {
		b.Fatalf("warm validation: %v", err)
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		reader.Reset(document)
		if err := session.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func identityCPUWideSchema(constraints int) string {
	var schema strings.Builder
	schema.Grow(256 + constraints*86)
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="row" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>`)
	for i := range constraints {
		fmt.Fprintf(&schema, `<xs:unique name="u%d"><xs:selector xpath="row"/><xs:field xpath="."/></xs:unique>`, i)
	}
	schema.WriteString(`</xs:element></xs:schema>`)
	return schema.String()
}

func identityCPUWideDocument(rows int) string {
	var document strings.Builder
	document.Grow(13 + rows*28)
	document.WriteString(`<root>`)
	for i := range rows {
		fmt.Fprintf(&document, `<row>v%d</row>`, i)
	}
	document.WriteString(`</root>`)
	return document.String()
}

func identityCPUDisparateFieldSchema(constraints int) string {
	var schema strings.Builder
	schema.Grow(512 + constraints*92)
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">`)
	for i := range constraints {
		fmt.Fprintf(&schema, `<xs:element name="f%d" type="xs:string"/>`, i)
	}
	if constraints == 0 {
		schema.WriteString(`<xs:any processContents="skip" minOccurs="0" maxOccurs="unbounded"/>`)
	}
	schema.WriteString(`</xs:choice></xs:complexType></xs:element></xs:sequence></xs:complexType>`)
	for i := range constraints {
		fmt.Fprintf(&schema, `<xs:unique name="u%d"><xs:selector xpath="row"/><xs:field xpath="f%d"/></xs:unique>`, i, i)
	}
	schema.WriteString(`</xs:element></xs:schema>`)
	return schema.String()
}

func identityCPUDisparateFieldDocument(rows, fields int) string {
	var document strings.Builder
	document.Grow(13 + rows*24)
	document.WriteString(`<root>`)
	for i := range rows {
		field := identityCPUDisparateFieldIndex(i, fields)
		fmt.Fprintf(&document, `<row><f%d>v%d</f%d></row>`, field, i, field)
	}
	document.WriteString(`</root>`)
	return document.String()
}

func identityCPUDisparateFieldIndex(row, fields int) int {
	if fields <= 0 {
		return row
	}
	return row % fields
}

func identityCPUSiblingScopeSchema(scopes int) string {
	var schema strings.Builder
	schema.Grow(256 + scopes*220)
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence>`)
	for i := range scopes {
		fmt.Fprintf(&schema, `<xs:element name="s%d"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType><xs:unique name="u%d"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:unique></xs:element>`, i, i)
	}
	schema.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	return schema.String()
}

func identityCPUSiblingScopeDocument(scopes int) string {
	var document strings.Builder
	document.Grow(13 + scopes*24)
	document.WriteString(`<root>`)
	for i := range scopes {
		fmt.Fprintf(&document, `<s%d id="v%d"/>`, i, i)
	}
	document.WriteString(`</root>`)
	return document.String()
}
