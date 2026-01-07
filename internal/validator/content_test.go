package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xml"
)

const (
	errUnexpectedElement      = string(errors.ErrUnexpectedElement)
	errTextInElementOnly      = string(errors.ErrTextInElementOnly)
	errRequiredElementMissing = string(errors.ErrRequiredElementMissing)
)

// TestEmptyContentValidation tests empty content validation per XSD spec
// Spec: "If CT is empty content (no allowed children, not mixed), then the element
// must have no element children and no character data other than whitespace."
func TestEmptyContentValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "empty content with no children and no text - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="xs:anyType">
          <xs:sequence/>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"></root>`,
			shouldErr: false,
		},
		{
			name: "empty content with whitespace only - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="xs:anyType">
          <xs:sequence/>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">   </root>`,
			shouldErr: false,
		},
		{
			name: "empty content with child element - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="xs:anyType">
          <xs:sequence/>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <child>value</child>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "empty sequence with non-whitespace text - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">text</root>`,
			shouldErr: true,
			errCode:   errTextInElementOnly, // element-only content cannot have character children
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestElementOnlyContentTextValidation tests that non-whitespace text in element-only
// content is flagged as invalid per XSD spec section 5.2
// Spec: "If CT is element-only (not mixed) and the type is not a simple content,
// any non-whitespace text in the element should be flagged as invalid."
func TestElementOnlyContentTextValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "element-only with valid children - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <child>value</child>
</root>`,
			shouldErr: false,
		},
		{
			name: "element-only with whitespace between elements - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child1" type="xs:string"/>
        <xs:element name="child2" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <child1>value1</child1>
  
  <child2>value2</child2>
</root>`,
			shouldErr: false,
		},
		{
			name: "element-only with non-whitespace text and no children - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">invalid text</root>`,
			shouldErr: true,
			errCode:   errTextInElementOnly, // element-only content cannot have character children
		},
		{
			name: "mixed content allows text - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType mixed="true">
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  valid text
  <child>value</child>
  more text
</root>`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestEndOfContentValidation tests end-of-content validation
// Spec: "After consuming all child elements, any remaining part of the content model
// must be satisfiable by empty"
func TestEndOfContentValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "sequence with all required elements - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
</root>`,
			shouldErr: false,
		},
		{
			name: "sequence with unexpected element after end - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
  <c>unexpected</c>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "sequence with optional element missing - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
</root>`,
			shouldErr: false,
		},
		{
			name: "sequence with required element missing - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestMinOccursMaxOccursValidation tests occurrence constraints
func TestMinOccursMaxOccursValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "element with minOccurs=0 maxOccurs=1, absent - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" minOccurs="0" maxOccurs="1"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"></root>`,
			shouldErr: false,
		},
		{
			name: "element with minOccurs=2, only one present - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" minOccurs="2" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
		{
			name: "element with maxOccurs=2, three present - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <a>value2</a>
  <a>value3</a>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "element with maxOccurs=unbounded, multiple present - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <a>value2</a>
  <a>value3</a>
</root>`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestChoiceGroupValidation tests choice group validation
func TestChoiceGroupValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "choice with one valid option - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value</a>
</root>`,
			shouldErr: false,
		},
		{
			name: "choice with unexpected element - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <c>value</c>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "choice with minOccurs=0, absent - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice minOccurs="0">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"></root>`,
			shouldErr: false,
		},
		{
			name: "choice with minOccurs=1, absent - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice minOccurs="1">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"></root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestAllGroupValidation tests all group validation
func TestAllGroupValidation(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "all group with all elements in any order - valid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <b>value2</b>
  <a>value1</a>
</root>`,
			shouldErr: false,
		},
		{
			name: "all group with missing required element - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
		{
			name: "all group with duplicate element - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
  <a>duplicate</a>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "all group with unexpected element - invalid",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
  <c>unexpected</c>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement, // element not in content model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestContentModelRequiredElements tests that missing required elements are detected
// This tests issues like ipo3, ipo5, ipo6 where required elements are missing
func TestContentModelRequiredElements(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "sequence with required element missing at end",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
		{
			name: "sequence with required element missing in middle",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <c>value3</c>
</root>`,
			shouldErr: true,
			// when encountering 'c' while expecting 'b', the sequence fails
			errCode: errRequiredElementMissing,
		},
		{
			name: "sequence with minOccurs=2, only one present",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" minOccurs="2" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestContentModelUnexpectedElements tests that unexpected elements are caught
// This tests issues like stZ050 where unexpected elements are not being rejected
func TestContentModelUnexpectedElements(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "sequence with unexpected element at end",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <b>value2</b>
  <c>unexpected</c>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
		{
			name: "sequence with unexpected element in middle",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <c>unexpected</c>
  <b>value2</b>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement, // element 'c' is not allowed (not in content model)
		},
		{
			name: "empty sequence with unexpected element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>unexpected</a>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestContentModelOutOfOrder tests that out-of-order elements in sequences are rejected
func TestContentModelOutOfOrder(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "sequence with elements out of order",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <c>value3</c>
  <b>value2</b>
</root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing, // sequence fails because expected element not in order
		},
		{
			name: "sequence with optional element skipped, then later element appears",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string" minOccurs="0"/>
        <xs:element name="c" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <a>value1</a>
  <c>value3</c>
  <b>value2</b>
</root>`,
			shouldErr: true,
			errCode:   errUnexpectedElement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

// TestContentModelChoiceRequired tests that choice groups properly handle minOccurs constraints
func TestContentModelChoiceRequired(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
		errCode   string
	}{
		{
			name: "choice with minOccurs=1, absent",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice minOccurs="1">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"></root>`,
			shouldErr: true,
			errCode:   errRequiredElementMissing,
		},
		// TODO: Choice with minOccurs > 1 requires more sophisticated occurrence counting
		// the current automaton counts per-symbol, not per-group occurrence
		// {
		// 	name: "choice with minOccurs=2, only one present",
		// 	schemaXML: `...`,
		// 	xmlDoc: `...`,
		// 	shouldErr: true,
		// 	errCode:   errRequiredElementMissing,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := v.Validate(doc)

			if tt.shouldErr {
				if len(violations) == 0 {
					t.Errorf("Expected violation with code %s, got none", tt.errCode)
				} else {
					found := false
					for _, viol := range violations {
						if viol.Code == tt.errCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation code %s, got: %v", tt.errCode, violations)
					}
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

func TestEmptyChoiceRejectsAll(t *testing.T) {
	tests := []struct {
		name    string
		xmlDoc  string
		errCode string
	}{
		{
			name:    "empty content rejects empty element",
			xmlDoc:  `<?xml version="1.0"?><root xmlns="http://example.com/test"></root>`,
			errCode: errRequiredElementMissing,
		},
		{
			name:    "empty content rejects child elements",
			xmlDoc:  `<?xml version="1.0"?><root xmlns="http://example.com/test"><child/></root>`,
			errCode: errUnexpectedElement,
		},
	}

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			violations := v.Validate(doc)
			if len(violations) == 0 {
				t.Fatalf("Expected violation code %s, got none", tt.errCode)
			}
			found := false
			for _, viol := range violations {
				if viol.Code == tt.errCode {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Expected violation code %s, got: %v", tt.errCode, violations)
			}
		})
	}
}

func TestGroupFixedOccurrenceCounts(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:sequence minOccurs="1" maxOccurs="2">
          <xs:element name="b" minOccurs="2" maxOccurs="2"/>
        </xs:sequence>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	tests := []struct {
		name      string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name:      "exactly two elements",
			xmlDoc:    `<?xml version="1.0"?><root xmlns="http://example.com/test"><b/><b/></root>`,
			shouldErr: false,
		},
		{
			name:      "exactly four elements",
			xmlDoc:    `<?xml version="1.0"?><root xmlns="http://example.com/test"><b/><b/><b/><b/></root>`,
			shouldErr: false,
		},
		{
			name:      "three elements is invalid",
			xmlDoc:    `<?xml version="1.0"?><root xmlns="http://example.com/test"><b/><b/><b/></root>`,
			shouldErr: true,
		},
	}

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := xml.Parse(strings.NewReader(tt.xmlDoc))
			if err != nil {
				t.Fatalf("Parse XML: %v", err)
			}

			violations := v.Validate(doc)
			if tt.shouldErr {
				if len(violations) == 0 {
					t.Fatalf("Expected validation error, got none")
				}
				return
			}
			if len(violations) > 0 {
				t.Fatalf("Expected no violations, got: %v", violations)
			}
		})
	}
}