package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestTimeFacetDerivationOrdersTimezoneAdjustedBounds(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Add(atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "12:00:00-10:00", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveTime,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "12:00:00-14:00", Type: value.NoType},
			Present:     true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); !errors.Is(err, value.ErrFacet) {
		t.Fatalf("timezone-adjusted widening of maxInclusive was accepted: %v", err)
	}
}

func TestTimeFacetDerivationRejectsIncomparableTimezoneBounds(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Add(atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "12:00:00", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveTime,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "12:00:00Z", Type: value.NoType},
			Present:     true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); !errors.Is(err, value.ErrFacet) {
		t.Fatalf("incomparable timezone bounds were accepted: %v", err)
	}
}

func TestTimeFacetDerivationAllowsContainedUnknownTimezoneBound(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Add(atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "22:00:00Z", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveTime,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "00:00:00", Type: value.NoType},
			Present:     true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("contained unknown-timezone bound rejected: %v", err)
	}
}

func TestTimeFacetRuntimeComparisonUsesRecurringTimeOfDay(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	typeID, err := builder.Add(atomicFacetType(value.PrimitiveTime, value.FacetSpec{
		MinInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "10:30:00Z", Type: value.NoType},
			Present:     true,
		},
		MaxInclusive: value.BoundFacet{
			LiteralSpec: value.LiteralSpec{Lexical: "10:30:00Z", Type: value.NoType},
			Present:     true,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(typeID, "00:30:00+14:00", value.Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("timezone-adjusted recurring time violated equal bounds: %v", err)
	}
}
