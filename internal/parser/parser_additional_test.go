package parser

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func TestParseTopLevelDefinitions(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:example"
           xmlns:tns="urn:example"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified"
           blockDefault="extension restriction"
           finalDefault="restriction list union">
  <xs:annotation id="schema-ann">
    <xs:documentation xml:lang="en">doc</xs:documentation>
    <xs:appinfo source="urn:app">info</xs:appinfo>
  </xs:annotation>

  <xs:notation name="note" public="pub" system="sys"/>

  <xs:simpleType name="finalType" final="restriction list">
    <xs:restriction base="xs:string">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>

  <xs:attribute name="attr" type="xs:string"/>

  <xs:attributeGroup name="ag">
    <xs:attribute ref="tns:attr"/>
  </xs:attributeGroup>

  <xs:group name="g">
    <xs:sequence>
      <xs:element ref="tns:child" minOccurs="0" maxOccurs="1"/>
    </xs:sequence>
  </xs:group>

  <xs:complexType name="base">
    <xs:sequence>
      <xs:element ref="tns:child"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="extended">
    <xs:complexContent>
      <xs:extension base="tns:base">
        <xs:sequence>
          <xs:element ref="tns:child"/>
        </xs:sequence>
        <xs:anyAttribute namespace="##other" processContents="skip" id="any1"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>

  <xs:element name="child" type="xs:string"/>

  <xs:element name="root" type="tns:extended">
    <xs:key name="k1">
      <xs:selector xpath="tns:child"/>
      <xs:field xpath="@attr"/>
    </xs:key>
    <xs:keyref name="kref" refer="tns:k1">
      <xs:selector xpath="tns:child"/>
      <xs:field xpath="@attr"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	noteQName := types.QName{Namespace: "urn:example", Local: "note"}
	if schema.NotationDecls[noteQName] == nil {
		t.Fatalf("expected notation %s to be parsed", noteQName)
	}

	attrQName := types.QName{Namespace: "urn:example", Local: "attr"}
	if schema.AttributeDecls[attrQName] == nil {
		t.Fatalf("expected attribute %s to be parsed", attrQName)
	}

	groupQName := types.QName{Namespace: "urn:example", Local: "g"}
	if schema.Groups[groupQName] == nil {
		t.Fatalf("expected group %s to be parsed", groupQName)
	}

	attrGroupQName := types.QName{Namespace: "urn:example", Local: "ag"}
	if schema.AttributeGroups[attrGroupQName] == nil {
		t.Fatalf("expected attributeGroup %s to be parsed", attrGroupQName)
	}

	finalQName := types.QName{Namespace: "urn:example", Local: "finalType"}
	if st, ok := schema.TypeDefs[finalQName].(*types.SimpleType); !ok || st.Final == 0 {
		t.Fatalf("expected simpleType final to be parsed")
	}

	extendedQName := types.QName{Namespace: "urn:example", Local: "extended"}
	ct, ok := schema.TypeDefs[extendedQName].(*types.ComplexType)
	if !ok {
		t.Fatalf("expected complexType %s to be parsed", extendedQName)
	}
	content, ok := ct.Content().(*types.ComplexContent)
	if !ok || content.Extension == nil || content.Extension.AnyAttribute == nil {
		t.Fatalf("expected complexContent extension with anyAttribute")
	}

	rootQName := types.QName{Namespace: "urn:example", Local: "root"}
	root := schema.ElementDecls[rootQName]
	if root == nil || len(root.Constraints) != 2 {
		t.Fatalf("expected identity constraints on root element")
	}
	if root.Constraints[0].NamespaceContext["tns"] != "urn:example" {
		t.Fatalf("expected namespace context to include tns mapping")
	}
}

func TestParseSimpleContentRestrictionWithAttributes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:sc"
           xmlns:tns="urn:sc">
  <xs:complexType name="withSimpleContent">
    <xs:simpleContent>
      <xs:restriction base="xs:string">
        <xs:minLength value="1"/>
        <xs:pattern value="[A-Z]+"/>
        <xs:attribute name="extra" type="xs:string"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:sc", Local: "withSimpleContent"}
	ct, ok := schema.TypeDefs[qname].(*types.ComplexType)
	if !ok {
		t.Fatalf("expected complexType with simpleContent to be parsed")
	}
	content, ok := ct.Content().(*types.SimpleContent)
	if !ok || content.Restriction == nil {
		t.Fatalf("expected simpleContent restriction to be parsed")
	}
	if len(content.Restriction.Facets) == 0 {
		t.Fatalf("expected restriction facets to be parsed")
	}
	if len(content.Restriction.Attributes) != 1 {
		t.Fatalf("expected restriction attributes to be parsed")
	}
}

