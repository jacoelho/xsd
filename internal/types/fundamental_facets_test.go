package types

import (
	"testing"
)

func TestFundamentalFacets_Ordered(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		ordered  Ordered
	}{
		{name: "total", ordered: OrderedTotal, expected: "total"},
		{name: "partial", ordered: OrderedPartial, expected: "partial"},
		{name: "none", ordered: OrderedNone, expected: "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ordered.String() != tt.expected {
				t.Errorf("Ordered.String() = %v, want %v", tt.ordered.String(), tt.expected)
			}
		})
	}
}

func TestFundamentalFacets_ForPrimitiveTypes(t *testing.T) {
	tests := []struct {
		typeName    string
		ordered     Ordered
		cardinality Cardinality
		bounded     bool
		numeric     bool
	}{
		// primitive types
		{typeName: "decimal", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: true},
		{typeName: "float", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: true},
		{typeName: "double", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: true},
		{typeName: "duration", ordered: OrderedPartial, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "dateTime", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "time", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "date", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "gYearMonth", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "gYear", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "gMonthDay", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "gDay", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "gMonth", ordered: OrderedTotal, cardinality: CardinalityUncountablyInfinite, bounded: false, numeric: false},
		{typeName: "string", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
		{typeName: "boolean", ordered: OrderedNone, cardinality: CardinalityFinite, bounded: false, numeric: false},
		{typeName: "hexBinary", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
		{typeName: "base64Binary", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
		{typeName: "anyURI", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
		{typeName: "QName", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
		{typeName: "NOTATION", ordered: OrderedNone, cardinality: CardinalityCountablyInfinite, bounded: false, numeric: false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			facets := ComputeFundamentalFacets(TypeName(tt.typeName))
			if facets == nil {
				t.Fatalf("ComputeFundamentalFacets(%q) returned nil", tt.typeName)
			}
			if facets.Ordered != tt.ordered {
				t.Errorf("Ordered = %v, want %v", facets.Ordered, tt.ordered)
			}
			if facets.Bounded != tt.bounded {
				t.Errorf("Bounded = %v, want %v", facets.Bounded, tt.bounded)
			}
			if facets.Cardinality != tt.cardinality {
				t.Errorf("Cardinality = %v, want %v", facets.Cardinality, tt.cardinality)
			}
			if facets.Numeric != tt.numeric {
				t.Errorf("Numeric = %v, want %v", facets.Numeric, tt.numeric)
			}
		})
	}
}

func TestFundamentalFacets_Inheritance(t *testing.T) {
	// test that derived types inherit fundamental facets from base type
	baseType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	baseType.SetVariety(AtomicVariety)
	baseType.SetFundamentalFacets(&FundamentalFacets{
		Ordered:     OrderedTotal,
		Bounded:     false,
		Cardinality: CardinalityUncountablyInfinite,
		Numeric:     true,
	})

	derivedType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.SetVariety(AtomicVariety)

	// inherit facets from base
	derivedType.SetFundamentalFacets(baseType.FundamentalFacets())

	facets := derivedType.FundamentalFacets()
	if facets == nil {
		t.Fatal("FundamentalFacets() returned nil")
		return
	}
	if facets.Ordered != OrderedTotal {
		t.Errorf("Derived type should inherit Ordered = %v, got %v", OrderedTotal, facets.Ordered)
	}
	if facets.Numeric != true {
		t.Errorf("Derived type should inherit Numeric = true, got %v", facets.Numeric)
	}
}

func TestFundamentalFacets_FacetApplicability(t *testing.T) {
	tests := []struct {
		name        string
		facets      *FundamentalFacets
		facetName   string
		shouldApply bool
	}{
		{
			name: "maxInclusive with ordered total",
			facets: &FundamentalFacets{
				Ordered: OrderedTotal,
			},
			facetName:   "maxInclusive",
			shouldApply: true,
		},
		{
			name: "maxInclusive with ordered none",
			facets: &FundamentalFacets{
				Ordered: OrderedNone,
			},
			facetName:   "maxInclusive",
			shouldApply: false,
		},
		{
			name: "length with ordered none",
			facets: &FundamentalFacets{
				Ordered: OrderedNone,
			},
			facetName:   "length",
			shouldApply: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applies := IsFacetApplicable(tt.facetName, tt.facets)
			if applies != tt.shouldApply {
				t.Errorf("IsFacetApplicable(%q, facets) = %v, want %v", tt.facetName, applies, tt.shouldApply)
			}
		})
	}
}
