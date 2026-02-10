package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestParseSimpleSchema(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           xmlns:tns="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="message" type="xs:string"/>
  <xs:element name="person">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="age" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if schema.TargetNamespace != model.NamespaceURI("http://example.com/simple") {
		t.Errorf("TargetNamespace = %q, want %q", schema.TargetNamespace, "http://example.com/simple")
	}

	messageQName := model.QName{
		Namespace: model.NamespaceURI("http://example.com/simple"),
		Local:     "message",
	}
	if _, ok := schema.ElementDecls[messageQName]; !ok {
		t.Errorf("element 'message' not found in schema")
	}

	personQName := model.QName{
		Namespace: model.NamespaceURI("http://example.com/simple"),
		Local:     "person",
	}
	if _, ok := schema.ElementDecls[personQName]; !ok {
		t.Errorf("element 'person' not found in schema")
	}
}

func TestParseBooleanAttributesWithWhitespace(t *testing.T) {
	schemaXML := "<?xml version=\"1.0\"?>\n" +
		"<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n" +
		"  <xs:complexType name=\"T\" mixed=\" true \">\n" +
		"    <xs:sequence>\n" +
		"      <xs:element name=\"child\" type=\"xs:string\"/>\n" +
		"    </xs:sequence>\n" +
		"  </xs:complexType>\n" +
		"  <xs:element name=\"root\" type=\"xs:string\" nillable=\"\t0\n\"/>\n" +
		"</xs:schema>"

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	typ, ok := schema.TypeDefs[model.QName{Local: "T"}]
	if !ok {
		t.Fatalf("complexType 'T' not found in schema")
	}
	ct, ok := typ.(*model.ComplexType)
	if !ok {
		t.Fatalf("complexType 'T' = %T, want *model.ComplexType", typ)
	}
	if !ct.Mixed() {
		t.Fatalf("complexType 'T' mixed = false, want true")
	}

	elem, ok := schema.ElementDecls[model.QName{Local: "root"}]
	if !ok {
		t.Fatalf("element 'root' not found in schema")
	}
	if elem.Nillable {
		t.Fatalf("element 'root' nillable = true, want false")
	}
}

func TestParseEnumAttributesWithWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:enum"
           xmlns:tns="urn:enum"
           elementFormDefault=" qualified ">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use=" required "/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if schema.ElementFormDefault != Qualified {
		t.Fatalf("ElementFormDefault = %v, want Qualified", schema.ElementFormDefault)
	}

	root := schema.ElementDecls[model.QName{Namespace: "urn:enum", Local: "root"}]
	if root == nil {
		t.Fatalf("element 'root' not found in schema")
	}
	ct, ok := root.Type.(*model.ComplexType)
	if !ok {
		t.Fatalf("root type = %T, want *model.ComplexType", root.Type)
	}
	attrs := ct.Attributes()
	if len(attrs) != 1 || attrs[0].Use != model.Required {
		t.Fatalf("attribute use = %v, want Required", attrs)
	}
}

func TestParseTargetNamespaceWhitespaceError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="   ">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
		t.Fatalf("expected targetNamespace whitespace error")
	}
}

