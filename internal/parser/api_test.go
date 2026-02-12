package parser

import (
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
