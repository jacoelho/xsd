package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestElementFixedAllowsChildElements(t *testing.T) {
	tests := []struct {
		name        string
		fixedValue  string
		expectError bool
	}{
		{
			name:        "fixed empty allows empty child content",
			fixedValue:  "",
			expectError: false,
		},
		{
			name:        "fixed non-empty rejects empty child content",
			fixedValue:  "x",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaXML := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" fixed="%s">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`, tt.fixedValue)

			schema, err := parser.Parse(strings.NewReader(schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			xmlDoc := `<?xml version="1.0"?><root xmlns="http://example.com/test"><child/></root>`
			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, xmlDoc)

			found := false
			for _, violation := range violations {
				if violation.Code == string(errors.ErrElementFixedValue) {
					found = true
					break
				}
			}
			if tt.expectError && !found {
				t.Fatalf("expected fixed-value violation, got %v", violations)
			}
			if !tt.expectError && found {
				t.Fatalf("unexpected fixed-value violation, got %v", violations)
			}
		})
	}
}
