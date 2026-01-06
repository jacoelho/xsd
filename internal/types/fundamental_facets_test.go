package types

import (
	"testing"
)

func TestFundamentalFacets_Ordered(t *testing.T) {
	tests := []struct {
		name     string
		ordered  Ordered
		expected string
	}{
		{"total", OrderedTotal, "total"},
		{"partial", OrderedPartial, "partial"},
		{"none", OrderedNone, "none"},
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
		bounded     bool
		cardinality Cardinality
		numeric     bool
	}{
		// Primitive types
		{"decimal", OrderedTotal, false, CardinalityUncountablyInfinite, true},
		{"float", OrderedTotal, false, CardinalityUncountablyInfinite, true},
		{"double", OrderedTotal, false, CardinalityUncountablyInfinite, true},
		{"duration", OrderedPartial, false, CardinalityUncountablyInfinite, false},
		{"dateTime", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"time", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"date", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"gYearMonth", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"gYear", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"gMonthDay", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"gDay", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"gMonth", OrderedTotal, false, CardinalityUncountablyInfinite, false},
		{"string", OrderedNone, false, CardinalityCountablyInfinite, false},
		{"boolean", OrderedNone, false, CardinalityFinite, false},
		{"hexBinary", OrderedNone, false, CardinalityCountablyInfinite, false},
		{"base64Binary", OrderedNone, false, CardinalityCountablyInfinite, false},
		{"anyURI", OrderedNone, false, CardinalityCountablyInfinite, false},
		{"QName", OrderedNone, false, CardinalityCountablyInfinite, false},
		{"NOTATION", OrderedNone, false, CardinalityCountablyInfinite, false},
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
	// Test that derived types inherit fundamental facets from base type
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

	// Inherit facets from base
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
