package schemair

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestBuildFingerprintDeterministic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ir"
           xmlns:tns="urn:ir"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="A"/>
      <xs:enumeration value="B"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:complexType name="ItemType">
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:ItemType" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`

	first := mustIR(t, schemaXML)
	second := mustIR(t, schemaXML)
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("IR fingerprint mismatch for same schema")
	}
	if len(first.Types) == 0 {
		t.Fatalf("expected types")
	}
	if len(first.Elements) == 0 {
		t.Fatalf("expected elements")
	}
	if len(first.IdentityConstraints) != 1 {
		t.Fatalf("identity constraint count = %d, want 1", len(first.IdentityConstraints))
	}

	changed := mustIR(t, strings.Replace(schemaXML, "itemKey", "itemKey2", 1))
	if first.Fingerprint() == changed.Fingerprint() {
		t.Fatalf("expected IR fingerprint to change")
	}
}

func TestBuildRuntimeNamePlanAndGlobalIndexes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:plan"
           xmlns:tns="urn:plan"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:notation name="jpeg" public="image/jpeg" system="viewer"/>
  <xs:complexType name="ItemType">
    <xs:sequence>
      <xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="1"/>
    </xs:sequence>
    <xs:attribute name="id" type="xs:ID" use="required"/>
    <xs:anyAttribute namespace="##targetNamespace" processContents="lax"/>
  </xs:complexType>
  <xs:attribute name="globalAttr" type="xs:string"/>
  <xs:element name="root" type="tns:ItemType"/>
</xs:schema>`

	ir := mustIR(t, schemaXML)
	if len(ir.RuntimeNames.Ops) == 0 {
		t.Fatalf("expected runtime name ops")
	}
	if !hasSymbolOp(ir.RuntimeNames, Name{Namespace: "urn:plan", Local: "root"}) {
		t.Fatalf("missing root symbol op")
	}
	if !hasNamespaceOp(ir.RuntimeNames, "urn:plan") {
		t.Fatalf("missing wildcard namespace op")
	}
	if !hasName(ir.RuntimeNames.Notations, Name{Namespace: "urn:plan", Local: "jpeg"}) {
		t.Fatalf("missing notation")
	}
	if !hasGlobalType(ir.GlobalIndexes, Name{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "string"}, true) {
		t.Fatalf("missing builtin string global type")
	}
	if !hasGlobalElement(ir.GlobalIndexes, Name{Namespace: "urn:plan", Local: "root"}) {
		t.Fatalf("missing root global element")
	}
	if !hasGlobalAttribute(ir.GlobalIndexes, Name{Namespace: "urn:plan", Local: "globalAttr"}) {
		t.Fatalf("missing global attribute")
	}
}

func TestBuildTypeDescriptors(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:types"
           xmlns:tns="urn:types"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Codes">
    <xs:list itemType="tns:Code"/>
  </xs:simpleType>
  <xs:simpleType name="Either">
    <xs:union memberTypes="tns:Code xs:int"/>
  </xs:simpleType>
  <xs:complexType name="Flagged" abstract="true" final="extension" block="restriction">
    <xs:sequence/>
  </xs:complexType>
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="baseChild" type="xs:string" minOccurs="0"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Extended">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="extra" type="xs:string" minOccurs="0"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:complexContent>
      <xs:restriction base="tns:Base">
        <xs:sequence>
          <xs:element name="baseChild" type="xs:string" minOccurs="0"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Extended"/>
</xs:schema>`

	ir := mustIR(t, schemaXML)
	if len(ir.BuiltinTypes) < 2 {
		t.Fatalf("builtin type count = %d, want at least 2", len(ir.BuiltinTypes))
	}
	if got := ir.BuiltinTypes[0]; !got.AnyType || got.Name != (Name{Namespace: xsdNamespace, Local: "anyType"}) {
		t.Fatalf("first builtin = %+v, want xs:anyType", got)
	}
	if got := ir.BuiltinTypes[1]; !got.AnySimpleType || got.Base.TypeName() != (Name{Namespace: xsdNamespace, Local: "anyType"}) {
		t.Fatalf("second builtin = %+v, want xs:anySimpleType derived from xs:anyType", got)
	}

	code := mustType(t, ir, "Code")
	if code.Kind != TypeSimple || code.Base.TypeName() != (Name{Namespace: xsdNamespace, Local: "string"}) || code.Derivation != DerivationRestriction {
		t.Fatalf("Code descriptor = %+v", code)
	}
	codes := mustType(t, ir, "Codes")
	if codes.Kind != TypeSimple || codes.Base.TypeName() != (Name{Namespace: xsdNamespace, Local: "anySimpleType"}) || codes.Derivation != DerivationList {
		t.Fatalf("Codes descriptor = %+v", codes)
	}
	either := mustType(t, ir, "Either")
	if either.Kind != TypeSimple || either.Base.TypeName() != (Name{Namespace: xsdNamespace, Local: "anySimpleType"}) || either.Derivation != DerivationUnion {
		t.Fatalf("Either descriptor = %+v", either)
	}
	flagged := mustType(t, ir, "Flagged")
	if flagged.Kind != TypeComplex || !flagged.Abstract || flagged.Final != DerivationExtension || flagged.Block != DerivationRestriction {
		t.Fatalf("Flagged descriptor = %+v", flagged)
	}
	extended := mustType(t, ir, "Extended")
	if extended.Kind != TypeComplex || extended.Base.TypeName() != (Name{Namespace: "urn:types", Local: "Base"}) || extended.Derivation != DerivationExtension {
		t.Fatalf("Extended descriptor = %+v", extended)
	}
	restricted := mustType(t, ir, "Restricted")
	if restricted.Kind != TypeComplex || restricted.Base.TypeName() != (Name{Namespace: "urn:types", Local: "Base"}) || restricted.Derivation != DerivationRestriction {
		t.Fatalf("Restricted descriptor = %+v", restricted)
	}

	changed := mustIR(t, strings.Replace(schemaXML, `base="xs:string"`, `base="xs:token"`, 1))
	if ir.Fingerprint() == changed.Fingerprint() {
		t.Fatalf("expected fingerprint to change when type base changes")
	}
}

