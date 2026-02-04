package runtimebuild

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/schema"
)

func TestElementDefaultEmptyStringPresent(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="empty" type="xs:string" default=""/>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	elemID := elementIDForLocal(t, reg, "empty")
	ref, ok := compiled.ElementDefaults[elemID]
	if !ok {
		t.Fatalf("missing default for element empty")
	}
	if !ref.Present {
		t.Fatalf("expected default Present=true")
	}
	if ref.Len != 0 {
		t.Fatalf("expected empty default length 0, got %d", ref.Len)
	}
}

func TestAttributeFixedQNameCanonicalization(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:ex="urn:ex"
           targetNamespace="urn:ex">
  <xs:attribute name="q" type="xs:QName" fixed="ex:val"/>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	attrID := attributeIDForLocal(t, reg, "q")
	ref, ok := compiled.AttributeFixed[attrID]
	if !ok {
		t.Fatalf("missing fixed value for attribute q")
	}
	if !ref.Present {
		t.Fatalf("expected fixed Present=true")
	}
	got := compiled.Values.Blob[ref.Off : ref.Off+ref.Len]
	expected := []byte("urn:ex\x00val")
	if !bytes.Equal(got, expected) {
		t.Fatalf("expected canonical QName %q, got %q", expected, got)
	}
}

func TestDefaultRejectsEnumerationViolation(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="OnlyA">
    <xs:restriction base="xs:string">
      <xs:enumeration value="a"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="bad" type="OnlyA" default="b"/>
</xs:schema>`

	if _, err := resolveSchema(schemaXML); err == nil {
		t.Fatalf("expected enumeration violation error")
	}
}

func TestDefaultRejectsInvalidBuiltinDefaults(t *testing.T) {
	cases := []struct {
		name      string
		schemaXML string
	}{
		{
			name: "int out of range",
			schemaXML: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int" default="2147483648"/>
</xs:schema>`,
		},
		{
			name: "NCName with colon",
			schemaXML: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:NCName" default="a:b"/>
</xs:schema>`,
		},
		{
			name: "anyURI invalid percent",
			schemaXML: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyURI" default="http://example.com/%GG"/>
</xs:schema>`,
		},
		{
			name: "anyURI with spaces",
			schemaXML: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyURI" default="http://exa mple.com"/>
</xs:schema>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveSchema(tc.schemaXML); err == nil {
				t.Fatalf("expected default validation error")
			}
		})
	}
}

func TestDefaultAcceptsValidBuiltinDefaults(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="intOK" type="xs:int" default="2147483647"/>
  <xs:element name="nameOK" type="xs:NCName" default="validName"/>
  <xs:element name="uriOK" type="xs:anyURI" default="http://example.com/%20"/>
</xs:schema>`

	sch, reg, err := parseAndAssign(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := CompileValidators(sch, reg); err != nil {
		t.Fatalf("expected valid defaults, got %v", err)
	}
}

func elementIDForLocal(t *testing.T, reg *schema.Registry, local string) schema.ElemID {
	t.Helper()
	for _, entry := range reg.ElementOrder {
		if entry.QName.Local == local {
			return entry.ID
		}
	}
	t.Fatalf("element %s not found", local)
	return 0
}

func attributeIDForLocal(t *testing.T, reg *schema.Registry, local string) schema.AttrID {
	t.Helper()
	for _, entry := range reg.AttributeOrder {
		if entry.QName.Local == local {
			return entry.ID
		}
	}
	t.Fatalf("attribute %s not found", local)
	return 0
}
