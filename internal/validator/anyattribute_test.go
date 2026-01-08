package validator

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
)

// TestAnyAttributeNamespaceMatching tests namespace matching for anyAttribute
func TestAnyAttributeNamespaceMatching(t *testing.T) {
	tests := []struct {
		name        string
		schemaXML   string
		xmlDoc      string
		expectValid bool
		description string
	}{
		{
			name: "##any allows any namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:attr="value"/>`,
			expectValid: true,
			description: "##any should allow attributes from any namespace",
		},
		{
			name: "##other allows non-target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:attr="value"/>`,
			expectValid: true,
			description: "##other should allow attributes from other namespaces",
		},
		{
			name: "##other rejects target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
			expectValid: false,
			description: "##other should reject attributes from target namespace",
		},
		{
			name: "##other rejects empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      attr="value"/>`,
			expectValid: false,
			description: "##other should reject attributes with no namespace",
		},
		{
			name: "##targetNamespace allows only target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
			expectValid: true,
			description: "##targetNamespace should allow attributes from target namespace",
		},
		{
			name: "##targetNamespace rejects other namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:attr="value"/>`,
			expectValid: false,
			description: "##targetNamespace should reject attributes from other namespaces",
		},
		{
			name: "##local allows only empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##local" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      attr="value"/>`,
			expectValid: true,
			description: "##local should allow attributes with no namespace",
		},
		{
			name: "##local rejects namespaced attributes",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##local" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:attr="value"/>`,
			expectValid: false,
			description: "##local should reject attributes with namespaces",
		},
		{
			name: "namespace list allows listed namespaces",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="http://example.com/ns1 http://example.com/ns2" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:ns1="http://example.com/ns1"
      ns1:attr="value"/>`,
			expectValid: true,
			description: "Namespace list should allow attributes from listed namespaces",
		},
		{
			name: "namespace list rejects non-listed namespaces",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="http://example.com/ns1 http://example.com/ns2" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:attr="value"/>`,
			expectValid: false,
			description: "Namespace list should reject attributes from non-listed namespaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
				t.Fatalf("Validate schema: %v", validationErrors)
			}

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			valid := len(violations) == 0
			if valid != tt.expectValid {
				if tt.expectValid {
					t.Errorf("%s\nExpected valid, got invalid:\n", tt.description)
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				} else {
					t.Errorf("%s\nExpected invalid, got valid", tt.description)
				}
			}
		})
	}
}

// TestAnyAttributeProcessContents tests processContents behavior
func TestAnyAttributeProcessContents(t *testing.T) {
	tests := []struct {
		name        string
		schemaXML   string
		xmlDoc      string
		expectValid bool
		description string
	}{
		{
			name: "strict requires declaration",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##any" processContents="strict"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:undeclared="value"/>`,
			expectValid: false,
			description: "strict mode should require attribute declaration",
		},
		{
			name: "strict validates declared attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attribute name="testAttr" type="xs:integer"/>
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="strict"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:testAttr="42"/>`,
			expectValid: true,
			description: "strict mode should validate declared attribute",
		},
		{
			name: "strict rejects invalid value",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attribute name="testAttr" type="xs:integer"/>
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="strict"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:testAttr="not-a-number"/>`,
			expectValid: false,
			description: "strict mode should reject invalid attribute value",
		},
		{
			name: "lax validates if declaration found",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:attribute name="testAttr" type="xs:integer"/>
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##any" processContents="lax"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:testAttr="42"/>`,
			expectValid: true,
			description: "lax mode should validate if declaration found",
		},
		{
			name: "lax allows undeclared attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##any" processContents="lax"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:undeclared="value"/>`,
			expectValid: true,
			description: "lax mode should allow undeclared attributes",
		},
		{
			name: "lax rejects invalid value for declared attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attribute name="testAttr" type="xs:integer"/>
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="lax"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      tns:testAttr="not-a-number"/>`,
			expectValid: false,
			description: "lax mode should reject invalid value for declared attribute",
		},
		{
			name: "skip allows any value",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:anyAttr="any-value"/>`,
			expectValid: true,
			description: "skip mode should allow any attribute value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
				t.Fatalf("Validate schema: %v", validationErrors)
			}

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			valid := len(violations) == 0
			if valid != tt.expectValid {
				if tt.expectValid {
					t.Errorf("%s\nExpected valid, got invalid:\n", tt.description)
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				} else {
					t.Errorf("%s\nExpected invalid, got valid", tt.description)
				}
			}
		})
	}
}

func TestAnyAttributeDerivationTargetNamespaceMismatch(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:a="a"
           xmlns:b="b"
           targetNamespace="a"
           elementFormDefault="qualified">
  <xs:import namespace="b" schemaLocation="b.xsd"/>

  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="b:Base">
        <xs:anyAttribute namespace="##other" processContents="skip"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	otherSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="b"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
</xs:schema>`

	schemaLoader := loader.NewLoader(loader.Config{
		FS: fstest.MapFS{
			"main.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
			"b.xsd":    &fstest.MapFile{Data: []byte(otherSchema)},
		},
	})

	_, err := schemaLoader.Load("main.xsd")
	if err == nil {
		t.Fatalf("expected derivation error for anyAttribute target namespace mismatch")
	}
	if !strings.Contains(err.Error(), "anyAttribute restriction") {
		t.Fatalf("expected anyAttribute restriction error, got: %v", err)
	}
}

// TestAnyAttributeDerivationExtension tests anyAttribute in extension (union)
func TestAnyAttributeDerivationExtension(t *testing.T) {
	tests := []struct {
		name        string
		schemaXML   string
		xmlDoc      string
		expectValid bool
		description string
	}{
		{
			name: "extension unions anyAttribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:anyAttribute namespace="##other" processContents="skip"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      xmlns:other="http://example.com/other"
      tns:targetAttr="value1"
      other:otherAttr="value2"/>`,
			expectValid: true,
			description: "Extension should union anyAttribute (allows both target and other namespaces)",
		},
		{
			name: "extension with base ##any",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`,
			xmlDoc: `<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:anyAttr="value"/>`,
			expectValid: true,
			description: "Extension with base ##any should result in ##any (union)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
				t.Fatalf("Validate schema: %v", validationErrors)
			}

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			valid := len(violations) == 0
			if valid != tt.expectValid {
				if tt.expectValid {
					t.Errorf("%s\nExpected valid, got invalid:\n", tt.description)
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				} else {
					t.Errorf("%s\nExpected invalid, got valid", tt.description)
				}
			}
		})
	}
}

