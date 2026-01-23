package loader

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		// parser may reject this during parse-time facet checks.
		return
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for totalDigits facet on non-decimal type, but got none")
	}
}

func TestKeyrefAnonymousFieldTypesIncompatible(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:keys"
           targetNamespace="urn:keys"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a">
          <xs:simpleType>
            <xs:restriction base="xs:int"/>
          </xs:simpleType>
        </xs:element>
        <xs:element name="b">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="tns:a"/>
      <xs:field xpath="."/>
    </xs:key>
    <xs:keyref name="kref" refer="tns:k">
      <xs:selector xpath="tns:b"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatalf("expected keyref field type compatibility error")
	}
	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "not compatible") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected keyref compatibility error, got %v", errors)
	}
}

func TestElementDefaultEmptyViolatesMinLength(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:minLength value="1"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatalf("expected invalid default value error for empty element default")
	}
}

func TestAttributeDefaultEmptyViolatesMinLength(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="attr" default="">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:minLength value="1"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:attribute>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatalf("expected invalid default value error for empty attribute default")
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Error("Expected schema validation error for invalid fixed attribute value, but got none")
	}
}

func TestQNameEnumerationDefaultValueNamespaceEquivalent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:a"
           xmlns:q="urn:a"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameType">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:foo"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:QNameType" default="q:foo"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("Expected schema to be valid, got errors: %v", errors)
	}
}

func TestQNameEnumerationDefaultValueNamespaceMismatch(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:a"
           xmlns:q="urn:b"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameType">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:foo"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:QNameType" default="q:foo"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for QName enumeration mismatch, but got none")
	}
}

func TestDefaultValueInheritsBaseFacets(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string">
      <xs:minLength value="3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="tns:Base">
      <xs:maxLength value="5"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Derived" default="aa"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for default violating base facets, but got none")
	}
}

func TestListDefaultValueViolatesLengthFacet(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedIntList">
    <xs:restriction base="tns:IntList">
      <xs:length value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:RestrictedIntList" default="1 2 3"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for list default length violation, but got none")
	}
}

func TestUnionDefaultValueViolatesEnumerationFacet(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="tns:BaseUnion">
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:RestrictedUnion" default="2"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for union default enumeration violation, but got none")
	}
}

func TestKeyFieldSelectsNillableElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" nillable="true" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for nillable key field, but got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "nillable") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected nillable key field error, got: %v", errors)
	}
}

func TestKeyrefFieldSelectsNillableElement(t *testing.T) {
	// Per XSD 1.0 spec, keyref fields CAN select nillable elements.
	// Nil values in keyref fields cause the tuple to be excluded from the check.
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="keyItem" type="xs:string"/>
        <xs:element name="refItem" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:keyItem"/>
      <xs:field xpath="."/>
    </xs:key>
    <xs:keyref name="itemRef" refer="tns:itemKey">
      <xs:selector xpath="tns:refItem"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("Expected no schema validation error for nillable keyref field, got: %v", errors)
	}
}

func TestUniqueFieldSelectsNillableElement(t *testing.T) {
	// Per XSD 1.0 spec, unique fields CAN select nillable elements.
	// Nil values in unique fields cause the tuple to be excluded from the uniqueness check.
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="itemUnique">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("Expected no schema validation error for nillable unique field, got: %v", errors)
	}
}

func TestUniqueAllowsMixedContentField(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="sub">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="idelt">
                <xs:complexType mixed="true">
                  <xs:attribute name="attr"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="test">
      <xs:selector xpath="sub"/>
      <xs:field xpath="idelt"/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("expected schema to be valid, got: %v", errors)
	}
}

// TestElementDefaultValueValidation tests that default element values must be valid for the type
func TestElementDefaultValueValidation(t *testing.T) {
	// schema with invalid default value for integer element - should be invalid
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="count" type="xs:integer" default="not-a-number"/>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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
		errMsg    string
		wantError bool
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
			name: "indirectly derived from ID with default value - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A">
    <xs:restriction base="xs:ID"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="A"/>
  </xs:simpleType>
  <xs:element name="id" type="B" default="ABC"/>
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
			result, err := parser.ParseWithImports(strings.NewReader(tt.schemaXML))
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

func TestMultipleIDAttributesViaDerivedTypes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A">
    <xs:restriction base="xs:ID"/>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="A"/>
  </xs:simpleType>
  <xs:complexType name="CT">
    <xs:attribute name="id1" type="B"/>
    <xs:attribute name="id2" type="B"/>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for multiple derived ID attributes, but got none")
	}
}

func TestExtensionDuplicateAttributeRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:attribute name="a" type="xs:string"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for duplicate extension attribute, but got none")
	}
}

func TestExtensionDuplicateAttributeGroupRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:attributeGroup name="AG">
    <xs:attribute name="a" type="xs:string"/>
  </xs:attributeGroup>
  <xs:complexType name="Base">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:attributeGroup ref="tns:AG"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for duplicate extension attribute via group, but got none")
	}
}

func TestGYearFacetValueSpaceEqualityAccepted(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="BaseYear">
    <xs:restriction base="xs:gYear">
      <xs:minInclusive value="2000Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="DerivedYear">
    <xs:restriction base="tns:BaseYear">
      <xs:minInclusive value="2000+00:00"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("expected schema to be valid, got: %v", errors)
	}
}

func TestGYearFacetValueSpaceOrderingRejectsDerived(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="BaseYear">
    <xs:restriction base="xs:gYear">
      <xs:minInclusive value="2000Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="DerivedYear">
    <xs:restriction base="tns:BaseYear">
      <xs:minInclusive value="1999Z"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for derived gYear facet ordering, but got none")
	}
}

func TestAnyAttributeIntersectionInvalid(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attributeGroup name="AG1">
    <xs:anyAttribute namespace="##targetNamespace"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="AG2">
    <xs:anyAttribute namespace="##local"/>
  </xs:attributeGroup>
  <xs:complexType name="CT">
    <xs:attributeGroup ref="tns:AG1"/>
    <xs:attributeGroup ref="tns:AG2"/>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected schema validation error for anyAttribute intersection, but got none")
	}
}

func TestAnyAttributeIntersectionValid(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attributeGroup name="AG1">
    <xs:anyAttribute namespace="##any"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="AG2">
    <xs:anyAttribute namespace="##targetNamespace"/>
  </xs:attributeGroup>
  <xs:complexType name="CT">
    <xs:attributeGroup ref="tns:AG1"/>
    <xs:attributeGroup ref="tns:AG2"/>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("expected schema to be valid, got: %v", errors)
	}
}

func TestUPAWithGroupRefInChoiceInvalid(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="CT">
    <xs:choice>
      <xs:group ref="tns:G"/>
      <xs:element name="a" type="xs:string"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) == 0 {
		t.Fatal("Expected UPA validation error for groupRef choice, but got none")
	}
}

func TestUPAWithGroupRefValid(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="CT">
    <xs:sequence>
      <xs:group ref="tns:G"/>
      <xs:element name="c" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	errors := ValidateSchema(result.Schema)
	if len(errors) != 0 {
		t.Fatalf("expected schema to be valid, got: %v", errors)
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

	result, err := parser.ParseWithImports(strings.NewReader(schemaXML))
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