func TestNamespaceResolutionHelpers(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           xmlns:ex="urn:extra"
           targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:child"/>
      </xs:sequence>
      <xs:attribute ref="ex:attr"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	root := doc.DocumentElement()
	refElem := findElementWithAttr(doc, root, "element", "ref")
	if refElem == xsdxml.InvalidNode {
		t.Fatalf("expected ref element to be found")
	}

	schema := NewSchema()
	schema.TargetNamespace = "urn:test"
	schema.NamespaceDecls["tns"] = "urn:test"
	schema.NamespaceDecls["ex"] = "urn:extra"
	schema.ImportedNamespaces[schema.TargetNamespace] = map[types.NamespaceURI]bool{
		types.NamespaceURI("urn:extra"): true,
	}

	ctx := namespaceContextForElement(doc, refElem, schema)
	if ctx["tns"] != "urn:test" || ctx["xml"] == "" {
		t.Fatalf("expected namespace context to include tns and xml")
	}

	refQName, err := resolveQNameWithPolicy(doc, doc.GetAttribute(refElem, "ref"), refElem, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveElementQName error = %v", err)
	}
	if refQName.Namespace != "urn:test" || refQName.Local != "child" {
		t.Fatalf("unexpected ref QName: %s", refQName)
	}

	attrElem := findElementWithAttr(doc, root, "attribute", "ref")
	if attrElem == xsdxml.InvalidNode {
		t.Fatalf("expected attribute ref element to be found")
	}
	attrQName, err := resolveQNameWithPolicy(doc, doc.GetAttribute(attrElem, "ref"), attrElem, schema, forceEmptyNamespace)
	if err != nil {
		t.Fatalf("resolveAttributeRefQName error = %v", err)
	}
	if attrQName.Namespace != "urn:extra" || attrQName.Local != "attr" {
		t.Fatalf("unexpected attribute ref QName: %s", attrQName)
	}

	idQName, err := resolveQNameWithPolicy(doc, "tns:key", refElem, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveIdentityConstraintQName error = %v", err)
	}
	if idQName.Namespace != "urn:test" || idQName.Local != "key" {
		t.Fatalf("unexpected identity constraint QName: %s", idQName)
	}
}

func TestParseDerivationSetWithValidation(t *testing.T) {
	allowed := types.DerivationSet(types.DerivationExtension | types.DerivationRestriction)
	set, err := parseDerivationSetWithValidation("extension restriction", allowed)
	if err != nil {
		t.Fatalf("parseDerivationSetWithValidation error = %v", err)
	}
	if !set.Has(types.DerivationExtension) || !set.Has(types.DerivationRestriction) {
		t.Fatalf("expected both derivation methods in set")
	}

	if _, err := parseDerivationSetWithValidation("#all extension", allowed); err == nil {
		t.Fatalf("expected error for #all combined with other values")
	}

	if _, err := parseDerivationSetWithValidation("list", allowed); err == nil {
		t.Fatalf("expected error for disallowed derivation method")
	}

	if _, err := parseDerivationSetWithValidation("extension\u00A0restriction", allowed); err == nil {
		t.Fatalf("expected error for non-XML whitespace in derivation set")
	}
}

func TestParseSimpleTypeFinal(t *testing.T) {
	allowed := types.DerivationSet(types.DerivationRestriction | types.DerivationList | types.DerivationUnion)
	if _, err := parseDerivationSetWithValidation("restriction list", allowed); err != nil {
		t.Fatalf("parseDerivationSetWithValidation simpleType final error = %v", err)
	}
	if _, err := parseDerivationSetWithValidation("extension", allowed); err == nil {
		t.Fatalf("expected error for invalid simpleType final value")
	}
}

func TestValidateOccursInteger(t *testing.T) {
	tests := []struct {
		value string
		ok    bool
	}{
		{"0", true},
		{"1", true},
		{"-1", false},
		{"1.5", false},
		{"abc", false},
	}

	for _, tt := range tests {
		err := validateOccursInteger(tt.value)
		if (err == nil) != tt.ok {
			t.Fatalf("validateOccursInteger(%q) error = %v, ok=%v", tt.value, err, tt.ok)
		}
	}
}

func TestParseDerivationSetEmptyRejected(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
	}{
		{
			name: "schema blockDefault empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" blockDefault=""/>`,
		},
		{
			name: "schema finalDefault empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" finalDefault=""/>`,
		},
		{
			name: "element block empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" block=""/>
</xs:schema>`,
		},
		{
			name: "element final empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" final=""/>
</xs:schema>`,
		},
		{
			name: "complexType block empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ct" block=""/>
</xs:schema>`,
		},
		{
			name: "complexType final empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ct" final=""/>
</xs:schema>`,
		},
		{
			name: "simpleType final empty",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="st" final=""/>
</xs:schema>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(tt.schemaXML)); err == nil {
				t.Fatalf("expected parse error for %s", tt.name)
			}
		})
	}
}

