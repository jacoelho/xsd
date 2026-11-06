package validate

import (
	"testing"

	xsdValue "github.com/jacoelho/xsd/internal/value"
)

func durationValueForTest(t *testing.T, lexical string, needs xsdValue.Needs) xsdValue.Value {
	t.Helper()
	b := xsdValue.NewBuilder(xsdValue.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	v, err := p.Validate(xsdValue.BuiltinType(xsdValue.PrimitiveDuration), lexical, xsdValue.Resolver{}, needs, xsdValue.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFixedValueEqualUsesValueSpace(t *testing.T) {
	t.Parallel()
	const withIdentity = xsdValue.NeedCanonical | xsdValue.NeedIdentity
	fixed := durationValueForTest(t, "P1Y", withIdentity)
	for _, tc := range []struct {
		name         string
		actual       xsdValue.Value
		equal, valid bool
	}{
		{"equal in value space", durationValueForTest(t, "P12M", withIdentity), true, true},
		{"different value", durationValueForTest(t, "P13M", withIdentity), false, true},
		{"missing identity", durationValueForTest(t, "P12M", xsdValue.NeedCanonical), false, false},
	} {
		equal, valid := fixedValueEqual(tc.actual, fixed)
		if equal != tc.equal || valid != tc.valid {
			t.Fatalf("%s: got %v, %v want %v, %v", tc.name, equal, valid, tc.equal, tc.valid)
		}
	}
}
