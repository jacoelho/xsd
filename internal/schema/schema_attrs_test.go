package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// testRawNodeWithLocalAttrs returns an XSD element whose unqualified
// attributes each carry a value of the lexical form the attribute requires.
func testRawNodeWithLocalAttrs(local string, attrs []string) *schemaSyntaxNode {
	rawAttrs := make([]xmlstream.Attr, len(attrs))
	for i, attr := range attrs {
		value := "v"
		switch attr {
		case "nillable", "abstract", "mixed", "fixed":
			value = "true"
		case "minOccurs", "maxOccurs":
			value = "1"
		case "form", "elementFormDefault", "attributeFormDefault":
			value = "qualified"
		case "use":
			value = "optional"
		case "processContents":
			value = "lax"
		case "block", "final", "blockDefault", "finalDefault":
			value = "#all"
		}
		rawAttrs[i] = testRawAttr("", attr, value)
	}
	return testXSDRawNode(local, rawAttrs)
}

func TestReferenceAttributesRejectedAtStartInDocumentOrder(t *testing.T) {
	t.Parallel()
	invalid := func(name, body, msg string, line, col int) schemaStartFixture {
		return schemaStartFixture{name: name, body: body, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: msg, wantLine: line, wantCol: col}
	}
	for _, fixture := range []schemaStartFixture{
		{name: "element ref accepts occurrence attrs", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element id="i" ref="Target" minOccurs="0" maxOccurs="2"/>
    </xs:sequence>
  </xs:complexType>`},
		{name: "attribute ref accepts value constraint attrs", body: `  <xs:complexType name="T">
    <xs:attribute id="i" ref="Target" use="optional" default="x"/>
  </xs:complexType>`},
		{name: "group ref accepts occurrence attrs", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:group id="i" ref="Target" minOccurs="0" maxOccurs="2"/>
    </xs:sequence>
  </xs:complexType>`},
		{name: "foreign attributes are not reference attributes", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" f:type="xs:string" f:form=""/>
    </xs:sequence>
  </xs:complexType>`},
		invalid("element ref type", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "type"`, 4, 7),
		invalid("element ref first forbidden attribute wins", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" form="" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "form"`, 4, 7),
		invalid("element ref unknown attribute before forbidden one", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element foo="1" ref="Target" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `"element" cannot have attribute "foo"`, 4, 7),
		invalid("element ref attribute before invalid child", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" nillable="true">
        <xs:complexType/>
      </xs:element>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "nillable"`, 4, 7),
		invalid("attribute ref name before form", `  <xs:complexType name="T">
    <xs:attribute ref="Target" name="Local" form=""/>
  </xs:complexType>`, `attribute ref cannot have attribute "name"`, 3, 5),
		invalid("attribute ref form before name", `  <xs:complexType name="T">
    <xs:attribute ref="Target" form="" name="Local"/>
  </xs:complexType>`, `attribute ref cannot have attribute "form"`, 3, 5),
		invalid("attribute ref attribute before invalid child", `  <xs:complexType name="T">
    <xs:attribute ref="Target" type="xs:string">
      <xs:simpleType/>
    </xs:attribute>
  </xs:complexType>`, `attribute ref cannot have attribute "type"`, 3, 5),
		invalid("nested group name before invalid child", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="Target" name="g">
        <xs:sequence/>
      </xs:group>
    </xs:sequence>
  </xs:complexType>`, `group cannot have attribute "name"`, 4, 7),
		invalid("attributeGroup use name before invalid child", `  <xs:complexType name="T">
    <xs:attributeGroup ref="Target" name="g">
      <xs:attribute name="a"/>
    </xs:attributeGroup>
  </xs:complexType>`, `attributeGroup use cannot have attribute "name"`, 3, 5),
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSchemaStartFixture(t, fixture)
			assertSchemaStartFixture(t, err, fixture)
		})
	}
}

func TestCheckRawSchemaAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		element     string
		attrs       []string
		wantMessage string
	}{
		{
			name:    "schema accepts document attributes",
			element: "schema",
			attrs:   []string{"id", "targetNamespace", "version", "finalDefault", "blockDefault", "attributeFormDefault", "elementFormDefault"},
		},
		{
			name:        "schema rejects unrelated attribute",
			element:     "schema",
			attrs:       []string{"type"},
			wantMessage: "\"schema\" cannot have attribute \"type\"",
		},
		{
			name:    "element accepts declaration and occurrence attributes",
			element: "element",
			attrs:   []string{"id", "name", "ref", "type", "substitutionGroup", "nillable", "default", "fixed", "form", "block", "final", "abstract", "minOccurs", "maxOccurs"},
		},
		{
			name:        "attribute rejects nillable",
			element:     "attribute",
			attrs:       []string{"name", "nillable"},
			wantMessage: "\"attribute\" cannot have attribute \"nillable\"",
		},
		{
			name:    "ordinary facet accepts fixed",
			element: "maxLength",
			attrs:   []string{"id", "value", "fixed"},
		},
		{
			name:        "pattern rejects fixed",
			element:     "pattern",
			attrs:       []string{"value", "fixed"},
			wantMessage: "\"pattern\" cannot have attribute \"fixed\"",
		},
		{
			name:        "enumeration rejects fixed",
			element:     "enumeration",
			attrs:       []string{"value", "fixed"},
			wantMessage: "\"enumeration\" cannot have attribute \"fixed\"",
		},
		{
			name:        "facet rejects unknown attribute",
			element:     "maxLength",
			attrs:       []string{"value", "ref"},
			wantMessage: "\"maxLength\" cannot have attribute \"ref\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkElementChildAttributes(testRawNodeWithLocalAttrs(tt.element, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("checkSchemaAttributesSyntax() error = %v", err)
			}
		})
	}
}

