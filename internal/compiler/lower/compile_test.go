package lower

import (
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCompileProducesNamedSimpleTypeValidator(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:compile"
           xmlns:tns="urn:compile"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	compiled, sch, reg := mustCompileSchema(t, schemaXML)

	typeName := model.QName{Namespace: "urn:compile", Local: "Code"}
	typeID, ok := reg.Types[typeName]
	if !ok {
		t.Fatalf("missing type ID for %s", typeName)
	}
	vid, ok := compiled.TypeValidators[typeID]
	if !ok || vid == 0 {
		t.Fatalf("missing compiled validator for %s", typeName)
	}
	typ := sch.TypeDefs[typeName]
	if typ == nil {
		t.Fatalf("missing type definition for %s", typeName)
	}
	if got, ok := compiled.ValidatorForType(typ); !ok || got != vid {
		t.Fatalf("ValidatorForType(%s) = (%d,%v), want (%d,true)", typeName, got, ok, vid)
	}
}

func TestCompilePreservesElementDefaultAndAttributeFixed(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:defaults"
           xmlns:tns="urn:defaults"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:attribute name="status" type="xs:string" fixed="ok"/>
  <xs:element name="root" type="xs:string" default="hello"/>
</xs:schema>`

	compiled, _, reg := mustCompileSchema(t, schemaXML)

	elemName := model.QName{Namespace: "urn:defaults", Local: "root"}
	elemID, ok := reg.Elements[elemName]
	if !ok {
		t.Fatalf("missing element ID for %s", elemName)
	}
	elemDefault, ok := compiled.ElementDefault(elemID)
	if !ok || !elemDefault.Ref.Present {
		t.Fatalf("missing compiled element default for %s", elemName)
	}
	if got := string(valueRefBytes(compiled.Values, elemDefault.Ref)); got != "hello" {
		t.Fatalf("element default value = %q, want %q", got, "hello")
	}

	attrName := model.QName{Namespace: "urn:defaults", Local: "status"}
	attrID, ok := reg.Attributes[attrName]
	if !ok {
		t.Fatalf("missing attribute ID for %s", attrName)
	}
	attrFixed, ok := compiled.AttributeFixed(attrID)
	if !ok || !attrFixed.Ref.Present {
		t.Fatalf("missing compiled attribute fixed for %s", attrName)
	}
	if got := string(valueRefBytes(compiled.Values, attrFixed.Ref)); got != "ok" {
		t.Fatalf("attribute fixed value = %q, want %q", got, "ok")
	}
}

func TestCompilePropagatesValidatorMayTrackIDs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:track"
           xmlns:tns="urn:track"
           elementFormDefault="qualified">
  <xs:simpleType name="PlainList">
    <xs:list itemType="xs:double"/>
  </xs:simpleType>
  <xs:simpleType name="TrackedList">
    <xs:list itemType="xs:IDREF"/>
  </xs:simpleType>
  <xs:simpleType name="TrackedUnion">
    <xs:union memberTypes="xs:string tns:TrackedList"/>
  </xs:simpleType>
</xs:schema>`

	compiled, _, reg := mustCompileSchema(t, schemaXML)

	tests := []struct {
		name model.QName
		want bool
	}{
		{name: model.QName{Namespace: "urn:track", Local: "PlainList"}, want: false},
		{name: model.QName{Namespace: "urn:track", Local: "TrackedList"}, want: true},
		{name: model.QName{Namespace: "urn:track", Local: "TrackedUnion"}, want: true},
	}

	for _, tt := range tests {
		typeID, ok := reg.Types[tt.name]
		if !ok {
			t.Fatalf("missing type ID for %s", tt.name)
		}
		validatorID, ok := compiled.TypeValidators[typeID]
		if !ok || validatorID == 0 {
			t.Fatalf("missing validator for %s", tt.name)
		}
		got := compiled.Validators.Meta[validatorID].Flags&runtime.ValidatorMayTrackIDs != 0
		if got != tt.want {
			t.Fatalf("validator %s trackIDs = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func mustCompileSchema(t *testing.T, schemaXML string) (*CompiledValidators, *parser.Schema, *analysis.Registry) {
	t.Helper()

	sch, reg, err := parseAndAssign(schemaXML)
	if err != nil {
		t.Fatalf("parse and assign schema: %v", err)
	}
	compiled, err := Compile(sch, reg)
	if err != nil {
		t.Fatalf("compile validators: %v", err)
	}
	return compiled, sch, reg
}

func valueRefBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(values.Blob) {
		return nil
	}
	return values.Blob[start:end]
}
