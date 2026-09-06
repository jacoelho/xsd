package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func atomicFacetType(primitive value.PrimitiveKind, facets value.FacetSpec) value.TypeSpec {
	return value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         primitive,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Facets:            facets,
	}
}

func boundFacet(lexical string) value.FacetSpec {
	literal := value.BoundFacet{
		Lexical: lexical, Type: value.NoType,
		Present: true,
	}
	return value.FacetSpec{MinInclusive: literal}
}

func assertFacetDerivationRejected(t *testing.T, base, derived value.TypeSpec) {
	t.Helper()
	builder := value.NewBuilder(value.BuilderOptions{})
	baseID, err := builder.Add(base)
	if err != nil {
		t.Fatalf("add base type: %v", err)
	}
	derived.Base = baseID
	if _, err := builder.Add(derived); err != nil {
		return
	}
	if _, err := builder.Seal(); err == nil {
		t.Fatal("facet derivation was accepted")
	}
}

func TestFacetContractRejectsFixedFacetChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    value.TypeSpec
		derived value.TypeSpec
	}{
		{
			name: "length",
			base: atomicFacetType(value.PrimitiveString, value.FacetSpec{
				Length: value.CardinalityFacet{Value: 3, Present: true},
				Fixed:  value.FacetLength,
			}),
			derived: atomicFacetType(value.PrimitiveString, value.FacetSpec{
				Length: value.CardinalityFacet{Value: 2, Present: true},
			}),
		},
		{
			name: "ordered bound",
			base: atomicFacetType(value.PrimitiveDecimal, func() value.FacetSpec {
				facets := boundFacet("1")
				facets.Fixed = value.FacetMinInclusive
				return facets
			}()),
			derived: atomicFacetType(value.PrimitiveDecimal, boundFacet("2")),
		},
		{
			name: "whiteSpace",
			base: value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         value.PrimitiveString,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              value.NoType,
				ListItem:          value.NoType,
				Facets: value.FacetSpec{
					Present: value.FacetWhiteSpace,
					Fixed:   value.FacetWhiteSpace,
				},
			},
			derived: value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         value.PrimitiveString,
				Whitespace:        value.WhitespaceReplace,
				WhitespacePresent: true,
				Base:              value.NoType,
				ListItem:          value.NoType,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertFacetDerivationRejected(t, test.base, test.derived)
		})
	}
}

func TestFacetContractRejectsWideningRestrictions(t *testing.T) {
	t.Parallel()

	cardinality := func(flag value.FacetMask, baseValue, derivedValue uint32) (value.TypeSpec, value.TypeSpec) {
		baseFacets := value.FacetSpec{}
		derivedFacets := value.FacetSpec{}
		set := func(f *value.FacetSpec, n uint32) {
			v := value.CardinalityFacet{Value: n, Present: true}
			// The fixture selects a single cardinality facet, not an arbitrary mask.
			//exhaustive:ignore
			switch flag {
			case value.FacetLength:
				f.Length = v
			case value.FacetMinLength:
				f.MinLength = v
			case value.FacetMaxLength:
				f.MaxLength = v
			case value.FacetTotalDigits:
				f.TotalDigits = v
			case value.FacetFractionDigits:
				f.FractionDigits = v
			}
		}
		set(&baseFacets, baseValue)
		set(&derivedFacets, derivedValue)
		primitive := value.PrimitiveString
		if flag == value.FacetTotalDigits || flag == value.FacetFractionDigits {
			primitive = value.PrimitiveDecimal
		}
		return atomicFacetType(primitive, baseFacets), atomicFacetType(primitive, derivedFacets)
	}

	tests := []struct {
		name    string
		base    value.TypeSpec
		derived value.TypeSpec
	}{
		{name: "minInclusive", base: atomicFacetType(value.PrimitiveDecimal, boundFacet("1")), derived: atomicFacetType(value.PrimitiveDecimal, boundFacet("0"))},
		{name: "maxInclusive", base: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{MaxInclusive: value.BoundFacet{Lexical: "10", Type: value.NoType, Present: true}}), derived: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{MaxInclusive: value.BoundFacet{Lexical: "11", Type: value.NoType, Present: true}})},
	}
	for _, flag := range []struct {
		name       string
		facet      value.FacetMask
		base, next uint32
	}{
		{name: "length", facet: value.FacetLength, base: 3, next: 4},
		{name: "minLength", facet: value.FacetMinLength, base: 2, next: 1},
		{name: "maxLength", facet: value.FacetMaxLength, base: 4, next: 5},
		{name: "totalDigits", facet: value.FacetTotalDigits, base: 3, next: 4},
		{name: "fractionDigits", facet: value.FacetFractionDigits, base: 2, next: 3},
	} {
		base, derived := cardinality(flag.facet, flag.base, flag.next)
		tests = append(tests, struct {
			name    string
			base    value.TypeSpec
			derived value.TypeSpec
		}{name: flag.name, base: base, derived: derived})
	}
	tests = append(tests, struct {
		name    string
		base    value.TypeSpec
		derived value.TypeSpec
	}{
		name: "whiteSpace",
		base: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceReplace, WhitespacePresent: true,
			Base: value.NoType, ListItem: value.NoType,
		},
		derived: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespacePreserve, WhitespacePresent: true,
			Base: value.NoType, ListItem: value.NoType,
		},
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertFacetDerivationRejected(t, test.base, test.derived)
		})
	}
}

