package value

import "testing"

func TestDecimalTotalDigitsCountsSignificantFractionDigitsAfterZeroInteger(t *testing.T) {
	for _, test := range []struct {
		name    string
		limit   uint32
		lexical string
		valid   bool
	}{
		{name: "below", limit: 2, lexical: "0", valid: true},
		{name: "at", limit: 2, lexical: "0.05", valid: true},
		{name: "above", limit: 2, lexical: "-0.005", valid: false},
		{name: "trailing zeroes", limit: 2, lexical: "0.0500", valid: true},
		{name: "zero", limit: 1, lexical: "0.00", valid: true},
		{name: "one", limit: 1, lexical: "1.00", valid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, id := decimalTotalDigitsProgram(t, test.limit)
			_, err := program.Validate(id, test.lexical, Resolver{}, 0, 16<<20, nil)
			if (err == nil) != test.valid {
				t.Fatalf("Validate(%q) error = %v, valid = %t", test.lexical, err, test.valid)
			}
		})
	}
}

func TestDecimalParseCountsLeadingFractionZeros(t *testing.T) {
	for _, test := range []struct {
		lexical string
		want    uint32
	}{
		{lexical: "0.05", want: 2},
		{lexical: "0.005", want: 3},
		{lexical: "0.0", want: 1},
	} {
		parsed, err := ParseDecimalValue(test.lexical)
		if err != nil {
			t.Fatalf("ParseDecimalValue(%q): %v", test.lexical, err)
		}
		if parsed.TotalDigits != test.want {
			t.Errorf("ParseDecimalValue(%q).TotalDigits = %d, want %d", test.lexical, parsed.TotalDigits, test.want)
		}
	}
}

func decimalTotalDigitsProgram(t *testing.T, totalDigits uint32) (*Program, TypeID) {
	t.Helper()
	builder := NewBuilder(BuilderOptions{})
	id, err := builder.Add(TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{TotalDigits: CardinalityFacet{
			Value:   totalDigits,
			Present: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	return program, id
}
