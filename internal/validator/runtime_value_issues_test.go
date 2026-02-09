package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
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
	validatorID := mustValidatorID(t, rt, "U")
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
	validatorID := mustValidatorID(t, rt, "L")
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

func TestHexBinaryCanonicalValueIsStable(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="H">
    <xs:restriction base="xs:hexBinary"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	validatorID := mustValidatorID(t, rt, "H")
	sess := NewSession(rt)

	opts := valueOptions{
		applyWhitespace:  true,
		requireCanonical: true,
		storeValue:       false,
	}
	canon, err := sess.validateValueInternalOptions(validatorID, []byte("0a"), nil, opts)
	if err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "0A" {
		t.Fatalf("canonical = %q, want %q", string(canon), "0A")
	}

	if _, err := sess.validateValueInternalOptions(validatorID, []byte("0b"), nil, opts); err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "0A" {
		t.Fatalf("canonical mutated to %q", string(canon))
	}
}

func TestBase64BinaryCanonicalValueIsStable(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="B">
    <xs:restriction base="xs:base64Binary"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	validatorID := mustValidatorID(t, rt, "B")
	sess := NewSession(rt)

	opts := valueOptions{
		applyWhitespace:  true,
		requireCanonical: true,
		storeValue:       false,
	}
	canon, err := sess.validateValueInternalOptions(validatorID, []byte(" YQ== "), nil, opts)
	if err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "YQ==" {
		t.Fatalf("canonical = %q, want %q", string(canon), "YQ==")
	}

	if _, err := sess.validateValueInternalOptions(validatorID, []byte(" Yg== "), nil, opts); err != nil {
		t.Fatalf("validate value: %v", err)
	}
	if string(canon) != "YQ==" {
		t.Fatalf("canonical mutated to %q", string(canon))
	}
}

func TestBinaryOctetLengthAllocationsStayAtParserBaseline(t *testing.T) {
	cases := []struct {
		name  string
		label string
		value []byte
		parse func([]byte) ([]byte, error)
	}{
		{
			name:  "hexBinary",
			label: "hexBinary",
			value: []byte("0A0B0C0D"),
			parse: value.ParseHexBinary,
		},
		{
			name:  "base64Binary",
			label: "base64Binary",
			value: []byte("AQIDBA=="),
			parse: value.ParseBase64Binary,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			parseAllocs := testing.AllocsPerRun(200, func() {
				if _, err := tc.parse(tc.value); err != nil {
					panic(err)
				}
			})
			lengthAllocs := testing.AllocsPerRun(200, func() {
				if _, err := binaryOctetLength(tc.parse, tc.value, nil, tc.label); err != nil {
					panic(err)
				}
			})

			// binaryOctetLength should not add per-call allocations beyond
			// the parser itself.
			if lengthAllocs > parseAllocs+0.1 {
				t.Fatalf("length allocations = %.2f, parser baseline = %.2f", lengthAllocs, parseAllocs)
			}
		})
	}
}

func mustValidatorID(t *testing.T, rt *runtime.Schema, local string) runtime.ValidatorID {
	t.Helper()
	nsID := rt.Namespaces.Lookup([]byte("urn:test"))
	if nsID == 0 {
		t.Fatalf("namespace %q not found", "urn:test")
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