func TestParseSchemaNamespacePrefixEmpty(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:bad="">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	reader := strings.NewReader(schemaXML)
	_, err := Parse(reader)
	if err == nil {
		t.Fatal("Parse() expected error for empty namespace prefix binding, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be bound to empty namespace") {
		t.Fatalf("Parse() error = %v, want error containing %q", err, "cannot be bound to empty namespace")
	}
}

func TestParseAllMinOccursConstraint(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "xs:all with minOccurs=0 should succeed",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all minOccurs="0">
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: false,
		},
		{
			name: "xs:all with minOccurs=2 should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all minOccurs="2">
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all must have minOccurs='0' or '1'",
		},
		{
			name: "xs:all with maxOccurs=2 should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all maxOccurs="2">
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all must have maxOccurs='1'",
		},
		{
			name: "xs:all with maxOccurs=unbounded should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all maxOccurs="unbounded">
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all must have maxOccurs='1'",
		},
		{
			name: "xs:all with minOccurs=1 maxOccurs=1 should succeed",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all minOccurs="1" maxOccurs="1">
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: false,
		},
		{
			name: "xs:all without attributes should succeed (defaults to 1)",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="TestType">
    <xs:all>
      <xs:element name="child" type="xs:string"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.schema)
			_, err := Parse(r)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Parse() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestElementWithoutTypeDefaultsToAnyType(t *testing.T) {
	tests := []struct {
		name         string
		schema       string
		elementName  string
		wantTypeName string
		wantTypeNS   model.NamespaceURI
		// "BuiltinType", "ComplexType", or "SimpleType"
		wantTypeKind string
	}{
		{
			name: "top-level element without type defaults to anyType",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "anyType",
			wantTypeNS:   model.XSDNamespace,
			wantTypeKind: "BuiltinType",
		},
		{
			name: "local element without type defaults to anyType",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:complexType name="TestType">
    <xs:sequence>
      <xs:element name="child"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`,
			elementName:  "child",
			wantTypeName: "anyType",
			wantTypeNS:   model.XSDNamespace,
			wantTypeKind: "BuiltinType",
		},
		{
			name: "element with explicit xs:anyType type",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement" type="xs:anyType"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "anyType",
			wantTypeNS:   model.XSDNamespace,
			wantTypeKind: "BuiltinType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.schema)
			schema, err := Parse(r)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// for top-level elements, check ElementDecls
			if tt.name == "top-level element without type defaults to anyType" || tt.name == "element with explicit xs:anyType type" {
				qname := model.QName{
					Namespace: model.NamespaceURI("http://example.com/test"),
					Local:     tt.elementName,
				}
				decl, ok := schema.ElementDecls[qname]
				if !ok {
					t.Fatalf("element %s not found in schema", tt.elementName)
				}

				if decl.Type == nil {
					t.Fatalf("element %s should have type anyType, but Type is nil", tt.elementName)
				}

				typeQName := decl.Type.Name()
				if typeQName.Local != tt.wantTypeName {
					t.Errorf("element type Local = %q, want %q", typeQName.Local, tt.wantTypeName)
				}
				if typeQName.Namespace != tt.wantTypeNS {
					t.Errorf("element type Namespace = %q, want %q", typeQName.Namespace, tt.wantTypeNS)
				}

				// verify it's the expected type kind and has the correct name
				switch tt.wantTypeKind {
				case "BuiltinType":
					bt, ok := decl.Type.(*model.BuiltinType)
					if !ok {
						t.Errorf("element type is %T, want *model.BuiltinType", decl.Type)
					} else if bt.Name().Local != tt.wantTypeName {
						t.Errorf("element type Local = %q, want %q", bt.Name().Local, tt.wantTypeName)
					}
				case "ComplexType":
					ct, ok := decl.Type.(*model.ComplexType)
					if !ok {
						t.Errorf("element type is %T, want *model.ComplexType", decl.Type)
					} else if ct.Name().Local != tt.wantTypeName {
						t.Errorf("element type Local = %q, want %q", ct.Name().Local, tt.wantTypeName)
					}
				case "SimpleType":
					st, ok := decl.Type.(*model.SimpleType)
					if !ok {
						t.Errorf("element type is %T, want *model.SimpleType", decl.Type)
					} else if st.Name().Local != tt.wantTypeName {
						t.Errorf("element type Local = %q, want %q", st.Name().Local, tt.wantTypeName)
					}
				}
			}
		})
	}
}

func TestUnqualifiedTypeReferences(t *testing.T) {
	tests := []struct {
		name         string
		schema       string
		elementName  string
		wantTypeName string
		wantTypeNS   model.NamespaceURI
	}{
		{
			name: "unqualified type without default namespace resolves to no namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:import schemaLocation="no-ns.xsd"/>
  <xs:element name="testElement" type="string"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "string",
			wantTypeNS:   model.NamespaceEmpty,
		},
		{
			name: "unqualified string type resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement" type="string"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "string",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified integer type resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement" type="integer"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "integer",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified positiveInteger type resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement" type="positiveInteger"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "positiveInteger",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified boolean type resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:element name="testElement" type="boolean"/>
</xs:schema>`,
			elementName:  "testElement",
			wantTypeName: "boolean",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified type in attribute resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:complexType name="TestType">
    <xs:attribute name="testAttr" type="string"/>
  </xs:complexType>
</xs:schema>`,
			elementName:  "TestType",
			wantTypeName: "string",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified type in simpleType restriction resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:simpleType name="MyString">
    <xs:restriction base="string"/>
  </xs:simpleType>
</xs:schema>`,
			elementName:  "MyString",
			wantTypeName: "string",
			wantTypeNS:   model.XSDNamespace,
		},
		{
			name: "unqualified type in complexType extension resolves to XSD namespace",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com/test">
  <xs:complexType name="TestType">
    <xs:simpleContent>
      <xs:extension base="string">
        <xs:attribute name="id" type="string"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			elementName:  "TestType",
			wantTypeName: "string",
			wantTypeNS:   model.XSDNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.schema)
			schema, err := Parse(r)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			switch {
			case strings.Contains(tt.name, "element"):
				qname := model.QName{
					Namespace: model.NamespaceURI("http://example.com/test"),
					Local:     tt.elementName,
				}
				decl, ok := schema.ElementDecls[qname]
				if !ok {
					t.Fatalf("element %s not found in schema", tt.elementName)
				}

				if decl.Type == nil {
					t.Fatalf("element %s should have type, but Type is nil", tt.elementName)
				}

				typeQName := decl.Type.Name()
				if typeQName.Local != tt.wantTypeName {
					t.Errorf("element type Local = %q, want %q", typeQName.Local, tt.wantTypeName)
				}
				if typeQName.Namespace != tt.wantTypeNS {
					t.Errorf("element type Namespace = %q, want %q", typeQName.Namespace, tt.wantTypeNS)
				}
			case strings.Contains(tt.name, "attribute"):
				qname := model.QName{
					Namespace: model.NamespaceURI("http://example.com/test"),
					Local:     tt.elementName,
				}
				ct, ok := schema.TypeDefs[qname].(*model.ComplexType)
				if !ok {
					t.Fatalf("complexType %s not found in schema", tt.elementName)
				}

				if len(ct.Attributes()) == 0 {
					t.Fatalf("complexType %s should have attributes", tt.elementName)
				}

				attrType := ct.Attributes()[0].Type
				if attrType == nil {
					t.Fatalf("attribute should have type, but Type is nil")
				}

				typeQName := attrType.Name()
				if typeQName.Local != tt.wantTypeName {
					t.Errorf("attribute type Local = %q, want %q", typeQName.Local, tt.wantTypeName)
				}
				if typeQName.Namespace != tt.wantTypeNS {
					t.Errorf("attribute type Namespace = %q, want %q", typeQName.Namespace, tt.wantTypeNS)
				}
			case strings.Contains(tt.name, "simpleType"):
				qname := model.QName{
					Namespace: model.NamespaceURI("http://example.com/test"),
					Local:     tt.elementName,
				}
				st, ok := schema.TypeDefs[qname].(*model.SimpleType)
				if !ok {
					t.Fatalf("simpleType %s not found in schema", tt.elementName)
				}

				if st.Restriction == nil {
					t.Fatalf("simpleType %s should have restriction", tt.elementName)
				}

				baseQName := st.Restriction.Base
				if baseQName.Local != tt.wantTypeName {
					t.Errorf("simpleType base Local = %q, want %q", baseQName.Local, tt.wantTypeName)
				}
				if baseQName.Namespace != tt.wantTypeNS {
					t.Errorf("simpleType base Namespace = %q, want %q", baseQName.Namespace, tt.wantTypeNS)
				}
			case strings.Contains(tt.name, "extension"):
				qname := model.QName{
					Namespace: model.NamespaceURI("http://example.com/test"),
					Local:     tt.elementName,
				}
				ct, ok := schema.TypeDefs[qname].(*model.ComplexType)
				if !ok {
					t.Fatalf("complexType %s not found in schema", tt.elementName)
				}

				sc, ok := ct.Content().(*model.SimpleContent)
				if !ok {
					t.Fatalf("complexType %s should have SimpleContent", tt.elementName)
				}

				if sc.Extension == nil {
					t.Fatalf("SimpleContent should have extension")
				}

				baseQName := sc.Extension.Base
				if baseQName.Local != tt.wantTypeName {
					t.Errorf("extension base Local = %q, want %q", baseQName.Local, tt.wantTypeName)
				}
				if baseQName.Namespace != tt.wantTypeNS {
					t.Errorf("extension base Namespace = %q, want %q", baseQName.Namespace, tt.wantTypeNS)
				}
			}
		})
	}
}

