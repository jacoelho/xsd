package validator

import (
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestValidateStreamReleasesAutomataOnError(t *testing.T) {
	depth := 8
	schema := nestedSchema(depth)
	document := nestedDocument(depth)

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	if _, err := v.ValidateStream(strings.NewReader(document)); err == nil {
		t.Fatalf("ValidateStream() error = nil, want error")
	}
	_, _ = v.ValidateStream(strings.NewReader(document))

	allocs := testing.AllocsPerRun(50, func() {
		_, _ = v.ValidateStream(strings.NewReader(document))
	})
	if allocs > 93 {
		t.Fatalf("allocs per run = %.2f, want <= 93", allocs)
	}
}

func nestedSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">`)
	for i := 0; i < depth; i++ {
		name := "n" + strconv.Itoa(i+1)
		b.WriteString("<xs:complexType><xs:sequence>")
		if i == depth-1 {
			b.WriteString(`<xs:element name="` + name + `" type="xs:string"/>`)
			continue
		}
		b.WriteString(`<xs:element name="` + name + `">`)
	}
	for i := depth - 2; i >= 0; i-- {
		b.WriteString("</xs:sequence></xs:complexType></xs:element>")
	}
	b.WriteString("</xs:sequence></xs:complexType></xs:element></xs:schema>")
	return b.String()
}

func nestedDocument(depth int) string {
	var b strings.Builder
	b.WriteString(`<root xmlns="urn:test">`)
	for i := 0; i < depth; i++ {
		name := "n" + strconv.Itoa(i+1)
		b.WriteString("<" + name + ">")
	}
	return b.String()
}
