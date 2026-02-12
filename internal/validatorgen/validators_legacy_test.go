package validatorgen

import (
	"bytes"
	"strings"
	"testing"

	schema "github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuecodec"
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
	keys := enumKeys(t, compiled, enumID)
	if len(keys) != 1 {
		t.Fatalf("expected 1 enum key, got %d", len(keys))
	}
	key := keys[0]
	if key.kind != runtime.VKDecimal {
		t.Fatalf("expected decimal key kind, got %v", key.kind)
	}
	dec, err := num.ParseDec([]byte("01"))
	if err != nil {
		t.Fatalf("parse decimal: %v", err)
	}
	want := num.EncodeDecKey(nil, dec)
	if !bytes.Equal(key.bytes, want) {
		t.Fatalf("expected decimal key %v, got %v", want, key.bytes)
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
	keys := enumKeys(t, compiled, enumID)
	if len(keys) != 1 {
		t.Fatalf("expected 1 enum key, got %d", len(keys))
	}
	expected := valuecodec.QNameKeyStrings(0, "urn:ex", "val")
	key := keys[0]
	if key.kind != runtime.VKQName {
		t.Fatalf("expected QName key kind, got %v", key.kind)
	}
	if !bytes.Equal(key.bytes, expected) {
		t.Fatalf("expected canonical QName %q, got %q", expected, key.bytes)
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
	keys := enumKeys(t, compiled, enumID)
	if len(keys) != 2 {
		t.Fatalf("expected 2 enum keys, got %d", len(keys))
	}
	var sawInt, sawString bool
	for _, key := range keys {
		switch key.kind {
		case runtime.VKDecimal:
			intVal, err := num.ParseInt([]byte("01"))
			if err != nil {
				t.Fatalf("parse integer: %v", err)
			}
			want := num.EncodeDecKey(nil, intVal.AsDec())
			if !bytes.Equal(key.bytes, want) {
				t.Fatalf("expected integer key %v, got %v", want, key.bytes)
			}
			sawInt = true
		case runtime.VKString:
			want := valuecodec.StringKeyString(0, "01")
			if !bytes.Equal(key.bytes, want) {
				t.Fatalf("expected string key %q, got %q", want, key.bytes)
			}
			sawString = true
		}
	}
	if !sawInt || !sawString {
		t.Fatalf("expected enum keys for int and string, sawInt=%v sawString=%v", sawInt, sawString)
	}
}

func TestUnionEnumViolatesUnionPattern_CompileError(t *testing.T) {
	// Per refactor.md §6.5.2 and §12.1 item 1:
	// Union with union-level pattern + enumeration where the enum lexical
	// violates the pattern MUST fail at compile time.
	schemaXML := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="U">
      <xs:pattern value="[a-z]+"/>
      <xs:enumeration value="123"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("assign ids: %v", err)
	}
	if _, err := schema.ResolveReferences(sch, reg); err != nil {
		t.Fatalf("resolve references: %v", err)
	}
	_, err = Compile(sch, reg)
	if err == nil {
		t.Fatalf("expected compile error for enum value violating union pattern")
	}
	if !strings.Contains(err.Error(), "pattern") && !strings.Contains(err.Error(), "enumeration") {
		t.Fatalf("expected pattern/enumeration related error, got: %v", err)
	}
}

func compileSchema(t *testing.T, schemaXML string) (*CompiledValidators, *schema.Registry) {
	t.Helper()
	sch := mustResolveSchema(t, schemaXML)
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("assign ids: %v", err)
	}
	if _, err := schema.ResolveReferences(sch, reg); err != nil {
		t.Fatalf("resolve references: %v", err)
	}
	compiled, err := Compile(sch, reg)
	if err != nil {
		t.Fatalf("compile validators: %v", err)
	}
	return compiled, reg
}

func validatorIDForType(t *testing.T, reg *schema.Registry, compiled *CompiledValidators, local string) runtime.ValidatorID {
	t.Helper()
	var typeID ids.TypeID
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

type enumKey struct {
	kind  runtime.ValueKind
	bytes []byte
}

func enumKeys(t *testing.T, compiled *CompiledValidators, enumID runtime.EnumID) []enumKey {
	t.Helper()
	if enumID == 0 {
		t.Fatalf("enum ID is zero")
	}
	if int(enumID) >= len(compiled.Enums.Off) {
		t.Fatalf("enum ID %d out of range", enumID)
	}
	off := compiled.Enums.Off[enumID]
	ln := compiled.Enums.Len[enumID]
	out := make([]enumKey, 0, ln)
	for i := range ln {
		key := compiled.Enums.Keys[off+i]
		out = append(out, enumKey{kind: key.Kind, bytes: append([]byte(nil), key.Bytes...)})
	}
	return out
}
