package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestEnumerationValueSpaceTemporalEquivalent(t *testing.T) {
	tests := []struct {
		name          string
		baseType      string
		enumValue     string
		instanceValue string
	}{
		{
			name:          "date",
			baseType:      "date",
			enumValue:     "2000-01-01Z",
			instanceValue: "2000-01-01+00:00",
		},
		{
			name:          "time",
			baseType:      "time",
			enumValue:     "13:20:00Z",
			instanceValue: "13:20:00+00:00",
		},
		{
			name:          "gYearMonth",
			baseType:      "gYearMonth",
			enumValue:     "2001-10Z",
			instanceValue: "2001-10+00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaXML := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="TempEnum">
    <xs:restriction base="xs:%s">
      <xs:enumeration value="%s"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="TempEnum"/>
</xs:schema>`, tt.baseType, tt.enumValue)

			schema, err := parser.Parse(strings.NewReader(schemaXML))
			if err != nil {
				t.Fatalf("parse schema: %v", err)
			}

			v := New(mustCompile(t, schema))
			doc := fmt.Sprintf("<root>%s</root>", tt.instanceValue)
			violations := validateStream(t, v, doc)
			if len(violations) > 0 {
				for _, v := range violations {
					t.Logf("Violation: [%s] %s at %s", v.Code, v.Message, v.Path)
				}
				t.Fatalf("expected no violations, got %d", len(violations))
			}
		})
	}
}

func TestFixedValueTemporalEquivalent(t *testing.T) {
	tests := []struct {
		name          string
		baseType      string
		fixedValue    string
		instanceValue string
	}{
		{
			name:          "date",
			baseType:      "date",
			fixedValue:    "2000-01-01Z",
			instanceValue: "2000-01-01+00:00",
		},
		{
			name:          "time",
			baseType:      "time",
			fixedValue:    "13:20:00Z",
			instanceValue: "13:20:00+00:00",
		},
		{
			name:          "gYearMonth",
			baseType:      "gYearMonth",
			fixedValue:    "2001-10Z",
			instanceValue: "2001-10+00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaXML := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:%s" fixed="%s"/>
</xs:schema>`, tt.baseType, tt.fixedValue)

			schema, err := parser.Parse(strings.NewReader(schemaXML))
			if err != nil {
				t.Fatalf("parse schema: %v", err)
			}

			v := New(mustCompile(t, schema))
			doc := fmt.Sprintf("<root>%s</root>", tt.instanceValue)
			violations := validateStream(t, v, doc)
			if len(violations) > 0 {
				for _, v := range violations {
					t.Logf("Violation: [%s] %s at %s", v.Code, v.Message, v.Path)
				}
				t.Fatalf("expected no violations, got %d", len(violations))
			}
		})
	}
}