func TestFacetContractRejectsLengthAncestryWithoutInheritedBound(t *testing.T) {
	t.Parallel()

	assertFacetDerivationRejected(t,
		atomicFacetType(value.PrimitiveString, value.FacetSpec{}),
		atomicFacetType(value.PrimitiveString, value.FacetSpec{
			Length:    value.CardinalityFacet{Value: 2, Present: true},
			MinLength: value.CardinalityFacet{Value: 1, Present: true},
		}),
	)
	assertFacetDerivationRejected(t,
		atomicFacetType(value.PrimitiveString, value.FacetSpec{
			Length: value.CardinalityFacet{Value: 3, Present: true},
		}),
		atomicFacetType(value.PrimitiveString, value.FacetSpec{
			MinLength: value.CardinalityFacet{Value: 1, Present: true},
		}),
	)
}

func TestFacetContractSupportsListAndUnionFacets(t *testing.T) {
	t.Parallel()

	pattern, err := xsdregex.Compile("a b", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	listID, err := builder.Add(value.TypeSpec{
		Variety:           value.List,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.BuiltinType(value.PrimitiveString),
		Facets: value.FacetSpec{
			Length:   value.CardinalityFacet{Value: 2, Present: true},
			Patterns: [][]*value.Pattern{{pattern}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	unionID, err := builder.Add(value.TypeSpec{
		Variety:           value.Union,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Union:             []value.TypeID{value.BuiltinType(value.PrimitiveDecimal), value.BuiltinType(value.PrimitiveString)},
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{
			{Lexical: "1", Type: value.NoType},
			{Lexical: "yes", Type: value.NoType},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id      value.TypeID
		lexical string
		valid   bool
	}{
		{id: listID, lexical: "a b", valid: true},
		{id: listID, lexical: "a c", valid: false},
		{id: listID, lexical: "a", valid: false},
		{id: unionID, lexical: "1", valid: true},
		{id: unionID, lexical: "yes", valid: true},
		{id: unionID, lexical: "2", valid: false},
	} {
		_, err := program.Validate(test.id, test.lexical, value.Resolver{}, 0, 16<<20, nil)
		if (err == nil) != test.valid {
			t.Errorf("Validate(%d, %q) error = %v, valid = %v", test.id, test.lexical, err, test.valid)
		}
	}
}

func TestFacetContractRejectsForbiddenFacetFamilies(t *testing.T) {
	t.Parallel()

	pattern, err := xsdregex.Compile("x", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		spec value.TypeSpec
	}{
		{name: "decimal length", spec: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}})},
		{name: "string bound", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{MinInclusive: value.BoundFacet{Lexical: "x", Type: value.NoType, Present: true}})},
		{name: "string digits", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 1, Present: true}})},
		{name: "list bound", spec: value.TypeSpec{Variety: value.List, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveString), Facets: value.FacetSpec{MinInclusive: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true}}}},
		{name: "list digits", spec: value.TypeSpec{Variety: value.List, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveString), Facets: value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 1, Present: true}}}},
		{name: "union length", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}}}},
		{name: "union whitespace", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Present: value.FacetWhiteSpace}}},
		{name: "union digits", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}, TotalDigits: value.CardinalityFacet{Value: 1, Present: true}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			if _, err := builder.Add(test.spec); err != nil {
				return
			}
			if _, err := builder.Seal(); err == nil {
				t.Error("forbidden facet family was accepted")
			}
		})
	}
}

