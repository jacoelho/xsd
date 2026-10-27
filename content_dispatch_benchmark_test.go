package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func contentDispatchWidthFourFixture() (schemaText, documentText string) {
	var schema, doc strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">`)
	for i := range 4 {
		fmt.Fprintf(&schema, `<xs:element name="n%d" type="xs:string"/>`, i)
	}
	schema.WriteString(`</xs:choice></xs:complexType></xs:element></xs:schema>`)
	doc.WriteString("<r>")
	for i := range 4_000 {
		fmt.Fprintf(&doc, "<n%d/>", i%4)
	}
	doc.WriteString("</r>")
	return schema.String(), doc.String()
}

func checkContentDispatchWidthFour(b *testing.B, engine *xsd.Engine, doc string) {
	b.Helper()
	if err := engine.Validate(strings.NewReader(doc)); err != nil {
		b.Fatalf("known choice document: %v", err)
	}
	if err := engine.Validate(strings.NewReader("<r><outside/></r>")); !xsderrors.IsConclusiveValidation(err) {
		b.Fatalf("undeclared choice child = %v, want semantic rejection", err)
	}
}

func BenchmarkCompileContentDispatchWidthFour(b *testing.B) {
	schema, doc := contentDispatchWidthFourFixture()
	data := []byte(schema)
	engine, err := xsd.Compile(xsd.Bytes("choice-four.xsd", data))
	if err != nil {
		b.Fatal(err)
	}
	checkContentDispatchWidthFour(b, engine, doc)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(xsd.Bytes("choice-four.xsd", data))
		if err != nil || engine == nil {
			b.Fatalf("compile = %v/%v", engine, err)
		}
	}
	checkContentDispatchWidthFour(b, engine, doc)
}

func BenchmarkSessionContentDispatchWidthFour(b *testing.B) {
	schema, doc := contentDispatchWidthFourFixture()
	engine, err := xsd.Compile(xsd.Bytes("choice-four.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	checkContentDispatchWidthFour(b, engine, doc)
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	reader := strings.NewReader(doc)
	if err := session.Validate(reader); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(doc)
		if err := session.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func contentDispatchWidthSixteenFixture() (schemaText, uniformText, skewText string) {
	var schema, uniform, skew strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">`)
	for i := range 16 {
		fmt.Fprintf(&schema, `<xs:element name="n%d" type="xs:string"/>`, i)
	}
	schema.WriteString(`</xs:choice></xs:complexType></xs:element></xs:schema>`)
	uniform.WriteString("<r>")
	skew.WriteString("<r>")
	for i := range 4_000 {
		fmt.Fprintf(&uniform, "<n%d/>", i%16)
		selected := 0
		if i%20 == 19 {
			selected = 1 + (i/20)%15
		}
		fmt.Fprintf(&skew, "<n%d/>", selected)
	}
	uniform.WriteString("</r>")
	skew.WriteString("</r>")
	return schema.String(), uniform.String(), skew.String()
}

func BenchmarkCompileContentDispatchWidthSixteen(b *testing.B) {
	schema, uniform, skew := contentDispatchWidthSixteenFixture()
	data := []byte(schema)
	engine, err := xsd.Compile(xsd.Bytes("choice-sixteen.xsd", data))
	if err != nil {
		b.Fatal(err)
	}
	checkContentDispatchWidthFour(b, engine, uniform)
	checkContentDispatchWidthFour(b, engine, skew)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(xsd.Bytes("choice-sixteen.xsd", data))
		if err != nil || engine == nil {
			b.Fatalf("compile = %v/%v", engine, err)
		}
	}
	checkContentDispatchWidthFour(b, engine, uniform)
	checkContentDispatchWidthFour(b, engine, skew)
}

func BenchmarkSessionContentDispatchWidthSixteen(b *testing.B) {
	schema, uniform, skew := contentDispatchWidthSixteenFixture()
	for _, test := range []struct {
		name string
		doc  string
	}{{"uniform", uniform}, {"first-skew95", skew}} {
		b.Run(test.name, func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("choice-sixteen.xsd", []byte(schema)))
			if err != nil {
				b.Fatal(err)
			}
			checkContentDispatchWidthFour(b, engine, test.doc)
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			reader := strings.NewReader(test.doc)
			if err := session.Validate(reader); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(test.doc)))
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(test.doc)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
