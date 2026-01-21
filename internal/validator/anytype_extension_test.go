package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestAnyTypeExtensionAllowsWildcardContent(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="extAny">
    <xs:complexContent>
      <xs:extension base="xs:anyType"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:extAny"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:foo="urn:foo" foo:attr="1">
  <foo:child/>
</tns:root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations := validateStream(t, v, doc)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