func TestSchemaTargetNamespaceAttributeNamespaces(t *testing.T) {
	t.Run("foreign targetNamespace ignored", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:ex="urn:ex"
           ex:targetNamespace="urn:ex"/>`
		schema, err := Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if schema.TargetNamespace != "" {
			t.Fatalf("expected empty targetNamespace, got %q", schema.TargetNamespace)
		}
	})

	t.Run("xsd-prefixed targetNamespace rejected", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xs:targetNamespace="urn:x"/>`
		if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
			t.Fatalf("expected parse error for xs:targetNamespace")
		}
	})

	t.Run("unprefixed targetNamespace accepted", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:x"/>`
		schema, err := Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if schema.TargetNamespace != "urn:x" {
			t.Fatalf("expected targetNamespace urn:x, got %q", schema.TargetNamespace)
		}
	})
}

func TestSchemaXSDPrefixedAttributesRejected(t *testing.T) {
	tests := []struct {
		name      string
		schemaXML string
		wantErr   bool
	}{
		{
			name: "xsd-prefixed attribute on element",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" xs:foo="bar" type="xs:string"/>
</xs:schema>`,
			wantErr: true,
		},
		{
			name: "xsd-prefixed attribute on attribute",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="attr" xs:type="xs:string"/>
</xs:schema>`,
			wantErr: true,
		},
		{
			name: "foreign namespace attribute allowed",
			schemaXML: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:ex="urn:ex">
  <xs:element name="root" ex:ext="v" type="xs:string"/>
</xs:schema>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.schemaXML))
			if tt.wantErr && err == nil {
				t.Fatalf("expected parse error for %s", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
		})
	}
}

func TestParseErrorUnwrap(t *testing.T) {
	inner := errors.New("boom")
	err := newParseError("parse fail", inner)
	if !errors.Is(err, inner) {
		t.Fatalf("expected errors.Is to match inner error")
	}
}

func TestValidateIDAttribute(t *testing.T) {
	schema := NewSchema()
	if err := validateIDAttribute("1bad", "element", schema); err == nil {
		t.Fatalf("expected invalid id error")
	}
	if err := validateIDAttribute("good", "element", schema); err != nil {
		t.Fatalf("unexpected id error: %v", err)
	}
	if err := validateIDAttribute("good", "attribute", schema); err == nil {
		t.Fatalf("expected duplicate id error")
	}
}

func TestAnnotationValidation(t *testing.T) {
	valid := `<?xml version="1.0"?>
<xs:annotation xmlns:xs="http://www.w3.org/2001/XMLSchema" id="ann1">
  <xs:documentation xml:lang="en" source="urn:doc">doc</xs:documentation>
  <xs:appinfo source="urn:app">info</xs:appinfo>
</xs:annotation>`

	doc := parseDoc(t, valid)
	if err := validateAnnotationStructure(doc, doc.DocumentElement()); err != nil {
		t.Fatalf("validateAnnotationStructure error = %v", err)
	}

	invalid := `<?xml version="1.0"?>
<xs:annotation xmlns:xs="http://www.w3.org/2001/XMLSchema" bad="x"></xs:annotation>`
	doc = parseDoc(t, invalid)
	if err := validateAnnotationStructure(doc, doc.DocumentElement()); err == nil {
		t.Fatalf("expected annotation validation error")
	}

	invalidDoc := `<?xml version="1.0"?>
<xs:documentation xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:lang=""></xs:documentation>`
	doc = parseDoc(t, invalidDoc)
	if err := validateAnnotationChildAttributes(doc, doc.DocumentElement()); err == nil {
		t.Fatalf("expected documentation xml:lang validation error")
	}
}

func TestInlineTypesAndModelGroups(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:inline"
           xmlns:tns="urn:inline">
  <xs:element name="inlineElem">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>

  <xs:simpleType name="outer">
    <xs:restriction>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:pattern value="[A-Z]+"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:inline", Local: "outer"}
	if _, ok := schema.TypeDefs[qname]; !ok {
		t.Fatalf("expected inline simpleType to be parsed")
	}
}

func TestInvalidFacetAttribute(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad">
    <xs:restriction base="xs:string">
      <xs:attribute name="a" type="xs:string"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
		t.Fatalf("expected facet attribute error")
	}
}

