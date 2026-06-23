package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAtomicSimpleValueFallbackOwnsTypedFacetBounds(t *testing.T) {
	t.Parallel()

	floatValue, err := ParseFloatValue(PrimitiveDouble, "1.5", 0)
	if err != nil {
		t.Fatal(err)
	}
	durationValue, err := ParseDurationValue("P1D")
	if err != nil {
		t.Fatal(err)
	}
	gValue, err := ParseGValue(PrimitiveGYear, "2000")
	if err != nil {
		t.Fatal(err)
	}
	dateValue, err := ParseTemporalValue(PrimitiveDate, "2020-01-01")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		normalized string
		facets     SimpleValueFacets
		kind       PrimitiveKind
	}{
		{
			name:       "float",
			kind:       PrimitiveDouble,
			normalized: "1.4",
			facets: SimpleValueFacets{
				MinInclusive: SimpleValueFacetLiteral{
					Canonical: "1.5",
					Actual:    PrimitiveActualValue{Kind: PrimitiveDouble, Valid: true, Float: floatValue.Value},
					Present:   true,
				},
				Facets: FacetMinInclusive,
			},
		},
		{
			name:       "duration facet",
			kind:       PrimitiveDuration,
			normalized: "PT1H",
			facets: SimpleValueFacets{
				MinInclusive: SimpleValueFacetLiteral{
					Canonical: "P1D",
					Actual:    PrimitiveActualValue{Kind: PrimitiveDuration, Valid: true, Duration: durationValue},
					Present:   true,
				},
				Facets: FacetMinInclusive,
			},
		},
		{
			name:       "gvalue",
			kind:       PrimitiveGYear,
			normalized: "1999",
			facets: SimpleValueFacets{
				MinInclusive: SimpleValueFacetLiteral{
					Canonical: "2000",
					Actual:    PrimitiveActualValue{Kind: PrimitiveGYear, Valid: true, G: gValue},
					Present:   true,
				},
				Facets: FacetMinInclusive,
			},
		},
		{
			name:       "temporal",
			kind:       PrimitiveDate,
			normalized: "2019-12-31",
			facets: SimpleValueFacets{
				MinInclusive: SimpleValueFacetLiteral{
					Canonical: "2020-01-01",
					Actual:    PrimitiveActualValue{Kind: PrimitiveDate, Valid: true, Temporal: dateValue},
					Present:   true,
				},
				Facets: FacetMinInclusive,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typ := SimpleValueType{Primitive: tt.kind, Facets: tt.facets.Facets}
			_, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
				Type:       typ,
				Facets:     tt.facets,
				Normalized: tt.normalized,
				Present:    true,
			})
			if err == nil || err.Error() != "minInclusive facet failed" {
				t.Fatalf("ValidateAtomicSimpleValueFallback() error = %v, want minInclusive failure", err)
			}
		})
	}
}

func TestValidateAtomicSimpleValueFallbackRoutesQNameAndNotationEdges(t *testing.T) {
	t.Parallel()

	qnameType := SimpleValueType{Primitive: PrimitiveQName}
	emptyFacets := SimpleValueFacets{}
	var qnameCalls, notationCalls int
	qnameResult, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:   qnameType,
		Facets: emptyFacets,
		ResolveQName: func(lexical string) (string, string, bool) {
			qnameCalls++
			if lexical != "p:name" {
				t.Fatalf("QName resolver lexical = %q", lexical)
			}
			return "urn:test", "name", true
		},
		Notation: func(ns, local string) bool {
			notationCalls++
			return false
		},
		Normalized: "p:name",
		Needs:      PrimitiveNeedCanonical,
		Present:    true,
	})
	if err != nil {
		t.Fatalf("ValidateAtomicSimpleValueFallback(QName) error = %v", err)
	}
	if qnameResult.Canonical != "{urn:test}name" {
		t.Fatalf("QName canonical = %q", qnameResult.Canonical)
	}
	if qnameCalls != 1 || notationCalls != 0 {
		t.Fatalf("edge calls: qname=%d notation=%d", qnameCalls, notationCalls)
	}

	notationType := SimpleValueType{Primitive: PrimitiveNotation}
	notationResult, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:   notationType,
		Facets: emptyFacets,
		ResolveQName: func(lexical string) (string, string, bool) {
			if lexical != "n:token" {
				t.Fatalf("NOTATION resolver lexical = %q", lexical)
			}
			return "urn:notation", "token", true
		},
		Notation: func(ns, local string) bool {
			return ns == "urn:notation" && local == "token"
		},
		Normalized: "n:token",
		Needs:      PrimitiveNeedCanonical,
		Present:    true,
	})
	if err != nil {
		t.Fatalf("ValidateAtomicSimpleValueFallback(NOTATION) error = %v", err)
	}
	if notationResult.Canonical != "{urn:notation}token" {
		t.Fatalf("NOTATION canonical = %q", notationResult.Canonical)
	}
}

