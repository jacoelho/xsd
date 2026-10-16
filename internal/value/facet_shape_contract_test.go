package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestFacetShapeErrorPrecedence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		wantErr string
		spec    value.TypeSpec
	}{
		{name: "conflicting bounds before applicability", wantErr: "inclusive and exclusive bounds cannot both be present", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MinInclusive: value.BoundFacet{Lexical: "x", Type: value.NoType, Present: true},
			MinExclusive: value.BoundFacet{Lexical: "y", Type: value.NoType, Present: true},
		})},
		{name: "ordered before digits", wantErr: "ordered facets require an ordered atomic type", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MinInclusive: value.BoundFacet{Lexical: "x", Type: value.NoType, Present: true},
			TotalDigits:  value.CardinalityFacet{Value: 0, Present: true},
		})},
		{name: "digits before lengths", wantErr: "digit facets require atomic decimal type", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{
			TotalDigits: value.CardinalityFacet{Value: 0, Present: true},
			MinLength:   value.CardinalityFacet{Value: 2, Present: true},
			MaxLength:   value.CardinalityFacet{Value: 1, Present: true},
		})},
		{name: "length applicability before zero digits", wantErr: "length facets require a length-capable type", spec: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{
			Length:      value.CardinalityFacet{Value: 1, Present: true},
			TotalDigits: value.CardinalityFacet{Value: 0, Present: true},
		})},
		{name: "union digits before length", wantErr: "digit facets require atomic decimal type", spec: value.TypeSpec{
			Variety: value.Union, Primitive: value.PrimitiveString, Base: value.NoType, ListItem: value.NoType,
			Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}, TotalDigits: value.CardinalityFacet{Value: 1, Present: true}},
		}},
		{name: "zero digits before cardinality", wantErr: "totalDigits must be positive", spec: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{
			TotalDigits:    value.CardinalityFacet{Value: 0, Present: true},
			FractionDigits: value.CardinalityFacet{Value: 1, Present: true},
		})},
		{name: "inverted length cardinality", wantErr: "facet cardinality relationship is invalid", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MinLength: value.CardinalityFacet{Value: 2, Present: true},
			MaxLength: value.CardinalityFacet{Value: 1, Present: true},
		})},
		{name: "valid decimal digits", spec: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{
			TotalDigits:    value.CardinalityFacet{Value: 1, Present: true},
			FractionDigits: value.CardinalityFacet{Value: 0, Present: true},
		})},
		{name: "equal length endpoints", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MinLength: value.CardinalityFacet{Value: 2, Present: true},
			MaxLength: value.CardinalityFacet{Value: 2, Present: true},
		})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, test.spec)
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.wantErr {
				t.Fatalf("Complete() = %v; want ErrFacet with %q", err, test.wantErr)
			}
		})
	}
}
