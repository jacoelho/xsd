package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestValidateAttributeFixedValueSpace(t *testing.T) {
	tests := []struct {
		name     string
		attrType string
		fixed    string
		value    string
	}{
		{
			name:     "decimal fixed value matches in value space",
			attrType: "xs:decimal",
			fixed:    "1.0",
			value:    "1.00",
		},
		{
			name:     "boolean fixed value matches in value space",
			attrType: "xs:boolean",
			fixed:    "true",
			value:    "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaXML := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="attr" type="%s" fixed="%s"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`, tt.attrType, tt.fixed)

			schema, err := parser.Parse(strings.NewReader(schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			xmlDoc := fmt.Sprintf(`<?xml version="1.0"?><root xmlns="http://example.com/test" attr=%q/>`, tt.value)
			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, xmlDoc)
			if len(violations) > 0 {
				t.Fatalf("expected no violations, got %v", violations)
			}
		})
	}
}
