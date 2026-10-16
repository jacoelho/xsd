package valuebench

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

// BenchmarkFacetPolicyRejectedAdmission measures four independent failed completions per operation.
func BenchmarkFacetPolicyRejectedAdmission(b *testing.B) {
	atomic := func(primitive value.PrimitiveKind, facets value.FacetSpec) value.TypeSpec {
		return value.TypeSpec{
			Variety: value.Atomic, Primitive: primitive,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: value.NoType, ListItem: value.NoType, Facets: facets,
		}
	}
	cases := []struct {
		spec    value.TypeSpec
		message string
	}{
		{
			spec:    atomic(value.PrimitiveDecimal, value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}}),
			message: "length facets require a length-capable type",
		},
		{
			spec:    atomic(value.PrimitiveString, value.FacetSpec{MinInclusive: value.BoundFacet{Lexical: "x", Type: value.NoType, Present: true}}),
			message: "ordered facets require an ordered atomic type",
		},
		{
			spec:    atomic(value.PrimitiveString, value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 1, Present: true}}),
			message: "digit facets require atomic decimal type",
		},
		{
			spec: value.TypeSpec{
				Variety: value.Union, Primitive: value.PrimitiveString,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.NoType, ListItem: value.NoType,
				Union:  []value.TypeID{value.BuiltinType(value.PrimitiveString)},
				Facets: value.FacetSpec{Present: value.FacetWhiteSpace},
			},
			message: "facet is not allowed for simple type",
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, test := range cases {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				b.Fatal(err)
			}
			err = builder.Complete(id, test.spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.message {
				b.Fatalf("Complete() = %v; want %q", err, test.message)
			}
		}
	}
}
