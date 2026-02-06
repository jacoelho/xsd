package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

func TestBuildHashIdentityConstraintsDeterministic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="ka">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`

	rt1 := mustBuildRuntimeSchema(t, schemaXML)
	rt2 := mustBuildRuntimeSchema(t, schemaXML)
	if rt1.BuildHash != rt2.BuildHash {
		t.Fatalf("build hash mismatch: %d vs %d", rt1.BuildHash, rt2.BuildHash)
	}
}

func TestAttributeUsesSortedByQName(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:attr"
           targetNamespace="urn:attr"
           elementFormDefault="qualified">
  <xs:complexType name="CT">
    <xs:attribute name="b" type="xs:string"/>
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:element name="root" type="tns:CT"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	elemID := mustElemID(t, rt, "urn:attr", "root")
	elem := rt.Elements[elemID]
	typ := rt.Types[elem.Type]
	ct := rt.ComplexTypes[typ.Complex.ID]
	off := int(ct.Attrs.Off)
	end := off + int(ct.Attrs.Len)
	if end > len(rt.AttrIndex.Uses) {
		t.Fatalf("attr uses out of range")
	}

	var names []string
	for _, use := range rt.AttrIndex.Uses[off:end] {
		names = append(names, string(rt.Symbols.LocalBytes(use.Name)))
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("attribute order = %v, want [a b]", names)
	}
}

func TestKeyrefResolutionScopedToElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:keyref"
           targetNamespace="urn:keyref"
           elementFormDefault="qualified">
  <xs:element name="a">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
  <xs:element name="b">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="kb">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="kr" refer="tns:kb">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)

	elemA := mustElemID(t, rt, "urn:keyref", "a")
	elemB := mustElemID(t, rt, "urn:keyref", "b")

	keyA := findConstraintID(rt, elemA, runtime.ICKey)
	keyB := findConstraintID(rt, elemB, runtime.ICKey)
	keyRefB := findConstraintID(rt, elemB, runtime.ICKeyRef)

	if keyA == 0 || keyB == 0 || keyRefB == 0 {
		t.Fatalf("expected key/keyref constraints, got keyA=%d keyB=%d keyRefB=%d", keyA, keyB, keyRefB)
	}
	if rt.ICs[keyRefB].Referenced != keyB {
		t.Fatalf("keyref referenced %d, want %d", rt.ICs[keyRefB].Referenced, keyB)
	}
	if rt.ICs[keyRefB].Referenced == keyA {
		t.Fatalf("keyref incorrectly resolved to key on element a")
	}
}

func TestProhibitedAttributeUsePreserved(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:attr"
           targetNamespace="urn:attr"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##any" processContents="lax"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="tns:Base">
        <xs:attribute name="foo" type="xs:string" use="prohibited"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	elemID := mustElemID(t, rt, "urn:attr", "root")
	elem := rt.Elements[elemID]
	typ := rt.Types[elem.Type]
	ct := rt.ComplexTypes[typ.Complex.ID]
	off := int(ct.Attrs.Off)
	end := off + int(ct.Attrs.Len)
	if end > len(rt.AttrIndex.Uses) {
		t.Fatalf("attr uses out of range")
	}
	nsID := rt.Namespaces.Lookup([]byte("urn:attr"))
	if nsID == 0 {
		t.Fatalf("namespace urn:attr not found")
	}
	fooSym := rt.Symbols.Lookup(nsID, []byte("foo"))
	if fooSym == 0 {
		t.Fatalf("symbol foo not found")
	}

	found := false
	for _, use := range rt.AttrIndex.Uses[off:end] {
		if use.Name == fooSym {
			found = true
			if use.Use != runtime.AttrProhibited {
				t.Fatalf("foo use = %d, want prohibited", use.Use)
			}
		}
	}
	if !found {
		t.Fatalf("expected prohibited foo attribute use to be preserved")
	}
}

func TestProhibitedAttributeGroupUseIgnored(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:attributeGroup name="G">
    <xs:attribute name="a" type="xs:string" use="prohibited"/>
  </xs:attributeGroup>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attributeGroup ref="G"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="doc" type="Derived"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	elemID := mustElemID(t, rt, "", "doc")
	elem := rt.Elements[elemID]
	typ := rt.Types[elem.Type]
	ct := rt.ComplexTypes[typ.Complex.ID]
	off := int(ct.Attrs.Off)
	end := off + int(ct.Attrs.Len)
	if end > len(rt.AttrIndex.Uses) {
		t.Fatalf("attr uses out of range")
	}
	nsID := rt.Namespaces.Lookup([]byte(""))
	if nsID == 0 {
		t.Fatalf("empty namespace not found")
	}
	attrSym := rt.Symbols.Lookup(nsID, []byte("a"))
	if attrSym == 0 {
		t.Fatalf("symbol a not found")
	}

	found := false
	for _, use := range rt.AttrIndex.Uses[off:end] {
		if use.Name == attrSym {
			found = true
			if use.Use == runtime.AttrProhibited {
				t.Fatalf("attribute use unexpectedly prohibited")
			}
		}
	}
	if !found {
		t.Fatalf("expected attribute use to be present")
	}

	sess := validator.NewSession(rt)
	doc := `<doc a="1"/>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestUnionEnumerationRespectsMemberEnums(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:simpleType name="Color">
    <xs:restriction base="xs:string">
      <xs:enumeration value="red"/>
      <xs:enumeration value="blue"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="U">
    <xs:union memberTypes="tns:Color"/>
  </xs:simpleType>
  <xs:simpleType name="R">
    <xs:restriction base="tns:U">
      <xs:enumeration value="green"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	if _, err := resolveSchema(schemaXML); err == nil {
		t.Fatalf("expected union enumeration error")
	}
}