// TestAnyAttributeDerivationRestriction tests anyAttribute in restriction (subset)
func TestAnyAttributeDerivationRestriction(t *testing.T) {
	tests := []struct {
		name          string
		schemaXML     string
		expectValid   bool
		errorContains string
		description   string
	}{
		{
			name: "restriction allows subset",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##any"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:anyAttribute namespace="##targetNamespace"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`,
			expectValid: true,
			description: "Restriction should allow subset of base anyAttribute",
		},
		{
			name: "restriction rejects adding anyAttribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:anyAttribute namespace="##any"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`,
			expectValid:   false,
			errorContains: "cannot add anyAttribute when base type has no anyAttribute",
			description:   "Restriction should reject adding anyAttribute when base has none",
		},
		{
			name: "restriction rejects non-subset",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##targetNamespace"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:anyAttribute namespace="##other"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`,
			expectValid:   false,
			errorContains: "not a subset",
			description:   "Restriction should reject anyAttribute that is not a subset of base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			validationErrors := loader.ValidateSchema(schema)
			valid := len(validationErrors) == 0

			if valid != tt.expectValid {
				if tt.expectValid {
					t.Errorf("%s\nExpected valid schema, got invalid:\n", tt.description)
					for _, err := range validationErrors {
						t.Errorf("  %v", err)
					}
				} else {
					t.Errorf("%s\nExpected invalid schema, got valid", tt.description)
					if tt.errorContains != "" {
						found := false
						for _, err := range validationErrors {
							if strings.Contains(err.Error(), tt.errorContains) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, validationErrors)
						}
					}
				}
			}
		})
	}
}

// TestAnyAttributeFixedValue tests fixed value validation for anyAttribute
func TestAnyAttributeFixedValue(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attribute name="fixedAttr" type="xs:string" fixed="fixed-value"/>
  <xs:complexType name="TestType">
    <xs:anyAttribute namespace="##targetNamespace" processContents="strict"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	tests := []struct {
		name        string
		xmlDoc      string
		expectValid bool
		description string
	}{
		{
			name:        "fixed value matches",
			xmlDoc:      `<root xmlns="http://example.com/test" xmlns:tns="http://example.com/test" tns:fixedAttr="fixed-value"/>`,
			expectValid: true,
			description: "Fixed value should match exactly",
		},
		{
			name:        "fixed value mismatch",
			xmlDoc:      `<root xmlns="http://example.com/test" xmlns:tns="http://example.com/test" tns:fixedAttr="wrong-value"/>`,
			expectValid: false,
			description: "Fixed value mismatch should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			valid := len(violations) == 0
			if valid != tt.expectValid {
				if tt.expectValid {
					t.Errorf("%s\nExpected valid, got invalid:\n", tt.description)
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				} else {
					t.Errorf("%s\nExpected invalid, got valid", tt.description)
				}
			}
		})
	}
}

// TestAnyAttributeWithExplicitAttributes tests interaction with explicit attributes
func TestAnyAttributeWithExplicitAttributes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:attribute name="explicit" type="xs:string"/>
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	// XML with both explicit and wildcard attributes
	xmlDoc := `<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      xmlns:other="http://example.com/other"
      tns:explicit="value1"
      other:wildcard="value2"/>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	// should be valid - explicit attribute and wildcard attribute both allowed
	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

// TestProhibitedAttributeWithAnyAttribute tests that prohibited attributes don't block wildcard matching
// Per XSD spec section 3.4.2, prohibited attributes are NOT in {attribute uses}
// They only prevent inheritance during type derivation
func TestProhibitedAttributeWithAnyAttribute(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           attributeFormDefault="unqualified">
  <xs:element name='root'>
    <xs:complexType>
      <xs:attribute name="attr" use="prohibited"/>
      <xs:anyAttribute namespace="##local" processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	// XML with the "prohibited" attribute
	// this should be VALID because:
	// 1. prohibited attributes are not in {attribute uses}
	// 2. anyAttribute namespace="##local" allows local (no-namespace) attributes
	// 3. attr="123" is a local attribute, so it matches the wildcard
	xmlDoc := `<root attr="123"/>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	// should be valid - anyAttribute allows this attribute
	if len(violations) > 0 {
		t.Errorf("Expected valid (prohibited only matters for derivation), got invalid:")
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}