func TestValidateAtomicSimpleValueFallbackSkipsQNameCanonicalWhenUnneeded(t *testing.T) {
	t.Parallel()

	qnameResult, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type: SimpleValueType{Primitive: PrimitiveQName},
		ResolveQName: func(lexical string) (string, string, bool) {
			if lexical != "p:name" {
				t.Fatalf("QName resolver lexical = %q", lexical)
			}
			return "urn:test", "name", true
		},
		Normalized: "p:name",
		Present:    true,
	})
	if err != nil {
		t.Fatalf("ValidateAtomicSimpleValueFallback(QName) error = %v", err)
	}
	if qnameResult.Canonical != "" {
		t.Fatalf("QName canonical = %q, want empty", qnameResult.Canonical)
	}

	notationResult, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type: SimpleValueType{Primitive: PrimitiveNotation},
		ResolveQName: func(lexical string) (string, string, bool) {
			if lexical != "n:token" {
				t.Fatalf("NOTATION resolver lexical = %q", lexical)
			}
			return "urn:notation", "token", true
		},
		Notation: func(ns, local string) bool {
			return ns == "urn:notation" && local == "token"
		},
		Normalized: "n:token",
		Present:    true,
	})
	if err != nil {
		t.Fatalf("ValidateAtomicSimpleValueFallback(NOTATION) error = %v", err)
	}
	if notationResult.Canonical != "" {
		t.Fatalf("NOTATION canonical = %q, want empty", notationResult.Canonical)
	}
}

func TestValidateAtomicSimpleValueFallbackDoesNotRouteIndependentPrimitivesThroughQName(t *testing.T) {
	t.Parallel()

	typ := SimpleValueType{Primitive: PrimitiveAnyURI}
	facets := SimpleValueFacets{}
	called := false
	_, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:   typ,
		Facets: facets,
		ResolveQName: func(string) (string, string, bool) {
			called = true
			return "", "", false
		},
		Normalized: "urn:test",
		Needs:      PrimitiveNeedCanonical,
		Present:    true,
	})
	if err != nil {
		t.Fatalf("ValidateAtomicSimpleValueFallback() error = %v", err)
	}
	if called {
		t.Fatal("namespace-independent primitive used QName resolver")
	}
}

func TestValidateAtomicSimpleValueFallbackReportsMissingTypedFacetLiteral(t *testing.T) {
	t.Parallel()

	typ := SimpleValueType{Primitive: PrimitiveFloat, Facets: FacetMinInclusive}
	facets := SimpleValueFacets{Facets: FacetMinInclusive}
	_, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:       typ,
		Facets:     facets,
		Normalized: "1.0",
		Present:    true,
	})
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateAtomicSimpleValueFallback() error = %v, want metadata sentinel", err)
	}
}

func TestValidateAtomicSimpleValueFallbackRejectsUndeclaredNotation(t *testing.T) {
	t.Parallel()

	typ := SimpleValueType{Primitive: PrimitiveNotation}
	facets := SimpleValueFacets{}
	_, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:       typ,
		Facets:     facets,
		Normalized: "token",
		Notation:   func(string, string) bool { return false },
		Present:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "undeclared notation") {
		t.Fatalf("ValidateAtomicSimpleValueFallback() error = %v, want undeclared notation", err)
	}
}

func TestValidateAtomicSimpleValueFallbackRejectsMissingMetadata(t *testing.T) {
	t.Parallel()

	_, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{})
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateAtomicSimpleValueFallback() error = %v, want metadata sentinel", err)
	}
}
