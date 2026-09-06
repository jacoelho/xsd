package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestFacetContractUsesNewestInheritedBoundForFixedFacet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		flag       value.FacetMask
		primitive  value.PrimitiveKind
		older      string
		fixedValue string
	}{
		{name: "minInclusive", flag: value.FacetMinInclusive, primitive: value.PrimitiveDecimal, older: "1", fixedValue: "2"},
		{name: "minExclusive", flag: value.FacetMinExclusive, primitive: value.PrimitiveDecimal, older: "1", fixedValue: "2"},
		{name: "maxInclusive", flag: value.FacetMaxInclusive, primitive: value.PrimitiveDecimal, older: "10", fixedValue: "9"},
		{name: "maxExclusive", flag: value.FacetMaxExclusive, primitive: value.PrimitiveDecimal, older: "10", fixedValue: "9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			base, err := builder.Add(atomicFacetType(test.primitive, orderedFacet(test.flag, test.older, false)))
			if err != nil {
				t.Fatal(err)
			}
			middleSpec := atomicFacetType(test.primitive, orderedFacet(test.flag, test.fixedValue, true))
			middleSpec.Base = base
			middle, err := builder.Add(middleSpec)
			if err != nil {
				t.Fatal(err)
			}
			leafSpec := atomicFacetType(test.primitive, orderedFacet(test.flag, test.fixedValue, false))
			leafSpec.Base = middle
			if _, err := builder.Add(leafSpec); err != nil {
				t.Fatal(err)
			}
			if _, err := builder.Seal(); err != nil {
				t.Fatalf("same value as the nearest fixed declaration was rejected: %v", err)
			}
		})
	}
}

func TestFacetContractCarriesFixedBoundThroughUnchangedRestriction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flag      value.FacetMask
		primitive value.PrimitiveKind
		fixed     string
		changed   string
	}{
		{name: "minInclusive", flag: value.FacetMinInclusive, primitive: value.PrimitiveDecimal, fixed: "1", changed: "2"},
		{name: "minExclusive", flag: value.FacetMinExclusive, primitive: value.PrimitiveDecimal, fixed: "1", changed: "2"},
		{name: "maxInclusive", flag: value.FacetMaxInclusive, primitive: value.PrimitiveDecimal, fixed: "10", changed: "9"},
		{name: "maxExclusive", flag: value.FacetMaxExclusive, primitive: value.PrimitiveDecimal, fixed: "10", changed: "9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			base, err := builder.Add(atomicFacetType(test.primitive, orderedFacet(test.flag, test.fixed, true)))
			if err != nil {
				t.Fatal(err)
			}
			middleSpec := atomicFacetType(test.primitive, value.FacetSpec{})
			middleSpec.Base = base
			middle, err := builder.Add(middleSpec)
			if err != nil {
				t.Fatal(err)
			}
			leafSpec := atomicFacetType(test.primitive, orderedFacet(test.flag, test.changed, false))
			leafSpec.Base = middle
			if _, err := builder.Add(leafSpec); err != nil {
				t.Fatal(err)
			}
			if _, err := builder.Seal(); err == nil {
				t.Fatal("changed value passed through an inherited fixed declaration")
			}
		})
	}
}

//nolint:revive // Fixedness is facet input data in this fixture.
func orderedFacet(flag value.FacetMask, lexical string, fixed bool) value.FacetSpec {
	bound := value.BoundFacet{Lexical: lexical, Type: value.NoType, Present: true}
	var facets value.FacetSpec
	//nolint:exhaustive // This fixture covers the four ordered facets.
	switch flag {
	case value.FacetMinInclusive:
		facets.MinInclusive = bound
	case value.FacetMinExclusive:
		facets.MinExclusive = bound
	case value.FacetMaxInclusive:
		facets.MaxInclusive = bound
	case value.FacetMaxExclusive:
		facets.MaxExclusive = bound
	default:
		panic("invalid ordered facet")
	}
	if fixed {
		facets.Fixed = flag
	}
	return facets
}