func TestUnionWhitespaceNormalizationDuringCompile(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="R">
    <xs:restriction base="tns:U">
      <xs:pattern value="\S+"/>
      <xs:enumeration value="  a  "/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	parsed := mustResolveSchema(t, schemaXML)
	if _, err := BuildSchema(parsed, BuildConfig{}); err != nil {
		t.Fatalf("build schema: %v", err)
	}
}

func TestUnionPatternCollapseDuringCompile(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="R">
    <xs:restriction base="tns:U">
      <xs:pattern value="\S+\s{2}\S+"/>
      <xs:enumeration value="a  b"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	parsed := mustResolveSchema(t, schemaXML)
	if _, err := BuildSchema(parsed, BuildConfig{}); err == nil {
		t.Fatalf("expected compile error for union pattern violating collapsed lexical form")
	}
}

func TestUnionEnumerationRespectsNestedMemberEnums(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:simpleType name="Color">
    <xs:restriction base="xs:string">
      <xs:enumeration value="red"/>
      <xs:enumeration value="blue"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="U2">
    <xs:union memberTypes="tns:Color"/>
  </xs:simpleType>
  <xs:simpleType name="U">
    <xs:union memberTypes="tns:U2"/>
  </xs:simpleType>
  <xs:simpleType name="R">
    <xs:restriction base="tns:U">
      <xs:enumeration value="green"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	if _, err := resolveSchema(schemaXML); err == nil {
		t.Fatalf("expected union enumeration error")
	}
}

func TestUnionDefaultUsesMemberWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="C">
    <xs:attribute name="u" default="  a  ">
      <xs:simpleType>
        <xs:union memberTypes="xs:string"/>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
  <xs:element name="root" type="tns:C"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	elemID := mustElemID(t, rt, "urn:union", "root")
	elem := rt.Elements[elemID]
	typ := rt.Types[elem.Type]
	if typ.Kind != runtime.TypeComplex {
		t.Fatalf("root type is not complex")
	}
	ct := rt.ComplexTypes[typ.Complex.ID]
	off := int(ct.Attrs.Off)
	end := off + int(ct.Attrs.Len)
	if off < 0 || end > len(rt.AttrIndex.Uses) {
		t.Fatalf("attr uses out of range")
	}
	nsID := rt.Namespaces.Lookup([]byte("urn:union"))
	if nsID == 0 {
		t.Fatalf("namespace urn:union not found")
	}
	attrSym := rt.Symbols.Lookup(nsID, []byte("u"))
	if attrSym == 0 {
		t.Fatalf("attribute symbol not found")
	}
	for _, use := range rt.AttrIndex.Uses[off:end] {
		if use.Name != attrSym {
			continue
		}
		if !use.Default.Present {
			t.Fatalf("attribute default missing")
		}
		got := string(valueRefBytes(rt, use.Default))
		if got != "  a  " {
			t.Fatalf("default value = %q, want %q", got, "  a  ")
		}
		return
	}
	t.Fatalf("attribute use not found")
}

func TestUnionValidatorMismatchReturnsError(t *testing.T) {
	comp := newCompiler(nil)
	_, err := comp.addUnionValidator(runtime.WS_Preserve, runtime.FacetProgramRef{}, []runtime.ValidatorID{1}, nil, "U", 0)
	if err == nil {
		t.Fatalf("expected union member mismatch error")
	}
	if !strings.Contains(err.Error(), "validators=1") || !strings.Contains(err.Error(), "memberTypes=0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustBuildRuntimeSchema(t *testing.T, schemaXML string) *runtime.Schema {
	t.Helper()
	parsed := mustResolveSchema(t, schemaXML)
	rt, err := BuildSchema(parsed, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return rt
}

func mustElemID(t *testing.T, rt *runtime.Schema, ns, local string) runtime.ElemID {
	t.Helper()
	nsID := rt.Namespaces.Lookup([]byte(ns))
	if nsID == 0 {
		t.Fatalf("namespace %q not found", ns)
	}
	sym := rt.Symbols.Lookup(nsID, []byte(local))
	if sym == 0 {
		t.Fatalf("symbol %q not found", local)
	}
	if int(sym) >= len(rt.GlobalElements) {
		t.Fatalf("global elements missing for symbol %d", sym)
	}
	elemID := rt.GlobalElements[sym]
	if elemID == 0 {
		t.Fatalf("element %q not found", local)
	}
	return elemID
}

func findConstraintID(rt *runtime.Schema, elemID runtime.ElemID, cat runtime.ICCategory) runtime.ICID {
	if rt == nil || elemID == 0 || int(elemID) >= len(rt.Elements) {
		return 0
	}
	elem := rt.Elements[elemID]
	off := int(elem.ICOff)
	end := off + int(elem.ICLen)
	if off < 0 || end > len(rt.ElemICs) {
		return 0
	}
	for _, id := range rt.ElemICs[off:end] {
		if id == 0 || int(id) >= len(rt.ICs) {
			continue
		}
		if rt.ICs[id].Category == cat {
			return id
		}
	}
	return 0
}

func valueRefBytes(rt *runtime.Schema, ref runtime.ValueRef) []byte {
	if rt == nil || !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(rt.Values.Blob) {
		return nil
	}
	return rt.Values.Blob[start:end]
}
