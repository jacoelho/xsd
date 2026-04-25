package schemaast

import (
	"errors"
	"fmt"
	"strings"
	"testing"
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
	if result == nil || result.Document == nil {
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

	firstName := QName{Namespace: "urn:one", Local: "first"}
	secondName := QName{Namespace: "urn:two", Local: "second"}

	if got := resultOne.Document.TargetNamespace; got != "urn:one" {
		t.Fatalf("first document targetNamespace = %q, want %q", got, "urn:one")
	}
	if got := resultTwo.Document.TargetNamespace; got != "urn:two" {
		t.Fatalf("second document targetNamespace = %q, want %q", got, "urn:two")
	}
	if !documentHasElement(resultOne.Document, firstName) {
		t.Fatalf("first document missing element %v after second parse", firstName)
	}
	if documentHasElement(resultOne.Document, secondName) {
		t.Fatalf("first document unexpectedly contains second element %v", secondName)
	}
	if !documentHasElement(resultTwo.Document, secondName) {
		t.Fatalf("second document missing element %v", secondName)
	}
}

func TestParseDocumentWithImportsOptionsWithNilPool(t *testing.T) {
	result, err := ParseDocumentWithImportsOptionsWithPool(strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), nil)
	if err != nil {
		t.Fatalf("ParseDocumentWithImportsOptionsWithPool() error = %v", err)
	}
	if result == nil || result.Document == nil {
		t.Fatal("ParseDocumentWithImportsOptionsWithPool() returned nil document")
	}
}

func TestParseWrapsRootElementErrorAsParseError(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`<root/>`))
	if err == nil {
		t.Fatal("ParseDocumentWithImportsOptions() error = nil, want parse error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("ParseDocumentWithImportsOptions() error type = %T, want *ParseError", err)
	}
}

func documentHasElement(doc *SchemaDocument, name QName) bool {
	if doc == nil {
		return false
	}
	for _, decl := range doc.Decls {
		if decl.Kind == DeclElement && decl.Name == name {
			return true
		}
	}
	return false
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
