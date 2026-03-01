package parser

import (
	"errors"
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
	if result == nil || result.Schema == nil {
		t.Fatal("Parse() returned nil result")
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
