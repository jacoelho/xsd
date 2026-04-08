package parser

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestParserParse(t *testing.T) {
	p := NewParser()
	result, err := p.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:test"
	xmlns:tns="urn:test"
	elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result == nil || result.Schema == nil {
		t.Fatal("Parse() returned nil result")
	}
}

func TestParserParseSequentialResultsStayIndependent(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	resultOne, err := parser.Parse(strings.NewReader(testSchema("urn:one", "first")))
	if err != nil {
		t.Fatalf("first Parse() error = %v", err)
	}
	resultTwo, err := parser.Parse(strings.NewReader(testSchema("urn:two", "second")))
	if err != nil {
		t.Fatalf("second Parse() error = %v", err)
	}

	firstName := model.QName{Namespace: "urn:one", Local: "first"}
	secondName := model.QName{Namespace: "urn:two", Local: "second"}

	if got := resultOne.Schema.TargetNamespace; got != "urn:one" {
		t.Fatalf("first schema targetNamespace = %q, want %q", got, "urn:one")
	}
	if got := resultTwo.Schema.TargetNamespace; got != "urn:two" {
		t.Fatalf("second schema targetNamespace = %q, want %q", got, "urn:two")
	}
	if _, ok := resultOne.Schema.ElementDecls[firstName]; !ok {
		t.Fatalf("first schema missing element %v after second parse", firstName)
	}
	if _, ok := resultOne.Schema.ElementDecls[secondName]; ok {
		t.Fatalf("first schema unexpectedly contains second element %v", secondName)
	}
	if _, ok := resultTwo.Schema.ElementDecls[secondName]; !ok {
		t.Fatalf("second schema missing element %v", secondName)
	}
}

func TestParseWithImportsOptionsWithNilPool(t *testing.T) {
	result, err := ParseWithImportsOptionsWithPool(strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), nil)
	if err != nil {
		t.Fatalf("ParseWithImportsOptionsWithPool() error = %v", err)
	}
	if result == nil || result.Schema == nil {
		t.Fatal("ParseWithImportsOptionsWithPool() returned nil schema")
	}
}

func TestParseWrapsRootElementErrorAsParseError(t *testing.T) {
	_, err := ParseWithImportsOptions(strings.NewReader(`<root/>`))
	if err == nil {
		t.Fatal("ParseWithImportsOptions() error = nil, want parse error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("ParseWithImportsOptions() error type = %T, want *ParseError", err)
	}
}

func testSchema(targetNamespace, elementName string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="%s"
	xmlns:tns="%s"
	elementFormDefault="qualified">
  <xs:element name="%s" type="xs:string"/>
</xs:schema>`, targetNamespace, targetNamespace, elementName)
}