func TestFacetContractRejectsPrimitiveAndVarietyChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    value.TypeSpec
		derived value.TypeSpec
	}{
		{
			name:    "primitive",
			base:    atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{}),
			derived: atomicFacetType(value.PrimitiveString, value.FacetSpec{}),
		},
		{
			name: "variety",
			base: atomicFacetType(value.PrimitiveString, value.FacetSpec{}),
			derived: value.TypeSpec{
				Variety:           value.List,
				Primitive:         value.PrimitiveString,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              value.NoType,
				ListItem:          value.BuiltinType(value.PrimitiveString),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertFacetDerivationRejected(t, test.base, test.derived)
		})
	}
}

func TestFacetContractUsesORWithinStepAndANDAcrossAncestors(t *testing.T) {
	t.Parallel()

	basePattern, err := xsdregex.Compile("[A-Z]+", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	derivedA, err := xsdregex.Compile("AX", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	derivedB, err := xsdregex.Compile("BX", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Add(atomicFacetType(value.PrimitiveString, value.FacetSpec{
		Patterns: [][]*value.Pattern{{basePattern}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	derived, err := builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets:            value.FacetSpec{Patterns: [][]*value.Pattern{{derivedA, derivedB}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		lexical string
		valid   bool
	}{
		{lexical: "AX", valid: true},
		{lexical: "BX", valid: true},
		{lexical: "AB", valid: false},
		{lexical: "ax", valid: false},
	} {
		_, err := program.Validate(derived, test.lexical, value.Resolver{}, 0, 16<<20, nil)
		if (err == nil) != test.valid {
			t.Errorf("Validate(%q) error = %v, valid = %v", test.lexical, err, test.valid)
		}
	}
}

func TestFacetContractRejectsEnumerationOutsideBaseValueSpace(t *testing.T) {
	t.Parallel()

	assertFacetDerivationRejected(t,
		atomicFacetType(value.PrimitiveString, value.FacetSpec{
			Enumeration: []value.LiteralSpec{{Lexical: "a", Type: value.NoType}},
		}),
		atomicFacetType(value.PrimitiveString, value.FacetSpec{
			Enumeration: []value.LiteralSpec{{Lexical: "b", Type: value.NoType}},
		}),
	)
}

func TestFacetContractAllowsExplicitSelfTypedBound(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(id, value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveDecimal,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{MinInclusive: value.BoundFacet{
			Lexical: "1", Type: id,
			Present: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(id, "1", value.Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("self-typed bound rejected its boundary value: %v", err)
	}
	if _, err := program.Validate(id, "0", value.Resolver{}, 0, 16<<20, nil); err == nil {
		t.Fatal("self-typed bound accepted a value below its boundary")
	}
}

func TestFacetContractAllowsDirectAnySimpleTypeVarietyDerivation(t *testing.T) {
	t.Parallel()

	anySimpleType, ok := value.BuiltinTypeID("anySimpleType")
	if !ok {
		t.Fatal("anySimpleType builtin is missing")
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	list, err := builder.Add(value.TypeSpec{
		Variety:           value.List,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              anySimpleType,
		ListItem:          value.BuiltinType(value.PrimitiveString),
		Facets: value.FacetSpec{
			Length: value.CardinalityFacet{Value: 2, Present: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	union, err := builder.Add(value.TypeSpec{
		Variety:           value.Union,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              anySimpleType,
		ListItem:          value.NoType,
		Union:             []value.TypeID{value.BuiltinType(value.PrimitiveDecimal), value.BuiltinType(value.PrimitiveString)},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(list, "a b", value.Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("list derived from anySimpleType rejected: %v", err)
	}
	if _, err := program.Validate(union, "1", value.Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("union derived from anySimpleType rejected: %v", err)
	}
}

func TestFacetContractUsesValueSpaceForFixedAndPartialOrderedBounds(t *testing.T) {
	t.Parallel()

	baseSpec := atomicFacetType(value.PrimitiveDecimal, func() value.FacetSpec {
		facets := boundFacet("1.0")
		facets.Fixed = value.FacetMinInclusive
		return facets
	}())
	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Add(baseSpec)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveDecimal,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          value.NoType,
		Facets:            boundFacet("1.00"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("value-equivalent fixed bound rejected: %v", err)
	}

	for _, test := range []struct {
		name, lexical string
		wantErr       bool
	}{
		{name: "duration narrowing", lexical: "P2D", wantErr: false},
		{name: "duration widening", lexical: "P0D", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := value.NewBuilder(value.BuilderOptions{})
			baseID, err := b.Add(atomicFacetType(value.PrimitiveDuration, boundFacet("P1D")))
			if err != nil {
				t.Fatal(err)
			}
			_, err = b.Add(value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         value.PrimitiveDuration,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              baseID,
				ListItem:          value.NoType,
				Facets:            boundFacet(test.lexical),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := b.Seal(); (err != nil) != test.wantErr {
				t.Fatalf("Seal() error = %v, wantErr=%v", err, test.wantErr)
			}
		})
	}
	_ = derived
}

func TestFacetContractAllowsIncomparablePartialOrderedBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
		lower     string
		upper     string
	}{
		{name: "duration", primitive: value.PrimitiveDuration, lower: "P1M", upper: "P30D"},
		{name: "dateTime", primitive: value.PrimitiveDateTime, lower: "2020-01-01T00:00:00", upper: "2020-01-01T00:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			spec := atomicFacetType(test.primitive, value.FacetSpec{
				MinInclusive: value.BoundFacet{Lexical: test.lower, Type: value.NoType, Present: true},
				MaxInclusive: value.BoundFacet{Lexical: test.upper, Type: value.NoType, Present: true},
			})
			if _, err := builder.Add(spec); err != nil {
				t.Fatal(err)
			}
			if _, err := builder.Seal(); err != nil {
				t.Fatalf("incomparable partial-order bounds rejected: %v", err)
			}
		})
	}
}

func TestFacetContractRejectsEmptyEqualOrderedIntervals(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		lowerBound value.BoundFacet
		upperBound value.BoundFacet
	}{
		{
			name:       "exclusive upper",
			lowerBound: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true},
			upperBound: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true},
		},
		{
			name:       "exclusive lower",
			lowerBound: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true},
			upperBound: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.name == "exclusive upper" {
				test.upperBound = value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true}
			}
			facets := value.FacetSpec{MinInclusive: test.lowerBound, MaxInclusive: test.upperBound}
			if test.name == "exclusive upper" {
				facets.MaxInclusive = value.BoundFacet{}
				facets.MaxExclusive = test.upperBound
			} else {
				facets.MinInclusive = value.BoundFacet{}
				facets.MinExclusive = test.lowerBound
			}
			builder := value.NewBuilder(value.BuilderOptions{})
			if _, err := builder.Add(atomicFacetType(value.PrimitiveDecimal, facets)); err != nil {
				t.Fatal(err)
			}
			if _, err := builder.Seal(); err == nil {
				t.Fatal("empty equal ordered interval was accepted")
			}
		})
	}
}