func TestValidateOnlyAnnotationChildren(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:pattern xmlns:xs="http://www.w3.org/2001/XMLSchema" value="a">
  <xs:element name="bad"/>
</xs:pattern>`
	doc := parseDoc(t, xmlStr)
	if err := validateOnlyAnnotationChildren(doc, doc.DocumentElement(), "pattern"); err == nil {
		t.Fatalf("expected invalid child error")
	}
}

func TestParseBoolAndOccursValues(t *testing.T) {
	if _, err := parseBoolValue("nillable", "true"); err != nil {
		t.Fatalf("parseBoolValue error = %v", err)
	}
	if _, err := parseBoolValue("nillable", "yes"); err == nil {
		t.Fatalf("expected invalid bool error")
	}

	if _, err := parseOccursValue("maxOccurs", "unbounded"); err != nil {
		t.Fatalf("parseOccursValue error = %v", err)
	}
	if _, err := parseOccursValue("minOccurs", "unbounded"); err == nil {
		t.Fatalf("expected minOccurs unbounded error")
	}
	if got, err := parseOccursValue("minOccurs", " 1 "); err != nil {
		t.Fatalf("parseOccursValue whitespace error = %v", err)
	} else if got.CmpInt(1) != 0 {
		t.Fatalf("parseOccursValue whitespace = %s, want 1", got)
	}
	if got, err := parseOccursValue("maxOccurs", "\n2\t"); err != nil {
		t.Fatalf("parseOccursValue whitespace error = %v", err)
	} else if got.CmpInt(2) != 0 {
		t.Fatalf("parseOccursValue whitespace = %s, want 2", got)
	}
	if got, err := parseOccursValue("maxOccurs", " unbounded "); err != nil {
		t.Fatalf("parseOccursValue whitespace unbounded error = %v", err)
	} else if !got.IsUnbounded() {
		t.Fatalf("parseOccursValue whitespace unbounded = %s, want unbounded", got)
	}
	tooLarge := strconv.FormatUint(uint64(^uint32(0))+1, 10)
	if _, err := parseOccursValue("maxOccurs", tooLarge); err == nil {
		t.Fatalf("expected overflow error for maxOccurs")
	} else if !errors.Is(err, types.ErrOccursOverflow) {
		t.Fatalf("expected %v, got %v", types.ErrOccursOverflow, err)
	}
	if err := validateOccursValue("unbounded"); err == nil {
		t.Fatalf("expected validateOccursValue error")
	}
}

func TestParseOccursValueUint32On32Bit(t *testing.T) {
	if strconv.IntSize != 32 {
		t.Skip("requires 32-bit int")
	}
	got, err := parseOccursValue("maxOccurs", "3000000000")
	if err != nil {
		t.Fatalf("parseOccursValue error = %v", err)
	}
	if got.IsOverflow() {
		t.Fatalf("parseOccursValue = %s, want non-overflow", got)
	}
	if got.CmpInt(0) <= 0 {
		t.Fatalf("parseOccursValue = %s, want positive", got)
	}
}

func TestParseUnionMemberTypesNBSP(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="BadUnion">
    <xs:union memberTypes="xs:string` + "\u00A0" + `xs:int"/>
  </xs:simpleType>
</xs:schema>`

	if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
		t.Fatalf("expected union memberTypes with NBSP to fail parsing")
	}
}

func TestParseAttributeGroupWithAnyAttribute(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr">
  <xs:attribute name="a" type="xs:string"/>

  <xs:attributeGroup name="base">
    <xs:attribute ref="tns:a"/>
  </xs:attributeGroup>

  <xs:attributeGroup name="combo">
    <xs:annotation/>
    <xs:attribute ref="tns:a" use="required" fixed="x"/>
    <xs:attributeGroup ref="tns:base"/>
    <xs:anyAttribute namespace="##any" processContents="lax"/>
  </xs:attributeGroup>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:attr", Local: "combo"}
	attrGroup := schema.AttributeGroups[qname]
	if attrGroup == nil {
		t.Fatalf("expected attributeGroup %s", qname)
	}
	if len(attrGroup.Attributes) != 1 {
		t.Fatalf("expected attributeGroup to include attribute reference")
	}
	if len(attrGroup.AttrGroups) != 1 {
		t.Fatalf("expected attributeGroup to include attributeGroup reference")
	}
	if attrGroup.AnyAttribute == nil {
		t.Fatalf("expected attributeGroup to include anyAttribute")
	}
}

