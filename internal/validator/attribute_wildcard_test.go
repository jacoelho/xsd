package validator

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
)

// TestAttributeWildcard_NamespaceAny tests that ##any matches any namespace
func TestAttributeWildcard_NamespaceAny(t *testing.T) {
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
      <xs:anyAttribute namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
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
      <xs:anyAttribute namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" attr="value"/>`,
			shouldErr: false,
		},
		{
			name: "##any matches target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
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

// TestAttributeWildcard_NamespaceTargetNamespace tests ##targetNamespace matching
func TestAttributeWildcard_NamespaceTargetNamespace(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "##targetNamespace matches target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
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
      <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
			shouldErr: true,
		},
		{
			name: "##targetNamespace rejects empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" attr="value"/>`,
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

func TestAttributeWildcardTracksIDREF(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="ref" type="xs:IDREF"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="strict"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns="urn:test" xmlns:tns="urn:test" tns:ref="missing"/>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrIDRefNotFound) {
		t.Fatalf("expected IDREF violation, got %v", violations)
	}
}

func TestAttributeWildcardTracksID(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="id" type="xs:ID"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:anyAttribute namespace="##any" processContents="strict"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns="urn:test" xmlns:tns="urn:test">
  <item tns:id="dup"/>
  <item tns:id="dup"/>
</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if !hasViolationCode(violations, errors.ErrDuplicateID) {
		t.Fatalf("expected duplicate ID violation, got %v", violations)
	}
}

// TestAttributeWildcard_NamespaceOther tests ##other matching
func TestAttributeWildcard_NamespaceOther(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "##other matches non-target namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##other" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
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
      <xs:anyAttribute namespace="##other" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
			shouldErr: true,
		},
		{
			name: "##other rejects empty namespace when target namespace is present",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##other" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" attr="value"/>`,
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

// TestAttributeWildcard_NamespaceLocal tests ##local matching
func TestAttributeWildcard_NamespaceLocal(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "##local matches empty namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="unqualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##local" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" attr="value"/>`,
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
      <xs:anyAttribute namespace="##local" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
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

// TestAttributeWildcard_NamespaceList tests explicit namespace list matching
func TestAttributeWildcard_NamespaceList(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "namespace list matches listed namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="http://ns1.com http://ns2.com" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:ns1="http://ns1.com"
      ns1:attr="value"/>`,
			shouldErr: false,
		},
		{
			name: "namespace list rejects non-listed namespace",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="http://ns1.com http://ns2.com" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
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

// TestAttributeWildcard_ProcessContentsStrict tests strict processing mode
func TestAttributeWildcard_ProcessContentsStrict(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		errorCode string
		shouldErr bool
	}{
		{
			name: "strict requires declaration",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="strict"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
			shouldErr: true,
			errorCode: string(errors.ErrWildcardNotDeclared),
		},
		{
			name: "strict validates declared attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="strict"/>
    </xs:complexType>
  </xs:element>
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
			shouldErr: false,
		},
		{
			name: "strict rejects attribute in wrong namespace",
			// strict mode should reject attributes used in a namespace with no declaration.
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="strict"/>
    </xs:complexType>
  </xs:element>
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
			shouldErr: true, // attribute declared in target namespace, but used in other namespace
			errorCode: string(errors.ErrWildcardNotDeclared),
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

func TestAttributeWildcard_StrictMultiSchemaImport(t *testing.T) {
	mainSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:import namespace="urn:ext" schemaLocation="ext.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="strict"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	extSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ext">
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"main.xsd": {Data: []byte(mainSchema)},
		"ext.xsd":  {Data: []byte(extSchema)},
	}

	schemaLoader := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := schemaLoader.LoadCompiled("main.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled)
	doc := `<?xml version="1.0"?>
<root xmlns="urn:main" xmlns:ext="urn:ext" ext:attr="value"/>`
	if violations, err := v.ValidateStream(strings.NewReader(doc)); err != nil || len(violations) != 0 {
		t.Fatalf("expected valid document, got violations=%v err=%v", violations, err)
	}
}

// TestAttributeWildcard_ProcessContentsLax tests lax processing mode
func TestAttributeWildcard_ProcessContentsLax(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "lax validates if declaration found",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="lax"/>
    </xs:complexType>
  </xs:element>
  <xs:attribute name="attr" type="xs:string"/>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:tns="http://example.com/test"
      tns:attr="value"/>`,
			shouldErr: false,
		},
		{
			name: "lax accepts undeclared attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
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

// TestAttributeWildcard_ProcessContentsSkip tests skip processing mode
func TestAttributeWildcard_ProcessContentsSkip(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		xmlDoc    string
		shouldErr bool
	}{
		{
			name: "skip accepts any attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			xmlDoc: `<?xml version="1.0"?>
<root xmlns="http://example.com/test" 
      xmlns:other="http://other.com/ns"
      other:attr="value"/>`,
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
