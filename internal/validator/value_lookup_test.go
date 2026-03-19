package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestLookupActualUnionValidator(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:decimal"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)
	sym := rt.Symbols.Lookup(rt.PredefNS.Empty, []byte("U"))
	typeID := rt.GlobalTypes[sym]
	if typeID == 0 {
		t.Fatalf("union type not found")
	}
	unionValidator := rt.Types[typeID].Validator

	actual, err := sess.lookupActualUnionValidator(unionValidator, []byte("12.5"), nil)
	if err != nil {
		t.Fatalf("lookupActualUnionValidator() error = %v", err)
	}
	if actual == 0 {
		t.Fatal("lookupActualUnionValidator() = 0, want matching member validator")
	}
	meta, err := sess.validatorMeta(actual)
	if err != nil {
		t.Fatalf("validatorMeta() error = %v", err)
	}
	if meta.Kind != runtime.VDecimal {
		t.Fatalf("resolved member kind = %v, want %v", meta.Kind, runtime.VDecimal)
	}
}