func TestParseComplexContentRestriction(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:cc"
           xmlns:tns="urn:cc">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="restricted">
    <xs:complexContent>
      <xs:restriction base="tns:base">
        <xs:sequence minOccurs="0">
          <xs:element name="child" type="xs:string"/>
        </xs:sequence>
        <xs:attribute name="id" type="xs:ID" use="required"/>
        <xs:anyAttribute namespace="##other" processContents="skip"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:cc", Local: "restricted"}
	ct, ok := schema.TypeDefs[qname].(*types.ComplexType)
	if !ok {
		t.Fatalf("expected complexType %s", qname)
	}
	content, ok := ct.Content().(*types.ComplexContent)
	if !ok || content.Restriction == nil {
		t.Fatalf("expected complexContent restriction")
	}
	if content.Restriction.Particle == nil {
		t.Fatalf("expected restriction particle")
	}
	if len(content.Restriction.Attributes) != 1 {
		t.Fatalf("expected restriction attributes")
	}
	if content.Restriction.AnyAttribute == nil {
		t.Fatalf("expected restriction anyAttribute")
	}
}

func TestParseSimpleContentExtension(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:scx"
           xmlns:tns="urn:scx">
  <xs:complexType name="withExt">
    <xs:simpleContent>
      <xs:extension base="xs:string">
        <xs:attribute name="lang" type="xs:language" use="optional"/>
        <xs:anyAttribute namespace="##any" processContents="lax"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:scx", Local: "withExt"}
	ct, ok := schema.TypeDefs[qname].(*types.ComplexType)
	if !ok {
		t.Fatalf("expected complexType %s", qname)
	}
	content, ok := ct.Content().(*types.SimpleContent)
	if !ok || content.Extension == nil {
		t.Fatalf("expected simpleContent extension")
	}
	if len(content.Extension.Attributes) != 1 {
		t.Fatalf("expected extension attributes")
	}
	if content.Extension.AnyAttribute == nil {
		t.Fatalf("expected extension anyAttribute")
	}
}

func TestParseModelGroupAllErrors(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:all id="all1">
      <xs:any/>
    </xs:all>
  </xs:group>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	root := doc.DocumentElement()
	allElem := findElementWithAttr(doc, root, "all", "id")
	if allElem == xsdxml.InvalidNode {
		t.Fatalf("expected all element to be found")
	}

	schema := NewSchema()
	if _, err := parseModelGroup(doc, allElem, schema); err == nil {
		t.Fatalf("expected xs:all wildcard error")
	}
}

func TestParseNotationErrors(t *testing.T) {
	missingName := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation public="pub"/>
</xs:schema>`
	if _, err := Parse(strings.NewReader(missingName)); err == nil {
		t.Fatalf("expected notation name error")
	}

	missingPublicSystem := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="n1"/>
</xs:schema>`
	if _, err := Parse(strings.NewReader(missingPublicSystem)); err == nil {
		t.Fatalf("expected notation public/system error")
	}
}

func TestParseSimpleTypeListAndUnion(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:list"
           xmlns:tns="urn:list">
  <xs:simpleType name="listType">
    <xs:list itemType="xs:integer"/>
  </xs:simpleType>

  <xs:simpleType name="unionType">
    <xs:union memberTypes="xs:string xs:int">
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:pattern value="[A-Z]+"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	listQName := types.QName{Namespace: "urn:list", Local: "listType"}
	listType, ok := schema.TypeDefs[listQName].(*types.SimpleType)
	if !ok || listType.Variety() != types.ListVariety {
		t.Fatalf("expected list simpleType")
	}
	if listType.List == nil {
		t.Fatalf("expected list definition")
	}

	unionQName := types.QName{Namespace: "urn:list", Local: "unionType"}
	unionType, ok := schema.TypeDefs[unionQName].(*types.SimpleType)
	if !ok || unionType.Variety() != types.UnionVariety {
		t.Fatalf("expected union simpleType")
	}
	if unionType.Union == nil || len(unionType.Union.MemberTypes) == 0 {
		t.Fatalf("expected union member types")
	}
	if len(unionType.Union.InlineTypes) != 1 {
		t.Fatalf("expected union inline type")
	}
}

func TestParseListAttributesRejectNonXMLWhitespace(t *testing.T) {
	unionSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badUnion">
    <xs:union memberTypes="xs:string` + "\u00A0" + `xs:int"/>
  </xs:simpleType>
</xs:schema>`
	if _, err := Parse(strings.NewReader(unionSchema)); err == nil {
		t.Fatalf("expected memberTypes NBSP error")
	}

	anySchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##targetNamespace` + "\u00A0" + `##local" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	if _, err := Parse(strings.NewReader(anySchema)); err == nil {
		t.Fatalf("expected namespace list NBSP error")
	}
}

func TestParseSimpleTypeListErrors(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList">
    <xs:list itemType="xs:string">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:list>
  </xs:simpleType>
</xs:schema>`
	if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
		t.Fatalf("expected list itemType/inline simpleType error")
	}
}