func TestComplexContentRestrictionWithAttributesOnly(t *testing.T) {
	// test that a restriction can have attributes without a particle
	// this is valid XSD 1.0 - attributes can exist without particles
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="elem1" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="att1" type="xs:string"/>
    <xs:attribute name="att2" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="BaseType">
        <xs:attribute name="att1" use="prohibited"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	r := strings.NewReader(schema)
	parsed, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	derivedQName := model.QName{
		Namespace: model.NamespaceEmpty,
		Local:     "DerivedType",
	}
	ct, ok := parsed.TypeDefs[derivedQName].(*model.ComplexType)
	if !ok {
		t.Fatalf("DerivedType not found or not a ComplexType")
	}

	cc, ok := ct.Content().(*model.ComplexContent)
	if !ok {
		t.Fatalf("DerivedType content is not ComplexContent, got %T", ct.Content())
	}

	if cc.Restriction == nil {
		t.Fatalf("ComplexContent should have a restriction")
	}

	if len(cc.Restriction.Attributes) == 0 {
		t.Fatalf("Restriction should have attributes")
	}

	if cc.Restriction.Attributes[0].Name.Local != "att1" {
		t.Errorf("First attribute name = %q, want %q", cc.Restriction.Attributes[0].Name.Local, "att1")
	}

	if cc.Restriction.Attributes[0].Use != model.Prohibited {
		t.Errorf("Attribute use = %v, want %v", cc.Restriction.Attributes[0].Use, model.Prohibited)
	}
}

