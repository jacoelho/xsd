package loader

import (
	"strings"
	"testing"

	schema "github.com/jacoelho/xsd/internal/parser"
)

// TestSimpleTypeConstraintViolations tests that schemas with invalid simple type constraints
// are rejected during schema validation (not instance validation)

// TestFacetLengthWithMinMaxLength tests that length facet cannot be used with minLength/maxLength
// Per XSD 1.0 Errata E1-17, they are mutually exclusive regardless of derivation step
func TestFacetLengthWithMinMaxLength(t *testing.T) {
	// schema with length=5 and minLength=2 - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:string">
      <xs:length value="5"/>
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser may reject this during parse-time facet checks.
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for length facet used with minLength, but got none")
	} else {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Error(), "length") && strings.Contains(err.Error(), "minLength") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about length/minLength conflict, got: %v", errors)
		}
	}
}

// TestFacetLengthWithMaxLength tests that length facet cannot be used with maxLength
// Per XSD 1.0 Errata E1-17, they are mutually exclusive regardless of derivation step
func TestFacetLengthWithMaxLength(t *testing.T) {
	// schema with length=5 and maxLength=10 - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:string">
      <xs:length value="5"/>
      <xs:maxLength value="10"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser may reject this during parse-time facet checks.
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for length facet used with maxLength, but got none")
	}
}

// TestFacetLengthOnBoolean tests that length facet is not applicable to boolean type
func TestFacetLengthOnBoolean(t *testing.T) {
	// schema with length facet on boolean - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:boolean">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser may reject this during parse-time facet checks.
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for length facet on boolean type, but got none")
	}
}

// TestFacetTotalDigitsOnNonDecimal tests that totalDigits facet is only applicable to decimal types
func TestFacetTotalDigitsOnNonDecimal(t *testing.T) {
	// schema with totalDigits on string - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:string">
      <xs:totalDigits value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser may reject this during parse-time facet checks.
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for totalDigits facet on non-decimal type, but got none")
	}
}

// TestFacetRangeOnNonOrderedType tests that range facets are only applicable to ordered types
func TestFacetRangeOnNonOrderedType(t *testing.T) {
	// schema with maxInclusive on QName (not ordered) - should be invalid
	// QName has OrderedNone, so range facets don't apply
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="TestType">
    <xs:restriction base="xs:QName">
      <xs:maxInclusive value="test"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser might reject this - that's fine, it's still a constraint violation
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for range facet on non-ordered type, but got none")
	} else {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Error(), "ordered") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about ordered types, got: %v", errors)
		}
	}
}

// TestAttributeDefaultValueValidation tests that default attribute values must be valid for the type
func TestAttributeDefaultValueValidation(t *testing.T) {
	// schema with invalid default value for integer attribute - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="count" type="xs:integer" default="not-a-number"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid default attribute value, but got none")
	}
}

// TestAttributeFixedValueValidation tests that fixed attribute values must be valid for the type
func TestAttributeFixedValueValidation(t *testing.T) {
	// schema with invalid fixed value for integer attribute - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="count" type="xs:integer" fixed="not-a-number"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid fixed attribute value, but got none")
	}
}

// TestElementDefaultValueValidation tests that default element values must be valid for the type
func TestElementDefaultValueValidation(t *testing.T) {
	// schema with invalid default value for integer element - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="count" type="xs:integer" default="not-a-number"/>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid default element value, but got none")
	}
}

func TestElementFixedValueUnionValidation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="uid">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="pid" fixed="1">
          <xs:simpleType>
            <xs:union>
              <xs:simpleType>
                <xs:restriction base="xs:positiveInteger">
                  <xs:minInclusive value="8"/>
                  <xs:maxInclusive value="72"/>
                </xs:restriction>
              </xs:simpleType>
              <xs:simpleType>
                <xs:restriction base="xs:NMTOKEN">
                  <xs:enumeration value="small"/>
                  <xs:enumeration value="medium"/>
                  <xs:enumeration value="large"/>
                </xs:restriction>
              </xs:simpleType>
            </xs:union>
          </xs:simpleType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid fixed value in union, but got none")
	}
}

// TestIDTypeDefaultValueRejection tests that ID types cannot have default or fixed values
func TestIDTypeDefaultValueRejection(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		wantError bool
		errMsg    string
	}{
		{
			name: "ID element with default value - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="id" type="xs:ID" default="default-id"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "cannot have default or fixed values",
		},
		{
			name: "ID element with fixed value - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="id" type="xs:ID" fixed="fixed-id"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "cannot have default or fixed values",
		},
		{
			name: "IDREF element with default value - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="ref" type="xs:IDREF" default="default-ref"/>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "ID attribute with default value - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="id" type="xs:ID" default="default-id"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: true,
			errMsg:    "cannot have default or fixed values",
		},
		{
			name: "derived from ID with default value - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="myID">
    <xs:restriction base="xs:ID">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="id" type="myID" default="ABC"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "cannot have default or fixed values",
		},
		{
			name: "string element with default value - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="name" type="xs:string" default="unknown"/>
</xs:schema>`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := schema.ParseWithImports(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			errors := ValidateSchema(result.Schema)
			hasError := len(errors) > 0

			if tt.wantError {
				if !hasError {
					t.Error("expected error but got none")
				} else if tt.errMsg != "" {
					found := false
					for _, e := range errors {
						if strings.Contains(e.Error(), tt.errMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("error should contain %q, got: %v", tt.errMsg, errors)
					}
				}
			} else {
				if hasError {
					t.Errorf("unexpected error: %v", errors)
				}
			}
		})
	}
}

// TestWildcardInvalidNamespace tests that wildcards must have valid namespace constraints
func TestWildcardInvalidNamespace(t *testing.T) {
	// schema with invalid namespace value in wildcard - should be invalid
	// note: The parser might reject this before validation, but we should test
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##invalid"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	result, err := schema.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser might reject this - that's fine, it's still a constraint violation
		return
	}

	errors := ValidateSchema(result.Schema)
	// if parser accepts it, validation should reject it
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid wildcard namespace, but got none")
	}
}
