package schemair

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestResolveDocumentSetDeterministicInlineDeclarationIDs(t *testing.T) {
	doc := parseDocumentForIRTest(t, manyInlineDeclarationSchema(40))
	docs := &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}

	first, err := Resolve(docs, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() first error = %v", err)
	}
	second, err := Resolve(docs, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Resolve() emitted non-deterministic IR for inline declarations")
	}
}

func BenchmarkResolveManyInlineDeclarations(b *testing.B) {
	doc := parseDocumentForIRTest(b, manyInlineDeclarationSchema(200))
	docs := &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Resolve(docs, ResolveConfig{}); err != nil {
			b.Fatal(err)
		}
	}
}

func manyInlineDeclarationSchema(n int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">`)
	b.WriteString(`<xs:complexType name="Root"><xs:sequence>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<xs:element name="e%d"><xs:complexType><xs:sequence><xs:element name="child%d"><xs:simpleType><xs:restriction base="xs:string"><xs:maxLength value="%d"/></xs:restriction></xs:simpleType></xs:element></xs:sequence><xs:attribute name="a%d"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:attribute></xs:complexType></xs:element>`, i, i, i+1, i)
	}
	b.WriteString(`</xs:sequence></xs:complexType>`)
	b.WriteString(`</xs:schema>`)
	return b.String()
}