func TestBuildElementDescriptors(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:elements"
           xmlns:tns="urn:elements"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:complexType name="ItemType">
    <xs:sequence>
      <xs:element name="localSimple">
        <xs:simpleType>
          <xs:restriction base="tns:Code"/>
        </xs:simpleType>
      </xs:element>
      <xs:element name="localComplex">
        <xs:complexType>
          <xs:attribute name="code" type="tns:Code"/>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="head" type="tns:ItemType" abstract="true"/>
  <xs:element name="otherHead" type="tns:ItemType"/>
  <xs:element name="member" type="tns:ItemType" substitutionGroup="tns:head"/>
  <xs:element name="root" type="tns:ItemType" nillable="true" final="extension" block="substitution restriction"/>
</xs:schema>`

	ir := mustIR(t, schemaXML)
	root := mustElement(t, ir, "root")
	if root.TypeDecl.TypeName() != (Name{Namespace: "urn:elements", Local: "ItemType"}) ||
		!root.Nillable ||
		root.Abstract ||
		root.Final != DerivationExtension ||
		root.Block != ElementBlockSubstitution|ElementBlockRestriction {
		t.Fatalf("root descriptor = %+v", root)
	}
	head := mustElement(t, ir, "head")
	if !head.Abstract {
		t.Fatalf("head descriptor = %+v, want abstract", head)
	}
	member := mustElement(t, ir, "member")
	if member.SubstitutionHead != head.ID {
		t.Fatalf("member substitution head = %d, want %d", member.SubstitutionHead, head.ID)
	}
	localSimple := mustElement(t, ir, "localSimple")
	if localSimple.TypeDecl.TypeID() == 0 || localSimple.TypeDecl.IsBuiltin() {
		t.Fatalf("localSimple type ref = %+v, want anonymous user type", localSimple.TypeDecl)
	}
	localComplex := mustElement(t, ir, "localComplex")
	if localComplex.TypeDecl.TypeID() == 0 || localComplex.TypeDecl.IsBuiltin() {
		t.Fatalf("localComplex type ref = %+v, want anonymous user type", localComplex.TypeDecl)
	}

	assertFingerprintChanges(t, ir, schemaXML, `<xs:element name="root" type="tns:ItemType"`, `<xs:element name="root" type="xs:string"`)
	assertFingerprintChanges(t, ir, schemaXML, `block="substitution restriction"`, `block="extension"`)
	assertFingerprintChanges(t, ir, schemaXML, `nillable="true"`, `nillable="false"`)
	assertFingerprintChanges(t, ir, schemaXML, `substitutionGroup="tns:head"`, `substitutionGroup="tns:otherHead"`)
}

func TestBuildAttributeDescriptors(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attrs"
           xmlns:tns="urn:attrs"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:attribute name="globalString" type="xs:string"/>
  <xs:attribute name="globalCode" type="tns:Code" default="A"/>
  <xs:attribute name="globalFixed" type="xs:string" fixed="B"/>
  <xs:attributeGroup name="CommonAttrs">
    <xs:attribute name="groupBuiltin" type="xs:int"/>
    <xs:attribute name="groupInline">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:attribute>
  </xs:attributeGroup>
  <xs:complexType name="ItemType">
    <xs:attributeGroup ref="tns:CommonAttrs"/>
  </xs:complexType>
  <xs:element name="root" type="tns:ItemType"/>
</xs:schema>`

	ir := mustIR(t, schemaXML)
	globalString := mustAttribute(t, ir, "globalString")
	if !globalString.Global || !globalString.TypeDecl.IsBuiltin() || globalString.TypeDecl.TypeName() != (Name{Namespace: xsdNamespace, Local: "string"}) {
		t.Fatalf("globalString descriptor = %+v", globalString)
	}
	globalCode := mustAttribute(t, ir, "globalCode")
	if !globalCode.Global || globalCode.TypeDecl.TypeID() == 0 || globalCode.TypeDecl.TypeName() != (Name{Namespace: "urn:attrs", Local: "Code"}) {
		t.Fatalf("globalCode descriptor = %+v", globalCode)
	}
	groupBuiltin := mustAttribute(t, ir, "groupBuiltin")
	if groupBuiltin.Global || !groupBuiltin.TypeDecl.IsBuiltin() || groupBuiltin.TypeDecl.TypeName() != (Name{Namespace: xsdNamespace, Local: "int"}) {
		t.Fatalf("groupBuiltin descriptor = %+v", groupBuiltin)
	}
	groupInline := mustAttribute(t, ir, "groupInline")
	if groupInline.Global || groupInline.TypeDecl.TypeID() == 0 || groupInline.TypeDecl.IsBuiltin() {
		t.Fatalf("groupInline descriptor = %+v", groupInline)
	}

	assertFingerprintChanges(t, ir, schemaXML, `name="globalString"`, `name="globalString2"`)
	assertFingerprintChanges(t, ir, schemaXML, `<xs:attribute name="globalString" type="xs:string"/>`, `<xs:attribute name="globalString" type="xs:token"/>`)
}

