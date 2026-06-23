package runtime

import (
	"math"
	"testing"
)

func TestValidatePrimitiveFacetRestrictionsOwnsFacetSetBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		st      SimpleType
		base    FacetSet
		step    OrderedFacetStep
		wantErr string
	}{
		{
			name: "decimal lower exceeds upper",
			st: SimpleType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Facets: FacetSet{
					bounds: testFacetBounds(
						testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveDecimal, "2")},
						testBoundFacet{FacetMaxInclusive, testCompiledLiteral(t, PrimitiveDecimal, "1")},
					),
					Present: FacetMinInclusive | FacetMaxInclusive,
				},
			},
			step:    OrderedFacetStep{MinInclusive: true, MaxInclusive: true},
			wantErr: "decimal lower bound cannot exceed upper bound",
		},
		{
			name: "time lexical timezone absence cannot loosen base lower bound",
			st: SimpleType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveTime,
				Facets: FacetSet{
					bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveTime, "01:00:00")}),
					Present: FacetMinInclusive,
				},
			},
			base: FacetSet{
				bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveTime, "01:00:00Z")}),
				Present: FacetMinInclusive,
			},
			step:    OrderedFacetStep{MinInclusive: true},
			wantErr: "minInclusive cannot be less than base lower bound",
		},
		{
			name: "float nan bounds are inconsistent",
			st: SimpleType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDouble,
				Facets: FacetSet{
					bounds: testFacetBounds(
						testBoundFacet{FacetMinInclusive, testFloatLiteral(math.NaN())},
						testBoundFacet{FacetMaxInclusive, testFloatLiteral(math.NaN())},
					),
					Present: FacetMinInclusive | FacetMaxInclusive,
				},
			},
			step:    OrderedFacetStep{MinInclusive: true, MaxInclusive: true},
			wantErr: "float lower bound cannot exceed upper bound",
		},
		{
			name: "duration equal lower bound preserves base",
			st: SimpleType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDuration,
				Facets: FacetSet{
					bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveDuration, "P1D")}),
					Present: FacetMinInclusive,
				},
			},
			base: FacetSet{
				bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveDuration, "P1D")}),
				Present: FacetMinInclusive,
			},
			step: OrderedFacetStep{MinInclusive: true},
		},
		{
			name: "g value malformed bound is an error",
			st: SimpleType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveGYear,
				Facets: FacetSet{
					bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, CompiledLiteral{Canonical: "not-a-year"}}),
					Present: FacetMinInclusive,
				},
			},
			step:    OrderedFacetStep{MinInclusive: true},
			wantErr: "invalid date/time",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePrimitiveFacetRestrictions(tt.st, tt.base, tt.step)
			if got := errorMessage(err); got != tt.wantErr {
				t.Fatalf("ValidatePrimitiveFacetRestrictions() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestOrderedFacetSetRestrictsPrimitiveFacetSets(t *testing.T) {
	t.Parallel()

	looser := FacetSet{
		bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveGYear, "1999")}),
		Present: FacetMinInclusive,
	}
	base := FacetSet{
		bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveGYear, "2000")}),
		Present: FacetMinInclusive,
	}
	if OrderedFacetSetRestricts(SimpleVarietyAtomic, PrimitiveGYear, looser, base) {
		t.Fatal("OrderedFacetSetRestricts() accepted looser gYear lower bound")
	}

	tighter := FacetSet{
		bounds:  testFacetBounds(testBoundFacet{FacetMinInclusive, testCompiledLiteral(t, PrimitiveGYear, "2001")}),
		Present: FacetMinInclusive,
	}
	if !OrderedFacetSetRestricts(SimpleVarietyAtomic, PrimitiveGYear, tighter, base) {
		t.Fatal("OrderedFacetSetRestricts() rejected tighter gYear lower bound")
	}
}

type testBoundFacet struct {
	flag    FacetMask
	literal CompiledLiteral
}

func testFacetBounds(in ...testBoundFacet) facetBounds {
	var out facetBounds
	for _, bound := range in {
		idx, ok := boundFacetIndex(bound.flag)
		if !ok {
			panic("invalid bound facet")
		}
		lit := bound.literal
		out[idx] = &lit
	}
	return out
}

func testCompiledLiteral(tb testing.TB, kind PrimitiveKind, lexical string) CompiledLiteral {
	tb.Helper()

	actual, err := ParsePrimitiveActual(kind, lexical, PrimitiveNeedCanonical|PrimitiveNeedLength)
	if err != nil {
		tb.Fatalf("ParsePrimitiveActual(%v, %q) error = %v", kind, lexical, err)
	}
	return CompiledLiteral{
		Lexical:   lexical,
		Canonical: actual.Canonical,
		Actual:    actual.Actual,
	}
}

func testFloatLiteral(value float64) CompiledLiteral {
	return CompiledLiteral{
		Canonical: "NaN",
		Actual: PrimitiveActualValue{
			Float: value,
			Kind:  PrimitiveDouble,
			Valid: true,
		},
	}
}