func TestParseElementReferenceAndLocal(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:elem"
           xmlns:tns="urn:elem"
           elementFormDefault="qualified">
  <xs:complexType name="ct">
    <xs:sequence>
      <xs:element ref="tns:head" minOccurs="0" maxOccurs="2"/>
      <xs:element name="local" nillable="true" form="qualified" fixed="x">
        <xs:simpleType>
          <xs:restriction base="xs:string">
            <xs:pattern value="[a-z]+"/>
          </xs:restriction>
        </xs:simpleType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	schema.TargetNamespace = "urn:elem"
	schema.ElementFormDefault = Qualified
	schema.NamespaceDecls["tns"] = "urn:elem"

	root := doc.DocumentElement()
	refElem := findElementWithAttr(doc, root, "element", "ref")
	if refElem == xsdxml.InvalidNode {
		t.Fatalf("expected ref element to be found")
	}
	refDecl, err := parseElement(doc, refElem, schema)
	if err != nil {
		t.Fatalf("parseElement ref error = %v", err)
	}
	if !refDecl.IsReference || !refDecl.MinOccurs.IsZero() || !refDecl.MaxOccurs.EqualInt(2) {
		t.Fatalf("unexpected ref element declaration")
	}

	localElem := findElementWithAttr(doc, root, "element", "name")
	if localElem == xsdxml.InvalidNode {
		t.Fatalf("expected local element to be found")
	}
	localDecl, err := parseElement(doc, localElem, schema)
	if err != nil {
		t.Fatalf("parseElement local error = %v", err)
	}
	if !localDecl.Nillable || !localDecl.HasFixed || localDecl.Fixed != "x" {
		t.Fatalf("unexpected local element attributes")
	}
	if _, ok := localDecl.Type.(*types.SimpleType); !ok {
		t.Fatalf("expected inline simpleType")
	}
}

func TestParseTopLevelElementSubstitutionGroup(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:subs"
           xmlns:tns="urn:subs">
  <xs:element name="head" type="xs:string" abstract="true" block="extension" final="restriction"/>
  <xs:element name="sub" type="xs:string" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	headQName := types.QName{Namespace: "urn:subs", Local: "head"}
	subQName := types.QName{Namespace: "urn:subs", Local: "sub"}
	head := schema.ElementDecls[headQName]
	if head == nil || !head.Abstract {
		t.Fatalf("expected abstract head element")
	}
	if len(schema.SubstitutionGroups[headQName]) != 1 || schema.SubstitutionGroups[headQName][0] != subQName {
		t.Fatalf("expected substitution group membership")
	}
}

func TestParseAttributeLocalAndReference(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr">
  <xs:attribute name="base" type="xs:string"/>
  <xs:attributeGroup name="g">
    <xs:attribute ref="tns:base" use="required" fixed="x"/>
    <xs:attribute name="local" default="d" form="qualified">
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:pattern value="[a-z]+"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:attributeGroup>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	schema.TargetNamespace = "urn:attr"
	schema.NamespaceDecls["tns"] = "urn:attr"

	root := doc.DocumentElement()
	refAttr := findElementWithAttr(doc, root, "attribute", "ref")
	if refAttr == xsdxml.InvalidNode {
		t.Fatalf("expected ref attribute to be found")
	}
	refDecl, err := parseAttribute(doc, refAttr, schema, true)
	if err != nil {
		t.Fatalf("parseAttribute ref error = %v", err)
	}
	if !refDecl.IsReference || refDecl.Use != types.Required || !refDecl.HasFixed {
		t.Fatalf("unexpected reference attribute declaration")
	}

	localAttr := findElementWithAttr(doc, root, "attribute", "default")
	if localAttr == xsdxml.InvalidNode {
		t.Fatalf("expected local attribute to be found")
	}
	localDecl, err := parseAttribute(doc, localAttr, schema, true)
	if err != nil {
		t.Fatalf("parseAttribute local error = %v", err)
	}
	if localDecl.Default != "d" || localDecl.Form != types.FormQualified {
		t.Fatalf("unexpected local attribute values")
	}
	if _, ok := localDecl.Type.(*types.SimpleType); !ok {
		t.Fatalf("expected inline simpleType for local attribute")
	}
}

