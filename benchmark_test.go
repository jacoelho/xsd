package xsd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

const benchmarkSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="id" type="xs:int"/>
              <xs:element name="name" type="xs:string"/>
              <xs:element name="amount">
                <xs:simpleType>
                  <xs:restriction base="xs:decimal">
                    <xs:minInclusive value="0"/>
                    <xs:maxInclusive value="999999"/>
                  </xs:restriction>
                </xs:simpleType>
              </xs:element>
            </xs:sequence>
            <xs:attribute name="code" use="required">
              <xs:simpleType>
                <xs:restriction base="xs:string">
                  <xs:pattern value="[A-Z]{2}\d{4}"/>
                </xs:restriction>
              </xs:simpleType>
            </xs:attribute>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func BenchmarkCompileSmallSchema(b *testing.B) {
	for b.Loop() {
		if _, err := Compile(sourceBytes("schema.xsd", []byte(benchmarkSchema))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateRepeatedSmallDocument(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc(100)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRepeatedSmallDocument(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc(100)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSmallInvalidDocument(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := `<rows><row code="bad"><id>x</id><name>alpha</name><amount>-1</amount></row></rows>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err == nil {
			b.Fatal("Validate() succeeded unexpectedly")
		}
	}
}

func BenchmarkValidateDeeplyNestedDocument(b *testing.B) {
	const depth = 128
	engine, err := CompileWithOptions(
		CompileOptions{MaxSchemaDepth: depth*3 + 16},
		sourceBytes("schema.xsd", []byte(deepSchema(depth))),
	)
	if err != nil {
		b.Fatal(err)
	}
	doc := deepDoc(depth)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateDuplicateAttributes(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	doc := duplicateAttributeDoc(1000)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err == nil {
			b.Fatal("Validate() succeeded")
		}
	}
}

func BenchmarkFormatXMLDuplicateAttributes(b *testing.B) {
	doc := duplicateAttributeDoc(1000)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := FormatXML(&out, strings.NewReader(doc)); err == nil {
			b.Fatal("FormatXML() succeeded")
		}
	}
}

func BenchmarkCompileDuplicateSchemaSources(b *testing.B) {
	schema := largeSchemaWithText(64 << 10)
	sources := make([]SchemaSource, 8)
	for i := range sources {
		sources[i] = sourceBytes(fmt.Sprintf("schema%d.xsd", i), []byte(schema))
	}
	b.SetBytes(int64(len(schema) * len(sources)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Compile(sources...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileSchemaText(b *testing.B) {
	schema := largeSchemaWithText(256 << 10)
	b.SetBytes(int64(len(schema)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Compile(sourceBytes("schema.xsd", []byte(schema))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatXMLLargeAttribute(b *testing.B) {
	value := strings.Repeat("a", 64<<10)
	doc := `<root a="` + value + `"/>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := FormatXML(&out, strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatXMLMixedEscapedAttribute(b *testing.B) {
	value := strings.Repeat("abc&amp;&#10;&quot;", 4096)
	doc := `<root a="` + value + `"/>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := FormatXML(&out, strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateGeneratedLargeXML(b *testing.B) {
	schema := os.Getenv("XSD_LARGE_SCHEMA")
	doc := os.Getenv("XSD_LARGE_XML")
	if schema == "" || doc == "" {
		b.Skip("set XSD_LARGE_SCHEMA and XSD_LARGE_XML")
	}
	engine, err := Compile(File(schema))
	if err != nil {
		b.Fatal(err)
	}
	info, err := os.Stat(doc)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(info.Size())
	b.ReportAllocs()
	for b.Loop() {
		f, err := os.Open(doc)
		if err != nil {
			b.Fatal(err)
		}
		validateErr := engine.Validate(f)
		closeErr := f.Close()
		if validateErr != nil {
			b.Fatal(validateErr)
		}
		if closeErr != nil {
			b.Fatal(closeErr)
		}
	}
}

func BenchmarkValidateIdentityConstraints(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(identityBenchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := identityBenchmarkDoc(100)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateIdentityConstraintsRows(b *testing.B) {
	for _, rows := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows_%d", rows), func(b *testing.B) {
			engine, err := Compile(sourceBytes("schema.xsd", []byte(identityBenchmarkSchema)))
			if err != nil {
				b.Fatal(err)
			}
			doc := identityBenchmarkDoc(rows)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRecordIdentityValueIDREFS(b *testing.B) {
	for _, refs := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("refs_%d", refs), func(b *testing.B) {
			value := simpleValue{IDRefs: benchmarkIDREFS(refs)}
			s := &session{path: []string{"root", "refs"}}
			b.ReportAllocs()
			for b.Loop() {
				s.idrefs = s.idrefs[:0]
				s.identityEntries = 0
				if err := s.recordIdentityValue(value, 1, 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateIdentityConstraintsFields(b *testing.B) {
	for _, fields := range []int{1, 3, 8} {
		b.Run(fmt.Sprintf("fields_%d", fields), func(b *testing.B) {
			engine, err := Compile(sourceBytes("schema.xsd", []byte(identityFieldsBenchmarkSchema(fields))))
			if err != nil {
				b.Fatal(err)
			}
			doc := identityFieldsBenchmarkDoc(fields, 100)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompileCountedChoiceDFA(b *testing.B) {
	tests := []struct {
		name      string
		branches  int
		maxOccurs int
	}{
		{name: "small", branches: 4, maxOccurs: 4},
		{name: "medium", branches: 8, maxOccurs: 8},
		{name: "large", branches: 12, maxOccurs: 12},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			schema := countedChoiceDFASchema(tt.branches, tt.maxOccurs)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Compile(sourceBytes("schema.xsd", []byte(schema))); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateConcurrent(b *testing.B) {
	engine, err := Compile(sourceBytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc(100)
	const workers = 8
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for range workers {
		wg.Go(func() {
			for range jobs {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Error(err)
					return
				}
			}
		})
	}
	for b.Loop() {
		jobs <- 0
	}
	close(jobs)
	wg.Wait()
}

func benchmarkDoc(rows int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for range rows {
		b.WriteString(`<row code="AB1234"><id>`)
		b.WriteString("7")
		b.WriteString(`</id><name>alpha</name><amount>42.50</amount></row>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func benchmarkIDREFS(refs int) string {
	var b strings.Builder
	for i := range refs {
		if i > 0 {
			b.WriteByte(' ')
		}
		_, _ = fmt.Fprintf(&b, "id%d", i)
	}
	return b.String()
}

func duplicateAttributeDoc(attrs int) string {
	var b strings.Builder
	b.WriteString("<root")
	for i := range attrs {
		fmt.Fprintf(&b, ` a%d="%d"`, i, i)
	}
	fmt.Fprintf(&b, ` a%d="dup"/>`, attrs-1)
	return b.String()
}

func largeSchemaWithText(n int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:annotation><xs:documentation>`)
	b.WriteString(strings.Repeat("x", n))
	b.WriteString(`</xs:documentation></xs:annotation><xs:element name="root" type="xs:string"/></xs:schema>`)
	return b.String()
}

func deepSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range depth {
		fmt.Fprintf(&b, `<xs:element name="n%d"><xs:complexType><xs:sequence>`, i)
	}
	b.WriteString(`<xs:element name="leaf" type="xs:string"/>`)
	for range depth {
		b.WriteString(`</xs:sequence></xs:complexType></xs:element>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

func deepDoc(depth int) string {
	var b strings.Builder
	for i := range depth {
		fmt.Fprintf(&b, `<n%d>`, i)
	}
	b.WriteString(`<leaf>x</leaf>`)
	for i := depth - 1; i >= 0; i-- {
		fmt.Fprintf(&b, `</n%d>`, i)
	}
	return b.String()
}

const identityBenchmarkSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="ref" type="xs:IDREF" use="optional"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="rowID"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="rowRef" refer="rowID"><xs:selector xpath="row"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`

func identityBenchmarkDoc(rows int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for i := range rows {
		fmt.Fprintf(&b, `<row id="id%d"`, i)
		if i > 0 {
			fmt.Fprintf(&b, ` ref="id%d"`, i-1)
		}
		b.WriteString(`/>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func identityFieldsBenchmarkSchema(fields int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="rows"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType>`)
	for i := range fields {
		fmt.Fprintf(&b, `<xs:attribute name="k%d" type="xs:string" use="required"/>`, i)
	}
	b.WriteString(`</xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="rowKey"><xs:selector xpath="row"/>`)
	for i := range fields {
		fmt.Fprintf(&b, `<xs:field xpath="@k%d"/>`, i)
	}
	b.WriteString(`</xs:key></xs:element></xs:schema>`)
	return b.String()
}

func identityFieldsBenchmarkDoc(fields, rows int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for row := range rows {
		b.WriteString(`<row`)
		for field := range fields {
			fmt.Fprintf(&b, ` k%d="v%d-%d"`, field, row, field)
		}
		b.WriteString(`/>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func countedChoiceDFASchema(branches, maxOccurs int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:choice minOccurs="0" maxOccurs="%d">`, maxOccurs)
	for i := range branches {
		fmt.Fprintf(&b, `<xs:element name="e%d" type="xs:string"/>`, i)
	}
	b.WriteString(`</xs:choice></xs:complexType></xs:element></xs:schema>`)
	return b.String()
}