func TestComplexContentRestrictionOrderValidation(t *testing.T) {
	// test that attributes must come after particles if both are present
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "attributes before particle should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="elem1" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="BaseType">
        <xs:attribute name="att1" type="xs:string"/>
        <xs:sequence>
          <xs:element name="elem1" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`,
			wantErr: true,
			errMsg:  "attributes must come after the content model particle",
		},
		{
			name: "particle before attributes should succeed",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="elem1" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="BaseType">
        <xs:sequence>
          <xs:element name="elem1" type="xs:string"/>
        </xs:sequence>
        <xs:attribute name="att1" type="xs:string"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.schema)
			_, err := Parse(r)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Parse() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseInvalidNamespaceConstraints(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "##any ##other combination is invalid",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##any ##other"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: true,
			errMsg:  "invalid namespace constraint",
		},
		{
			name: "##other ##targetNamespace combination is invalid",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##other ##targetNamespace"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: true,
			errMsg:  "invalid namespace constraint",
		},
		{
			name: "##any with namespace URI is invalid",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##any http://example.com"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: true,
			errMsg:  "invalid namespace constraint",
		},
		{
			name: "##local ##targetNamespace combination is valid (both can appear in lists)",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##local ##targetNamespace"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: false,
		},
		{
			name: "##targetNamespace with namespace URI is valid",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##targetNamespace http://example.com"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: false,
		},
		{
			name: "##local with namespace URI is valid",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##local http://example.com"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: false,
		},
		{
			name: "valid ##any is accepted",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="##any"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: false,
		},
		{
			name: "valid namespace list is accepted",
			schema: `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="http://foobar">
	<xsd:element name="foo">
		<xsd:complexType>
			<xsd:sequence>
				<xsd:any namespace="http://example.com http://other.com"/>
			</xsd:sequence>
		</xsd:complexType>
	</xsd:element>
</xsd:schema>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.schema))
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Parse() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseXMLErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		wantCode string
		wantMsg  string
	}{
		{
			name:     "malformed XML - unclosed element",
			schema:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="test"></xs:schema>`,
			wantCode: "schema-parse-error",
			wantMsg:  "parse XML",
		},
		{
			name:     "malformed XML - invalid tag",
			schema:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="test" closed by </xs:restriction>`,
			wantCode: "schema-parse-error",
			wantMsg:  "parse XML",
		},
		{
			name:     "malformed XML - invalid characters in element",
			schema:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="test" type="xs:string"></xs:element></xs:schema><invalid>`,
			wantCode: "schema-parse-error",
			wantMsg:  "parse XML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.schema))
			if err == nil {
				t.Fatalf("Parse() expected error with code %q, got nil", tt.wantCode)
			}

			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Parse() error = %q, want error containing %q", err.Error(), tt.wantMsg)
			}

			var parseErr *ParseError
			if errors.As(err, &parseErr) {
				if parseErr.Code != tt.wantCode {
					t.Errorf("Parse() error code = %q, want %q", parseErr.Code, tt.wantCode)
				}
			} else {
				// check if error message contains the code (for wrapped errors)
				errStr := err.Error()
				if !strings.Contains(errStr, tt.wantCode) {
					t.Errorf("Parse() error = %q, want error containing code %q", errStr, tt.wantCode)
				}
			}
		})
	}
}

func TestParseSubtreeReadErrorIncludesElementContext(t *testing.T) {
	schemaXML := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad">
    <xs:complexType>
  </xs:element>
</xs:schema>`

	_, err := Parse(strings.NewReader(schemaXML))
	if err == nil {
		t.Fatal("Parse() error = nil, want parse error")
	}
	want := "xml read for element {http://www.w3.org/2001/XMLSchema}element"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Parse() error = %v, want message containing %q", err, want)
	}
}