func TestParseAttributeProhibitedFixedLocalAllowed(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use="prohibited" fixed="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()

	root := doc.DocumentElement()
	attrElem := findElementWithAttr(doc, root, "attribute", "fixed")
	if attrElem == xsdxml.InvalidNode {
		t.Fatalf("expected attribute with fixed to be found")
	}
	if _, err := parseAttribute(doc, attrElem, schema, true); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestParseAttributeProhibitedFixedReferenceAllowed(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr">
  <xs:attribute name="base" type="xs:string"/>
  <xs:complexType name="t">
    <xs:attribute ref="tns:base" use="prohibited" fixed="x"/>
  </xs:complexType>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	schema.TargetNamespace = "urn:attr"
	schema.NamespaceDecls["tns"] = "urn:attr"

	root := doc.DocumentElement()
	attrElem := findElementWithAttr(doc, root, "attribute", "ref")
	if attrElem == xsdxml.InvalidNode {
		t.Fatalf("expected attribute ref to be found")
	}
	if _, err := parseAttribute(doc, attrElem, schema, true); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestParseComplexContentExtension(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ccx"
           xmlns:tns="urn:ccx">
  <xs:attributeGroup name="ag">
    <xs:attribute name="attr" type="xs:string"/>
  </xs:attributeGroup>

  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:complexType name="extended" mixed="true">
    <xs:complexContent>
      <xs:extension base="tns:base">
        <xs:sequence>
          <xs:element name="b" type="xs:int"/>
        </xs:sequence>
        <xs:attributeGroup ref="tns:ag"/>
        <xs:anyAttribute namespace="##any" processContents="lax"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:ccx", Local: "extended"}
	ct, ok := schema.TypeDefs[qname].(*types.ComplexType)
	if !ok {
		t.Fatalf("expected complexType %s", qname)
	}
	content, ok := ct.Content().(*types.ComplexContent)
	if !ok || content.Extension == nil {
		t.Fatalf("expected complexContent extension")
	}
	if len(content.Extension.AttrGroups) != 1 || content.Extension.AnyAttribute == nil {
		t.Fatalf("expected extension attributeGroup and anyAttribute")
	}
	if ct.ResolvedBase != nil {
		t.Fatalf("expected parse phase to leave complexType base unresolved")
	}
}

func TestParseInlineComplexTypeMixedAny(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:complexType xmlns:xs="http://www.w3.org/2001/XMLSchema"
                mixed="true"
                abstract="true"
                block="extension restriction"
                final="restriction">
  <xs:any minOccurs="0" maxOccurs="unbounded" namespace="##any" processContents="lax"/>
  <xs:attribute name="a" type="xs:string" use="required"/>
  <xs:anyAttribute namespace="##other" processContents="skip"/>
</xs:complexType>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	ct, err := parseInlineComplexType(doc, doc.DocumentElement(), schema)
	if err != nil {
		t.Fatalf("parseInlineComplexType error = %v", err)
	}
	if !ct.Mixed() || !ct.Abstract {
		t.Fatalf("expected mixed and abstract complexType")
	}
	if ct.AnyAttribute() == nil || len(ct.Attributes()) != 1 {
		t.Fatalf("expected attributes and anyAttribute")
	}
	if _, ok := ct.Content().(*types.ElementContent); !ok {
		t.Fatalf("expected element content with any")
	}
}

func TestParseSimpleContentDirect(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:simpleContent xmlns:xs="http://www.w3.org/2001/XMLSchema" id="sc1">
  <xs:restriction base="xs:string">
    <xs:pattern value="[a-z]+"/>
    <xs:attribute name="a" type="xs:string"/>
  </xs:restriction>
</xs:simpleContent>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	sc, err := parseSimpleContent(doc, doc.DocumentElement(), schema)
	if err != nil {
		t.Fatalf("parseSimpleContent error = %v", err)
	}
	if sc.Restriction == nil || len(sc.Restriction.Attributes) != 1 {
		t.Fatalf("expected simpleContent restriction with attribute")
	}
}

func TestParseFacetsWithPolicy(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:restriction xmlns:xs="http://www.w3.org/2001/XMLSchema" base="xs:string" id="r1">
  <xs:pattern value="x"/>
  <xs:attribute name="a" type="xs:string"/>
</xs:restriction>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	elem := doc.DocumentElement()

	restriction := &types.Restriction{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
	if err := parseFacetsWithPolicy(doc, elem, restriction, nil, schema, facetAttributesDisallowed); err == nil {
		t.Fatalf("expected facet attribute policy error")
	}

	restriction = &types.Restriction{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
	if err := parseFacetsWithPolicy(doc, elem, restriction, nil, schema, facetAttributesAllowed); err != nil {
		t.Fatalf("parseFacetsWithPolicy allowed error = %v", err)
	}
	if len(restriction.Facets) == 0 {
		t.Fatalf("expected parsed facets")
	}
}

func TestResolveQNameDefaultNamespace(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://www.w3.org/2001/XMLSchema"></xs:schema>`
	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	root := doc.DocumentElement()

	qname, err := resolveQNameWithPolicy(doc, "string", root, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveQNameWithoutBuiltin error = %v", err)
	}
	if qname.Namespace != types.XSDNamespace || qname.Local != "string" {
		t.Fatalf("unexpected QName result: %s", qname)
	}
	if _, err := resolveQNameWithPolicy(doc, "bad:local", root, schema, useDefaultNamespace); err == nil {
		t.Fatalf("expected undefined prefix error")
	}
}

func TestParseModelGroupNestedAndGroupRef(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:mg"
           xmlns:tns="urn:mg">
  <xs:group name="g">
    <xs:sequence id="seq1">
      <xs:group ref="tns:g" minOccurs="0"/>
      <xs:any namespace="##any" processContents="lax"/>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:group>
</xs:schema>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	schema.TargetNamespace = "urn:mg"
	schema.NamespaceDecls["tns"] = "urn:mg"

	root := doc.DocumentElement()
	seqElem := findElementWithAttr(doc, root, "sequence", "id")
	if seqElem == xsdxml.InvalidNode {
		t.Fatalf("expected sequence element to be found")
	}
	if _, err := parseModelGroup(doc, seqElem, schema); err != nil {
		t.Fatalf("parseModelGroup error = %v", err)
	}
}

func TestParseModelGroupRejectsEmptyID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group string
	}{
		{name: "all", group: "all"},
		{name: "choice", group: "choice"},
		{name: "sequence", group: "sequence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			xmlStr := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:` + tt.group + ` id="">
      <xs:element name="a" type="xs:string"/>
    </xs:` + tt.group + `>
  </xs:group>
</xs:schema>`

			doc := parseDoc(t, xmlStr)
			root := doc.DocumentElement()
			groupElem := findElementWithAttr(doc, root, tt.group, "id")
			if groupElem == xsdxml.InvalidNode {
				t.Fatalf("expected %s element to be found", tt.group)
			}

			schema := NewSchema()
			if _, err := parseModelGroup(doc, groupElem, schema); err == nil {
				t.Fatalf("expected %s with empty id attribute to fail", tt.group)
			}
		})
	}
}

func TestParseInlineSimpleTypeErrors(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<xs:simpleType xmlns:xs="http://www.w3.org/2001/XMLSchema" name="bad">
  <xs:restriction base="xs:string"/>
</xs:simpleType>`

	doc := parseDoc(t, xmlStr)
	schema := NewSchema()
	if _, err := parseInlineSimpleType(doc, doc.DocumentElement(), schema); err == nil {
		t.Fatalf("expected inline simpleType name error")
	}
}

func TestParseElement_IgnoresNamespacedTypeAttribute(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:ex="urn:extra">
  <xs:element name="root" ex:type="xs:string"/>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	decl, ok := schema.ElementDecls[types.QName{Local: "root"}]
	if !ok {
		t.Fatalf("element 'root' not found in schema")
	}

	bt, ok := decl.Type.(*types.BuiltinType)
	if !ok {
		t.Fatalf("element type = %T, want anyType builtin type", decl.Type)
	}
	if bt.Name().Namespace != types.XSDNamespace || bt.Name().Local != "anyType" {
		t.Fatalf("element type = %s, want xs:anyType", bt.Name())
	}
}

func TestParseComplexContent_IgnoresXMLID(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:xml="http://www.w3.org/XML/1998/namespace">
  <xs:complexType name="T">
    <xs:complexContent xml:id="1bad">
      <xs:extension base="xs:anyType"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, ok := schema.IDAttributes["1bad"]; ok {
		t.Fatalf("xml:id value should not be registered as schema id attribute")
	}
}

func TestParseAttributeDefaultsToAnySimpleType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr">
  <xs:attribute name="Code"/>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := types.QName{Namespace: "urn:attr", Local: "Code"}
	decl := schema.AttributeDecls[qname]
	if decl == nil {
		t.Fatalf("attribute %s not found in schema", qname)
	}
	if decl.Type == nil {
		t.Fatalf("attribute %s type is nil", qname)
	}
	if decl.Type.Name().Local != string(types.TypeNameAnySimpleType) {
		t.Fatalf("attribute %s type = %s, want xs:anySimpleType", qname, decl.Type.Name())
	}
}

func parseDoc(t *testing.T, xmlStr string) *xsdxml.Document {
	t.Helper()
	doc, err := xsdxml.Parse(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("parse xml: %v", err)
	}
	return doc
}

func findElementWithAttr(doc *xsdxml.Document, elem xsdxml.NodeID, localName, attrName string) xsdxml.NodeID {
	if elem == xsdxml.InvalidNode {
		return xsdxml.InvalidNode
	}
	if doc.NamespaceURI(elem) == xsdxml.XSDNamespace && doc.LocalName(elem) == localName {
		if doc.HasAttribute(elem, attrName) {
			return elem
		}
	}
	for _, child := range doc.Children(elem) {
		if found := findElementWithAttr(doc, child, localName, attrName); found != xsdxml.InvalidNode {
			return found
		}
	}
	return xsdxml.InvalidNode
}