func TestSchemaAttributeValueSyntax(t *testing.T) {
	t.Parallel()

	const (
		invalidAttribute = xsderrors.CodeSchemaInvalidAttribute
		occurrence       = xsderrors.CodeSchemaOccurrence
	)
	tests := []struct {
		element  string
		attr     string
		value    string
		wantCode xsderrors.Code
		wantMsg  string
	}{
		{element: "element", attr: "nillable", value: " true "},
		{element: "element", attr: "nillable", value: "yes", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "nillable"`},
		{element: "element", attr: "abstract", value: "0"},
		{element: "element", attr: "abstract", value: "", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "abstract"`},
		{element: "complexType", attr: "abstract", value: "TRUE", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "abstract"`},
		{element: "complexType", attr: "mixed", value: "1"},
		{element: "complexType", attr: "mixed", value: "2", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "mixed"`},
		{element: "complexContent", attr: "mixed", value: "no", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "mixed"`},
		{element: "maxLength", attr: "fixed", value: "false"},
		{element: "maxLength", attr: "fixed", value: "x", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "fixed"`},
		{element: "whiteSpace", attr: "fixed", value: "x", wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "fixed"`},
		{element: "element", attr: "fixed", value: "x"},
		{element: "attribute", attr: "fixed", value: "x"},
		{element: "element", attr: "minOccurs", value: "0"},
		{element: "element", attr: "minOccurs", value: "unbounded", wantCode: occurrence, wantMsg: `invalid minOccurs "unbounded"`},
		{element: "element", attr: "minOccurs", value: "99999999999"},
		{element: "sequence", attr: "minOccurs", value: "-1", wantCode: occurrence, wantMsg: `invalid minOccurs "-1"`},
		{element: "group", attr: "maxOccurs", value: "unbounded"},
		{element: "choice", attr: "maxOccurs", value: "many", wantCode: occurrence, wantMsg: `invalid maxOccurs "many"`},
		{element: "all", attr: "maxOccurs", value: "1.0", wantCode: occurrence, wantMsg: `invalid maxOccurs "1.0"`},
		{element: "any", attr: "maxOccurs", value: "", wantCode: occurrence, wantMsg: `invalid maxOccurs ""`},
		{element: "element", attr: "form", value: "qualified"},
		{element: "element", attr: "form", value: "local", wantCode: invalidAttribute, wantMsg: `invalid element form value "local"`},
		{element: "attribute", attr: "form", value: "unqualified"},
		{element: "attribute", attr: "form", value: "local", wantCode: invalidAttribute, wantMsg: `invalid attribute form "local"`},
		{element: "schema", attr: "elementFormDefault", value: "qualified"},
		{element: "schema", attr: "elementFormDefault", value: "local", wantCode: invalidAttribute, wantMsg: `invalid "elementFormDefault" value "local"`},
		{element: "schema", attr: "attributeFormDefault", value: "local", wantCode: invalidAttribute, wantMsg: `invalid "attributeFormDefault" value "local"`},
		{element: "attribute", attr: "use", value: "prohibited"},
		{element: "attribute", attr: "use", value: "sometimes", wantCode: invalidAttribute, wantMsg: "invalid attribute use sometimes"},
		{element: "any", attr: "processContents", value: "skip"},
		{element: "any", attr: "processContents", value: "open", wantCode: invalidAttribute, wantMsg: "invalid processContents"},
		{element: "anyAttribute", attr: "processContents", value: "open", wantCode: invalidAttribute, wantMsg: "invalid processContents"},
		{element: "any", attr: "namespace", value: "##other"},
		{element: "any", attr: "namespace", value: ""},
		{element: "any", attr: "namespace", value: "urn:a ##bogus", wantCode: invalidAttribute, wantMsg: `invalid wildcard namespace "##bogus"`},
		{element: "anyAttribute", attr: "namespace", value: "%zz", wantCode: invalidAttribute, wantMsg: `invalid wildcard namespace "%zz"`},
		{element: "import", attr: "namespace", value: "##bogus"},
		{element: "import", attr: "namespace", value: "", wantCode: invalidAttribute, wantMsg: "import namespace cannot be empty"},
		{element: "schema", attr: "targetNamespace", value: "urn:a"},
		{element: "schema", attr: "targetNamespace", value: "", wantCode: invalidAttribute, wantMsg: "schema targetNamespace cannot be empty"},
		{element: "schema", attr: "blockDefault", value: "#all"},
		{element: "schema", attr: "blockDefault", value: "list", wantCode: invalidAttribute, wantMsg: `schema blockDefault cannot contain "list"`},
		{element: "schema", attr: "finalDefault", value: "list union"},
		{element: "schema", attr: "finalDefault", value: "#all extension", wantCode: invalidAttribute, wantMsg: "schema finalDefault cannot combine #all with other values"},
		{element: "schema", attr: "finalDefault", value: "substitution", wantCode: invalidAttribute, wantMsg: `schema finalDefault cannot contain "substitution"`},
		{element: "element", attr: "block", value: "substitution extension"},
		{element: "element", attr: "block", value: "list", wantCode: invalidAttribute, wantMsg: `element block cannot contain "list"`},
		{element: "element", attr: "final", value: "substitution", wantCode: invalidAttribute, wantMsg: `element final cannot contain "substitution"`},
		{element: "complexType", attr: "block", value: "substitution", wantCode: invalidAttribute, wantMsg: `complexType block cannot contain "substitution"`},
		{element: "complexType", attr: "final", value: "x", wantCode: invalidAttribute, wantMsg: `invalid complexType final value "x"`},
		{element: "simpleType", attr: "final", value: "list union restriction"},
		{element: "simpleType", attr: "final", value: "extension", wantCode: invalidAttribute, wantMsg: `simpleType final cannot contain "extension"`},
		{element: "element", attr: "default", value: "#all extension"},
		{element: "enumeration", attr: "value", value: "unbounded"},
	}
	for _, tt := range tests {
		t.Run(tt.element+" "+tt.attr+"="+tt.value, func(t *testing.T) {
			t.Parallel()

			err := checkSchemaAttributeValueSyntax(tt.element, tt.attr, tt.value)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("checkSchemaAttributeValueSyntax() = %v, want no error", err)
				}
				return
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error = %T %[1]v, want structured diagnostic", err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != tt.wantCode || diagnostic.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestCompilationReportsAdmittedValueSyntaxAsInvariant(t *testing.T) {
	t.Parallel()

	present := func(value string) lexicalAttribute { return lexicalAttribute{Value: value, Present: true} }
	tests := map[string]func() error{
		"schema targetNamespace": func() error {
			_, err := parseSchemaDefaults(schemaDocumentSource{TargetNamespace: present("")})
			return err
		},
		"schema blockDefault": func() error {
			_, err := parseSchemaDefaults(schemaDocumentSource{BlockDefault: present("list")})
			return err
		},
		"schema finalDefault": func() error {
			_, err := parseSchemaDefaults(schemaDocumentSource{FinalDefault: present("#all extension")})
			return err
		},
		"schema elementFormDefault": func() error {
			_, err := parseSchemaDefaults(schemaDocumentSource{ElementFormDefault: present("local")})
			return err
		},
		"schema attributeFormDefault": func() error {
			_, err := parseSchemaDefaults(schemaDocumentSource{AttributeFormDefault: present("local")})
			return err
		},
		"derivation set": func() error {
			_, err := parseDerivationAttrWithDefault(present("list"), 0, complexTypeBlockDerivation())
			return err
		},
		"minOccurs": func() error {
			_, err := parseOccurrence(rawOccurrence{MinOccurs: "x", HasMinOccurs: true}, Limits{})
			return err
		},
		"maxOccurs": func() error {
			_, err := parseOccurrence(rawOccurrence{MaxOccurs: "x", HasMaxOccurs: true}, Limits{})
			return err
		},
		"wildcard namespace": func() error {
			names, err := newNameTable(64, []string{EmptyNamespaceURI}, nil)
			if err != nil {
				return err
			}
			_, err = parseWildcard(newNameInterner(&names), wildcardAttrs{Namespace: "urn:a ##bogus", HasNamespace: true})
			return err
		},
		"wildcard processContents": func() error {
			_, err := parseWildcard(nameInterner{}, wildcardAttrs{ProcessContents: "open", HasProcessContents: true})
			return err
		},
	}
	for name, parse := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expectDiagnostic(t, parse(), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}
