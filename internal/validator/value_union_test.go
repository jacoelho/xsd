package validator

import (
	"testing"
)

func TestUnionIntegerDecimalNoMemberMatch(t *testing.T) {
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
	validator := rt.Types[typeID].Validator

	var metrics ValueMetrics
	_, err := sess.validateValueCore(
		validator,
		[]byte("abc"),
		nil,
		valueOptions{ApplyWhitespace: true, RequireCanonical: true, NeedKey: true},
		&metrics,
	)
	if err == nil {
		t.Fatalf("expected error for value that matches no union member")
	}
}

func TestUnionIntegerDecimalSelectsMatchingMember(t *testing.T) {
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

	validator := rt.Types[typeID].Validator
	tests := []struct {
		lexical       string
		wantCanonical string
	}{
		{lexical: "12", wantCanonical: "12"},
		{lexical: "12.5", wantCanonical: "12.5"},
	}

	for _, tc := range tests {
		t.Run(tc.lexical, func(t *testing.T) {
			var metrics ValueMetrics
			canon, err := sess.validateValueCore(
				validator,
				[]byte(tc.lexical),
				nil,
				valueOptions{ApplyWhitespace: true, RequireCanonical: true, NeedKey: true},
				&metrics,
			)
			if err != nil {
				t.Fatalf("validate union %q: %v", tc.lexical, err)
			}
			if string(canon) != tc.wantCanonical {
				t.Fatalf("canonical %q = %q, want %q", tc.lexical, canon, tc.wantCanonical)
			}
		})
	}
}
