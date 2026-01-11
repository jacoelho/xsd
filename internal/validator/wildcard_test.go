package validator

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
)

// TestWildcardNamespaceMatching tests namespace constraint matching
func TestWildcardNamespaceMatching(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "##any matches any namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "##any matches empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo>value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "##targetNamespace matches only target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://example.com/test">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "##targetNamespace rejects other namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: true,
		},
		{
			name: "##other matches non-target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##other" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "##other rejects target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##other" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://example.com/test">value</foo>
</root>`,
			shouldErr: true,
		},
		{
			name: "##other rejects empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##other" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo>value</foo>
</root>`,
			shouldErr: true,
		},
		{
			name: "##local matches empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="unqualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##local" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "##local rejects non-empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##local" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://example.com/test">value</foo>
</root>`,
			shouldErr: true,
		},
		{
			name: "explicit namespace list matches listed namespaces",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="http://ns1.com http://ns2.com" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://ns1.com">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "explicit namespace list rejects non-listed namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="http://ns1.com http://ns2.com" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			hasError := len(violations) > 0
			if hasError != tt.shouldErr {
				if tt.shouldErr {
					t.Errorf("Expected validation error, got none")
				} else {
					t.Errorf("Expected no validation error, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}

func TestWildcardDerivationTargetNamespaceMismatch(t *testing.T) {
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
        <xs:sequence>
          <xs:any namespace="##other" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	otherSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="b"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:any namespace="##other" processContents="skip"/>
    </xs:sequence>
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
		t.Fatalf("expected derivation error for wildcard target namespace mismatch")
	}
	if !strings.Contains(err.Error(), "wildcard restriction") {
		t.Fatalf("expected wildcard restriction error, got: %v", err)
	}
}

// TestWildcardProcessContents tests processContents handling
func TestWildcardProcessContents(t *testing.T) {
	tests := []struct {
		name        string
		schemaXML   string
		xmlDoc      string
		errorCode   string
		description string
		shouldErr   bool
	}{
		{
			name: "strict requires declaration",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="strict"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: true,
			errorCode: string(errors.ErrWildcardNotDeclared),
		},
		{
			name: "strict validates declared element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="strict"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="foo" type="xs:string"/>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://example.com/test">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "lax validates if declaration found",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="foo" type="xs:string"/>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://example.com/test">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "lax accepts undeclared element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`,
			shouldErr: false,
		},
		{
			name: "skip accepts any element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
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

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			hasError := len(violations) > 0
			if hasError != tt.shouldErr {
				if tt.shouldErr {
					t.Errorf("Expected validation error with code %s, got none", tt.errorCode)
				} else {
					t.Errorf("Expected no validation error, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}

			if tt.shouldErr && len(violations) > 0 && tt.errorCode != "" {
				found := false
				for _, v := range violations {
					if v.Code == tt.errorCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error code %s, got: %v", tt.errorCode, violations)
				}
			}
		})
	}
}

// TestWildcardMinMaxOccurs tests wildcard occurrence constraints
func TestWildcardMinMaxOccurs(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "minOccurs 0 allows no elements",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"/>
`,
			shouldErr: false,
		},
		{
			name: "minOccurs 1 requires at least one element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" minOccurs="1"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test"/>
`,
			shouldErr: true,
		},
		{
			name: "maxOccurs unbounded allows multiple elements",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value1</foo>
  <bar xmlns="http://other.com/ns">value2</bar>
</root>`,
			shouldErr: false,
		},
		{
			name: "maxOccurs 2 allows up to 2 elements",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value1</foo>
  <bar xmlns="http://other.com/ns">value2</bar>
</root>`,
			shouldErr: false,
		},
		{
			name: "maxOccurs 2 rejects more than 2 elements",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value1</foo>
  <bar xmlns="http://other.com/ns">value2</bar>
  <baz xmlns="http://other.com/ns">value3</baz>
</root>`,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schemaXML))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations := validateStream(t, v, tt.xmlDoc)

			hasError := len(violations) > 0
			if hasError != tt.shouldErr {
				if tt.shouldErr {
					t.Errorf("Expected validation error, got none")
				} else {
					t.Errorf("Expected no validation error, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  %s", v.Error())
					}
				}
			}
		})
	}
}
