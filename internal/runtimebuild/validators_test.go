package runtimebuild

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schema"
)

func TestValidatorOrderAndFacetInheritance(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:maxInclusive value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Base">
    <xs:restriction base="xs:int">
      <xs:minInclusive value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	baseID := validatorIDForType(t, reg, compiled, "Base")
	derivedID := validatorIDForType(t, reg, compiled, "Derived")

	if derivedID <= baseID {
		t.Fatalf("expected derived validator %d after base %d", derivedID, baseID)
	}

	derivedOps := facetOps(compiled, derivedID)
	if !derivedOps[runtime.FMinInclusive] || !derivedOps[runtime.FMaxInclusive] {
		t.Fatalf("expected derived facets to include minInclusive and maxInclusive")
	}
}

func TestEnumCanonicalizationDecimal(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Dec">
    <xs:restriction base="xs:decimal">
      <xs:enumeration value="01"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	id := validatorIDForType(t, reg, compiled, "Dec")
	enumID := enumIDForValidator(t, compiled, id)
	values := enumValues(t, compiled, enumID)
	if len(values) != 1 {
		t.Fatalf("expected 1 enum value, got %d", len(values))
	}
	if got := string(values[0]); got != "1.0" {
		t.Fatalf("expected canonical decimal '1.0', got %q", got)
	}
}

func TestEnumCanonicalizationQName(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:ex"
           targetNamespace="urn:ex">
  <xs:simpleType name="Q">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="tns:val"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	id := validatorIDForType(t, reg, compiled, "Q")
	enumID := enumIDForValidator(t, compiled, id)
	values := enumValues(t, compiled, enumID)
	if len(values) != 1 {
		t.Fatalf("expected 1 enum value, got %d", len(values))
	}
	expected := []byte("urn:ex\x00val")
	if !bytes.Equal(values[0], expected) {
		t.Fatalf("expected canonical QName %q, got %q", expected, values[0])
	}
}

func TestEnumCanonicalizationUnionOrder(t *testing.T) {
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="U">
      <xs:enumeration value="01"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	compiled, reg := compileSchema(t, schemaXML)
	id := validatorIDForType(t, reg, compiled, "E")
	enumID := enumIDForValidator(t, compiled, id)
	values := enumValues(t, compiled, enumID)
	if len(values) != 1 {
		t.Fatalf("expected 1 enum value, got %d", len(values))
	}
	if got := string(values[0]); got != "1" {
		t.Fatalf("expected canonical union value '1', got %q", got)
	}
}

func compileSchema(t *testing.T, schemaXML string) (*CompiledValidators, *schema.Registry) {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("assign ids: %v", err)
	}
	if _, err := schema.ResolveReferences(sch, reg); err != nil {
		t.Fatalf("resolve references: %v", err)
	}
	compiled, err := CompileValidators(sch, reg)
	if err != nil {
		t.Fatalf("compile validators: %v", err)
	}
	return compiled, reg
}

func validatorIDForType(t *testing.T, reg *schema.Registry, compiled *CompiledValidators, local string) runtime.ValidatorID {
	t.Helper()
	var typeID schema.TypeID
	for _, entry := range reg.TypeOrder {
		if entry.QName.Local == local {
			typeID = entry.ID
			break
		}
	}
	if typeID == 0 {
		t.Fatalf("type %s not found", local)
	}
	id, ok := compiled.TypeValidators[typeID]
	if !ok {
		t.Fatalf("validator for %s not found", local)
	}
	return id
}

func facetOps(compiled *CompiledValidators, id runtime.ValidatorID) map[runtime.FacetOp]bool {
	meta := compiled.Validators.Meta[id]
	if meta.Facets.Len == 0 {
		return map[runtime.FacetOp]bool{}
	}
	ops := make(map[runtime.FacetOp]bool)
	start := meta.Facets.Off
	for i := uint32(0); i < meta.Facets.Len; i++ {
		ops[compiled.Facets[start+i].Op] = true
	}
	return ops
}

func enumIDForValidator(t *testing.T, compiled *CompiledValidators, id runtime.ValidatorID) runtime.EnumID {
	t.Helper()
	meta := compiled.Validators.Meta[id]
	start := meta.Facets.Off
	for i := uint32(0); i < meta.Facets.Len; i++ {
		instr := compiled.Facets[start+i]
		if instr.Op == runtime.FEnum {
			return runtime.EnumID(instr.Arg0)
		}
	}
	t.Fatalf("enum facet not found")
	return 0
}

func enumValues(t *testing.T, compiled *CompiledValidators, enumID runtime.EnumID) [][]byte {
	t.Helper()
	if enumID == 0 {
		t.Fatalf("enum ID is zero")
	}
	if int(enumID) >= len(compiled.Enums.Off) {
		t.Fatalf("enum ID %d out of range", enumID)
	}
	off := compiled.Enums.Off[enumID]
	ln := compiled.Enums.Len[enumID]
	values := make([][]byte, 0, ln)
	for i := range ln {
		ref := compiled.Enums.Values[off+i]
		if int(ref.Off+ref.Len) > len(compiled.Values.Blob) {
			t.Fatalf("value ref out of range")
		}
		value := compiled.Values.Blob[ref.Off : ref.Off+ref.Len]
		values = append(values, append([]byte(nil), value...))
	}
	return values
}
