package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestRawIntegerMagnitudeOrdering(t *testing.T) {
	for _, bounds := range []struct {
		name, lower, upper string
		cases              []struct{ lexical, diagnostic string }
	}{
		{
			name: "positive", lower: "12345", upper: "12347",
			cases: []struct{ lexical, diagnostic string }{
				{"12346", ""},
				{"1234", "minInclusive facet failed"},
				{"123456", "maxInclusive facet failed"},
				{".", "invalid decimal"},
				{"-", "invalid decimal"},
			},
		},
		{
			name: "negative", lower: "-12347", upper: "-12345",
			cases: []struct{ lexical, diagnostic string }{
				{"-12346", ""},
				{"-123456", "minInclusive facet failed"},
				{"-22345", "minInclusive facet failed"},
				{"-12445", "minInclusive facet failed"},
				{"-1234", "maxInclusive facet failed"},
			},
		},
	} {
		t.Run(bounds.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, value.TypeSpec{
				Variety: value.Atomic, Primitive: value.PrimitiveDecimal, Builtin: value.BuiltinInteger,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.NoType, ListItem: value.NoType,
				Facets: value.FacetSpec{
					MinInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: bounds.lower, Type: value.NoType}, Present: true},
					MaxInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: bounds.upper, Type: value.NoType}, Present: true},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, test := range bounds.cases {
				t.Run(test.lexical, func(t *testing.T) {
					_, err := program.ValidateBytes(id, []byte(test.lexical), value.Resolver{}, 0, 1<<20, nil)
					if test.diagnostic == "" {
						if err != nil {
							t.Fatalf("ValidateBytes(%q): %v", test.lexical, err)
						}
					} else if err == nil || err.Error() != test.diagnostic {
						t.Fatalf("ValidateBytes(%q) error = %v, want %q", test.lexical, err, test.diagnostic)
					}
				})
			}
		})
	}
}