func TestBuildRejectsNilInputs(t *testing.T) {
	if _, err := Resolve(nil, ResolveConfig{}); err == nil || err.Error() != "schema ir: document set is nil" {
		t.Fatalf("nil input error = %v", err)
	}
}

const xsdNamespace = "http://www.w3.org/2001/XMLSchema"

func mustIR(t *testing.T, schemaXML string) *Schema {
	t.Helper()

	result, err := schemaast.ParseDocumentWithImportsOptions(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	docs := &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*result.Document}}
	ir, err := Resolve(docs, ResolveConfig{})
	if err != nil {
		t.Fatalf("build IR: %v", err)
	}
	return ir
}

func hasSymbolOp(plan RuntimeNamePlan, name Name) bool {
	for _, op := range plan.Ops {
		if op.Kind == RuntimeNameSymbol && op.Name == name {
			return true
		}
	}
	return false
}

func hasNamespaceOp(plan RuntimeNamePlan, namespace string) bool {
	for _, op := range plan.Ops {
		if op.Kind == RuntimeNameNamespace && op.Namespace == namespace {
			return true
		}
	}
	return false
}

func hasName(values []Name, name Name) bool {
	for _, value := range values {
		if value == name {
			return true
		}
	}
	return false
}

func hasGlobalType(indexes GlobalIndexes, name Name, builtin bool) bool {
	for _, value := range indexes.Types {
		if value.Name == name && value.Builtin == builtin {
			return true
		}
	}
	return false
}

func hasGlobalElement(indexes GlobalIndexes, name Name) bool {
	for _, value := range indexes.Elements {
		if value.Name == name {
			return true
		}
	}
	return false
}

func hasGlobalAttribute(indexes GlobalIndexes, name Name) bool {
	for _, value := range indexes.Attributes {
		if value.Name == name {
			return true
		}
	}
	return false
}

func mustType(t *testing.T, ir *Schema, local string) TypeDecl {
	t.Helper()
	for _, typ := range ir.Types {
		if typ.Name == (Name{Namespace: "urn:types", Local: local}) {
			return typ
		}
	}
	t.Fatalf("missing type %s", local)
	return TypeDecl{}
}

func mustElement(t *testing.T, ir *Schema, local string) Element {
	t.Helper()
	for _, elem := range ir.Elements {
		if elem.Name.Local == local {
			return elem
		}
	}
	t.Fatalf("missing element %s", local)
	return Element{}
}

func mustAttribute(t *testing.T, ir *Schema, local string) Attribute {
	t.Helper()
	for _, attr := range ir.Attributes {
		if attr.Name.Local == local {
			return attr
		}
	}
	t.Fatalf("missing attribute %s", local)
	return Attribute{}
}

func assertFingerprintChanges(t *testing.T, original *Schema, schemaXML, old, replacement string) {
	t.Helper()
	changedXML := strings.Replace(schemaXML, old, replacement, 1)
	changed := mustIR(t, changedXML)
	if original.Fingerprint() == changed.Fingerprint() {
		t.Fatalf("expected fingerprint to change after replacing %q with %q", old, replacement)
	}
}
