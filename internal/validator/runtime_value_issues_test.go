package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestUnionStoredValueIsStable(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:string xs:token"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	validatorID := mustValidatorID(t, rt, "urn:test", "U")
	sess := NewSession(rt)

	opts := valueOptions{
		applyWhitespace:  true,
		requireCanonical: true,
		storeValue:       true,
	}
	canon, err := sess.validateValueInternalOptions(validatorID, []byte("  a  "), nil, opts)
	if err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "  a  " {
		t.Fatalf("canonical = %q, want %q", string(canon), "  a  ")
	}

	if _, err := sess.validateValueInternalOptions(validatorID, []byte("  b  "), nil, opts); err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "  a  " {
		t.Fatalf("canonical mutated to %q", string(canon))
	}
}

func TestListNormalizedValueIsStable(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="L">
    <xs:list itemType="xs:string"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	validatorID := mustValidatorID(t, rt, "urn:test", "L")
	sess := NewSession(rt)

	opts := valueOptions{
		applyWhitespace:  true,
		requireCanonical: false,
		storeValue:       false,
	}
	canon, err := sess.validateValueInternalOptions(validatorID, []byte("  a   b  "), nil, opts)
	if err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "a b" {
		t.Fatalf("normalized = %q, want %q", string(canon), "a b")
	}

	if _, err := sess.validateValueInternalOptions(validatorID, []byte("c d"), nil, opts); err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "a b" {
		t.Fatalf("normalized mutated to %q", string(canon))
	}
}

func mustValidatorID(t *testing.T, rt *runtime.Schema, ns, local string) runtime.ValidatorID {
	t.Helper()
	nsID := rt.Namespaces.Lookup([]byte(ns))
	if nsID == 0 {
		t.Fatalf("namespace %q not found", ns)
	}
	sym := rt.Symbols.Lookup(nsID, []byte(local))
	if sym == 0 {
		t.Fatalf("symbol %q not found", local)
	}
	if int(sym) >= len(rt.GlobalTypes) {
		t.Fatalf("global types missing for symbol %d", sym)
	}
	typID := rt.GlobalTypes[sym]
	if typID == 0 || int(typID) >= len(rt.Types) {
		t.Fatalf("type for %q not found", local)
	}
	return rt.Types[typID].Validator
}
