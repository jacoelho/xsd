package value_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestOrderedFacetApplicabilityContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kind value.PrimitiveKind
		want bool
	}{
		{"string", value.PrimitiveString, false},
		{"boolean", value.PrimitiveBoolean, false},
		{"decimal", value.PrimitiveDecimal, true},
		{"float", value.PrimitiveFloat, true},
		{"double", value.PrimitiveDouble, true},
		{"duration", value.PrimitiveDuration, true},
		{"dateTime", value.PrimitiveDateTime, true},
		{"time", value.PrimitiveTime, true},
		{"date", value.PrimitiveDate, true},
		{"gYearMonth", value.PrimitiveGYearMonth, true},
		{"gYear", value.PrimitiveGYear, true},
		{"gMonthDay", value.PrimitiveGMonthDay, true},
		{"gDay", value.PrimitiveGDay, true},
		{"gMonth", value.PrimitiveGMonth, true},
		{"hexBinary", value.PrimitiveHexBinary, false},
		{"base64Binary", value.PrimitiveBase64Binary, false},
		{"anyURI", value.PrimitiveAnyURI, false},
		{"QName", value.PrimitiveQName, false},
		{"NOTATION", value.PrimitiveNotation, false},
		{"invalid next", value.PrimitiveKind(19), false},
		{"invalid maximum", value.PrimitiveKind(255), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, facet := range []value.FacetMask{value.FacetMinInclusive, value.FacetMinExclusive, value.FacetMaxInclusive, value.FacetMaxExclusive} {
				if got := value.FacetAllowed(value.Atomic, tc.kind, facet); got != tc.want {
					t.Fatalf("FacetAllowed(atomic, %d, %d) = %t, want %t", tc.kind, facet, got, tc.want)
				}
				for _, variety := range []value.Variety{value.List, value.Union, value.Variety(255)} {
					if value.FacetAllowed(variety, tc.kind, facet) {
						t.Fatalf("FacetAllowed(%d, %d, %d) accepted a non-atomic ordered facet", variety, tc.kind, facet)
					}
				}
			}
		})
	}
}

func TestOrderedRestrictionPredicatesContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		derived, base bool
		relation      value.OrderedFacetRelation
		lower, upper  bool
	}{
		{"equal inclusive", false, false, value.OrderedFacetEqual, true, true},
		{"equal narrowing", true, false, value.OrderedFacetEqual, true, true},
		{"equal exclusive", true, true, value.OrderedFacetEqual, true, true},
		{"equal loosening", false, true, value.OrderedFacetEqual, false, false},
		{"greater", false, true, value.OrderedFacetGreater, true, false},
		{"less", true, false, value.OrderedFacetLess, false, true},
		{"present incomparable", false, false, value.OrderedFacetIncomparable, false, false},
		{"invalid relation 4 inclusive", false, false, value.OrderedFacetRelation(4), false, false},
		{"invalid relation 4 exclusive", true, true, value.OrderedFacetRelation(4), false, false},
		{"invalid relation 255 inclusive", false, false, value.OrderedFacetRelation(255), false, false},
		{"invalid relation 255 exclusive", true, true, value.OrderedFacetRelation(255), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := value.OrderedFacetLowerRestricts(tc.derived, tc.base, tc.relation); got != tc.lower {
				t.Fatalf("lower restriction = %t, want %t", got, tc.lower)
			}
			if got := value.OrderedFacetUpperRestricts(tc.derived, tc.base, tc.relation); got != tc.upper {
				t.Fatalf("upper restriction = %t, want %t", got, tc.upper)
			}
		})
	}
}

func TestOrderedRestrictionFailurePriorityContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, minimum, message string }{
		{"both loosen", "0", "lower ordered facet cannot loosen base restriction"},
		{"upper loosens", "2", "upper ordered facet cannot loosen base restriction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			base, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completionErr := builder.Complete(base, atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{
				MinInclusive: value.BoundFacet{Present: true, Lexical: "1", Type: value.NoType},
				MaxInclusive: value.BoundFacet{Present: true, Lexical: "10", Type: value.NoType},
			})); completionErr != nil {
				t.Fatal(completionErr)
			}
			derived, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{
				MinInclusive: value.BoundFacet{Present: true, Lexical: tc.minimum, Type: value.NoType},
				MaxInclusive: value.BoundFacet{Present: true, Lexical: "11", Type: value.NoType},
			})
			spec.Base = base
			err = builder.Complete(derived, spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != tc.message {
				t.Fatalf("Complete() = %v, want ErrFacet with %q", err, tc.message)
			}
		})
	}
}

func TestFloatingIncomparableIntervalContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, lower, upper string
		valid              bool
	}{
		{"NaN below numeric", "NaN", "0", false},
		{"numeric below NaN", "0", "NaN", false},
		{"NaN equal NaN", "NaN", "NaN", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, atomicFacetType(value.PrimitiveDouble, value.FacetSpec{
				MinInclusive: value.BoundFacet{Present: true, Lexical: tc.lower, Type: value.NoType},
				MaxInclusive: value.BoundFacet{Present: true, Lexical: tc.upper, Type: value.NoType},
			}))
			if !tc.valid {
				if !errors.Is(err, value.ErrFacet) || err.Error() != "ordered facet bounds are invalid" {
					t.Fatalf("Complete() = %v, want invalid ordered bounds", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBuilderMetadataProvisionalFailureAndRetryContract(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	observed := false
	resolver := value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		observed = true
		view, available := builder.TypeView(id)
		if !available || view.Variety != value.Atomic || view.Primitive != value.PrimitiveQName {
			t.Fatalf("callback TypeView() = %#v, %t, want atomic QName", view, available)
		}
		requirements, available := builder.InputRequirements(id)
		if !available || requirements.NeedsQName {
			t.Fatalf("callback InputRequirements() = %#v, %t, want provisional available metadata", requirements, available)
		}
		return value.ExpandedName{}, false
	}}
	spec := atomicFacetType(value.PrimitiveQName, value.FacetSpec{
		Enumeration: []value.LiteralSpec{{Type: value.NoType, Lexical: "p:item", Resolver: resolver}},
	})
	err = builder.Complete(id, spec)
	if err == nil || err.Error() != `enumeration "p:item": unresolved QName` {
		t.Fatalf("Complete() = %v, want enumeration QName resolution failure", err)
	}
	if !observed {
		t.Fatal("resolver never observed the provisional type")
	}
	if got, available := builder.InputRequirements(id); available || got != (value.InputRequirements{}) {
		t.Fatalf("failed InputRequirements() = %#v, %t, want zero, false", got, available)
	}
	if got, available := builder.TypeView(id); available || !reflect.DeepEqual(got, value.TypeView{}) {
		t.Fatalf("failed TypeView() = %#v, %t, want zero, false", got, available)
	}
	resolved := value.Resolver{QName: func(string) (value.ExpandedName, bool) {
		return value.ExpandedName{Namespace: "urn:test", Local: "item"}, true
	}}
	spec.Facets.Enumeration[0].Resolver = resolved
	if completionErr := builder.Complete(id, spec); completionErr != nil {
		t.Fatalf("Complete() retry = %v", completionErr)
	}
	if got, available := builder.InputRequirements(id); !available || !got.NeedsQName {
		t.Fatalf("retried InputRequirements() = %#v, %t, want QName requirement", got, available)
	}
	if got, available := builder.TypeView(id); !available || !got.HasEnumeration {
		t.Fatalf("retried TypeView() = %#v, %t, want enumeration", got, available)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(id, "p:item", resolved, 0, 16<<20, nil); err != nil {
		t.Fatalf("sealed retried QName validation = %v", err)
	}
}
