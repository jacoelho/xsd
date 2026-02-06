package validator

import (
	"testing"
)

func TestIsIntegerLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lexical string
		want    bool
	}{
		{name: "plain", lexical: "12", want: true},
		{name: "negative", lexical: "-12", want: true},
		{name: "positive", lexical: "+12", want: true},
		{name: "zero", lexical: "000", want: true},
		{name: "empty", lexical: "", want: false},
		{name: "sign only", lexical: "-", want: false},
		{name: "decimal", lexical: "12.5", want: false},
		{name: "exp", lexical: "12e3", want: false},
		{name: "space", lexical: " 12 ", want: false},
		{name: "word", lexical: "abc", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isIntegerLexical([]byte(tc.lexical)); got != tc.want {
				t.Fatalf("isIntegerLexical(%q) = %v, want %v", tc.lexical, got, tc.want)
			}
		})
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
			canon, _, err := sess.validateValueInternalWithMetrics(
				validator,
				[]byte(tc.lexical),
				nil,
				valueOptions{applyWhitespace: true, requireCanonical: true, needKey: true},
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
