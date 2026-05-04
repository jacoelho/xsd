package xsd

import (
	"fmt"
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
	engine, err := Compile(sourceBytes("schema.xsd", []byte(deepSchema(depth))))
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
