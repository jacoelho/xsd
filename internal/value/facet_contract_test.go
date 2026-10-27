package value_test

import (
	"errors"
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
	baseID, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(baseID, base)
	if err != nil {
		t.Fatalf("add base type: %v", err)
	}
	derived.Base = baseID
	reservedID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatalf("Reserve() error = %v", reserveErr)
	}
	if err := builder.Complete(reservedID, derived); err == nil {
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
	listID, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(listID, value.TypeSpec{
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
	unionID, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(unionID, value.TypeSpec{
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
		name    string
		wantErr string
		spec    value.TypeSpec
	}{
		{name: "decimal length", wantErr: "length facets require a length-capable type", spec: atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}})},
		{name: "string bound", wantErr: "ordered facets require an ordered atomic type", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{MinInclusive: value.BoundFacet{Lexical: "x", Type: value.NoType, Present: true}})},
		{name: "string digits", wantErr: "digit facets require atomic decimal type", spec: atomicFacetType(value.PrimitiveString, value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 1, Present: true}})},
		{name: "list bound", wantErr: "ordered facets require an ordered atomic type", spec: value.TypeSpec{Variety: value.List, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveString), Facets: value.FacetSpec{MinInclusive: value.BoundFacet{Lexical: "1", Type: value.NoType, Present: true}}}},
		{name: "list digits", wantErr: "digit facets require atomic decimal type", spec: value.TypeSpec{Variety: value.List, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveString), Facets: value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 1, Present: true}}}},
		{name: "union length", wantErr: "length facets are not valid on union types", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 1, Present: true}}}},
		{name: "union whitespace", wantErr: "facet is not allowed for simple type", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Present: value.FacetWhiteSpace}}},
		{name: "union digits", wantErr: "digit facets require atomic decimal type", spec: value.TypeSpec{Variety: value.Union, Primitive: value.PrimitiveString, Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{value.BuiltinType(value.PrimitiveString)}, Facets: value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}, TotalDigits: value.CardinalityFacet{Value: 1, Present: true}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			reservedID, reserveErr := builder.Reserve()
			if reserveErr != nil {
				t.Fatalf("Reserve() error = %v", reserveErr)
			}
			completeErr := builder.Complete(reservedID, test.spec)
			if completeErr == nil {
				t.Error("forbidden facet family was accepted")
				return
			}
			if !errors.Is(completeErr, value.ErrFacet) || completeErr.Error() != test.wantErr {
				t.Fatalf("Complete() = %v; want ErrFacet with %q", completeErr, test.wantErr)
			}
		})
	}
}

func TestBooleanEnumerationRejectionAndCompletionRetry(t *testing.T) {
	t.Parallel()
	pattern, err := xsdregex.Compile("true|1", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec := atomicFacetType(value.PrimitiveBoolean, value.FacetSpec{
		Enumeration: []value.LiteralSpec{{Lexical: "true", Type: value.NoType}},
	})
	spec.Base = value.BuiltinType(value.PrimitiveBoolean)
	err = builder.Complete(id, spec)
	if !errors.Is(err, value.ErrFacet) {
		t.Fatalf("Complete(boolean enumeration) = %v; want applicability rejection", err)
	}
	if _, available := builder.InputRequirements(id); available {
		t.Fatal("failed completion exposed Boolean type metadata")
	}
	spec.Facets = value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}}
	if err = builder.Complete(id, spec); err != nil {
		t.Fatalf("Complete(boolean pattern) retry = %v", err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, lexical := range []string{"true", "1", " true "} {
		if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); err != nil {
			t.Errorf("Validate(%q) = %v; want admitted Boolean pattern", lexical, err)
		}
	}
	for _, lexical := range []string{"false", "0"} {
		if _, validationErr := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); !errors.Is(validationErr, value.ErrFacet) {
			t.Errorf("Validate(%q) = %v; want Boolean pattern rejection", lexical, validationErr)
		}
	}
}

func TestBooleanListAndUnionEnumeration(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		variety value.Variety
		literal string
		valid   []string
		invalid []string
	}{
		{name: "list", variety: value.List, literal: "true false", valid: []string{"true false", "1 0"}, invalid: []string{"false true", "true"}},
		{name: "union", variety: value.Union, literal: "true", valid: []string{"true", "1"}, invalid: []string{"false", "0"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := value.TypeSpec{
				Variety: test.variety, Primitive: value.PrimitiveBoolean,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.NoType, ListItem: value.NoType,
				Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: test.literal, Type: value.NoType}}},
			}
			if test.variety == value.List {
				spec.ListItem = value.BuiltinType(value.PrimitiveBoolean)
			} else {
				spec.Union = []value.TypeID{value.BuiltinType(value.PrimitiveBoolean)}
			}
			if err = builder.Complete(id, spec); err != nil {
				t.Fatalf("Complete(%s Boolean enumeration) = %v", test.name, err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, lexical := range test.valid {
				if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); err != nil {
					t.Errorf("Validate(%q) = %v; want value-equivalent enumerand", lexical, err)
				}
			}
			for _, lexical := range test.invalid {
				if _, validationErr := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); !errors.Is(validationErr, value.ErrFacet) {
					t.Errorf("Validate(%q) = %v; want enumeration rejection", lexical, validationErr)
				}
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
	base, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(base, atomicFacetType(value.PrimitiveString, value.FacetSpec{
		Patterns: [][]*value.Pattern{{basePattern}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(derived, value.TypeSpec{
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
	list, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(list, value.TypeSpec{
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
	union, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(union, value.TypeSpec{
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
	base, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(base, baseSpec)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(derived, value.TypeSpec{
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
			baseID, err := b.Reserve()
			if err != nil {
				t.Fatalf("Reserve() error = %v", err)
			}
			err = b.Complete(baseID, atomicFacetType(value.PrimitiveDuration, boundFacet("P1D")))
			if err != nil {
				t.Fatal(err)
			}
			reservedID, reserveErr := b.Reserve()
			if reserveErr != nil {
				t.Fatalf("Reserve() error = %v", reserveErr)
			}
			err = b.Complete(reservedID, value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         value.PrimitiveDuration,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              baseID,
				ListItem:          value.NoType,
				Facets:            boundFacet(test.lexical),
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("Complete() error = %v, wantErr=%v", err, test.wantErr)
			}
			if err == nil {
				if _, sealErr := b.Seal(); sealErr != nil {
					t.Fatalf("Seal() after successful completion = %v", sealErr)
				}
			}
		})
	}
	_ = derived
}

func TestFacetContractAllowsIncomparableOrderedBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
		lower     string
		upper     string
		rejected  []string
	}{
		{name: "duration", primitive: value.PrimitiveDuration, lower: "P1M", upper: "P30D"},
		{name: "dateTime", primitive: value.PrimitiveDateTime, lower: "2020-01-01T00:00:00", upper: "2020-01-01T00:00:00Z"},
		{name: "float NaN lower", primitive: value.PrimitiveFloat, lower: "NaN", upper: "0", rejected: []string{"NaN", "0"}},
		{name: "float NaN upper", primitive: value.PrimitiveFloat, lower: "0", upper: "NaN", rejected: []string{"NaN", "0"}},
		{name: "double NaN lower", primitive: value.PrimitiveDouble, lower: "NaN", upper: "0", rejected: []string{"NaN", "0"}},
		{name: "double NaN upper", primitive: value.PrimitiveDouble, lower: "0", upper: "NaN", rejected: []string{"NaN", "0"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			lower := value.BoundFacet{Lexical: test.lower, Type: value.NoType, Present: true}
			upper := value.BoundFacet{Lexical: test.upper, Type: value.NoType, Present: true}
			for _, bounds := range []struct {
				name   string
				facets value.FacetSpec
			}{
				{"both inclusive", value.FacetSpec{MinInclusive: lower, MaxInclusive: upper}},
				{"inclusive exclusive", value.FacetSpec{MinInclusive: lower, MaxExclusive: upper}},
				{"exclusive inclusive", value.FacetSpec{MinExclusive: lower, MaxInclusive: upper}},
				{"both exclusive", value.FacetSpec{MinExclusive: lower, MaxExclusive: upper}},
			} {
				t.Run(bounds.name, func(t *testing.T) {
					builder := value.NewBuilder(value.BuilderOptions{})
					id, err := builder.Reserve()
					if err != nil {
						t.Fatal(err)
					}
					if completeErr := builder.Complete(id, atomicFacetType(test.primitive, bounds.facets)); completeErr != nil {
						t.Fatalf("Complete() rejected incomparable bounds: %v", completeErr)
					}
					program, err := builder.Seal()
					if err != nil {
						t.Fatal(err)
					}
					assertOrderedFacetValues(t, program, id, nil, test.rejected)
				})
			}
		})
	}
}

func TestFacetContractEqualOrderedBounds(t *testing.T) {
	t.Parallel()

	lower := value.BoundFacet{Lexical: "10", Type: value.NoType, Present: true}
	upper := value.BoundFacet{Lexical: "10.0", Type: value.NoType, Present: true}
	for _, test := range []struct {
		name     string
		facets   value.FacetSpec
		wantErr  bool
		accepted []string
		rejected []string
	}{
		{
			name: "both inclusive", facets: value.FacetSpec{MinInclusive: lower, MaxInclusive: upper},
			accepted: []string{"10", "10.0"}, rejected: []string{"9", "11"},
		},
		{
			name: "both exclusive", facets: value.FacetSpec{MinExclusive: lower, MaxExclusive: upper},
			rejected: []string{"9", "10", "10.0", "11"},
		},
		{name: "inclusive exclusive", facets: value.FacetSpec{MinInclusive: lower, MaxExclusive: upper}, wantErr: true},
		{name: "exclusive inclusive", facets: value.FacetSpec{MinExclusive: lower, MaxInclusive: upper}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			err = builder.Complete(id, atomicFacetType(value.PrimitiveDecimal, test.facets))
			if test.wantErr {
				if !errors.Is(err, value.ErrFacet) {
					t.Fatalf("Complete() = %v; want ErrFacet", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete() rejected consistent equal bounds: %v", err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			assertOrderedFacetValues(t, program, id, test.accepted, test.rejected)
		})
	}
}

func TestFacetContractEqualNaNBounds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
	}{
		{"float", value.PrimitiveFloat},
		{"double", value.PrimitiveDouble},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			bound := value.BoundFacet{Lexical: "NaN", Type: value.NoType, Present: true}
			if completeErr := builder.Complete(id, atomicFacetType(test.primitive, value.FacetSpec{
				MinInclusive: bound, MaxInclusive: bound,
			})); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			assertOrderedFacetValues(t, program, id, []string{"NaN"}, []string{"0"})
		})
	}
}

func assertOrderedFacetValues(t *testing.T, program *value.Program, id value.TypeID, accepted, rejected []string) {
	t.Helper()
	for _, form := range []string{"typed", "bytes"} {
		validate := func(input string) error {
			if form == "bytes" {
				_, err := program.ValidateBytes(id, []byte(input), value.Resolver{}, 0, 16<<20, nil)
				return err
			}
			_, err := program.Validate(id, input, value.Resolver{}, value.NeedCanonical, 16<<20, nil)
			return err
		}
		for _, input := range accepted {
			if err := validate(input); err != nil {
				t.Errorf("%s input=%q: %v; want acceptance", form, input, err)
			}
		}
		for _, input := range rejected {
			if err := validate(input); !errors.Is(err, value.ErrFacet) {
				t.Errorf("%s input=%q: %v; want ErrFacet", form, input, err)
			}
		}
	}
}

func TestCollectionFacetsCannotBeFixedAndCompletionCanRetry(t *testing.T) {
	t.Parallel()
	pattern, err := xsdregex.Compile("x", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		facets  value.FacetSpec
		message string
	}{
		{
			name:    "pattern",
			facets:  value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}, Fixed: value.FacetPattern},
			message: `value "y" does not satisfy pattern groups ["x"]`,
		},
		{
			name:    "enumeration",
			facets:  value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: "x", Type: value.NoType}}, Fixed: value.FacetEnumeration},
			message: `value "y" is not one of ["x"]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(value.PrimitiveString, test.facets)
			err = builder.Complete(id, spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != "pattern and enumeration facets cannot be fixed" {
				t.Fatalf("Complete() = %v; want fixed collection facet rejection", err)
			}
			if _, available := builder.InputRequirements(id); available {
				t.Fatal("failed completion exposed type metadata")
			}
			spec.Facets.Fixed = 0
			if completionErr := builder.Complete(id, spec); completionErr != nil {
				t.Fatalf("Complete() retry = %v", completionErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			if _, validationErr := program.Validate(id, "x", value.Resolver{}, 0, 16<<20, nil); validationErr != nil {
				t.Fatalf("Validate(x) = %v", validationErr)
			}
			_, err = program.Validate(id, "y", value.Resolver{}, 0, 16<<20, nil)
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.message {
				t.Fatalf("Validate(y) = %v; want %q", err, test.message)
			}
		})
	}
}

func TestFixedWhitespaceRejectsStrengtheningAndCompletionCanRetry(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	baseID, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	base := atomicFacetType(value.PrimitiveString, value.FacetSpec{Fixed: value.FacetWhiteSpace})
	base.Whitespace = value.WhitespacePreserve
	if completionErr := builder.Complete(baseID, base); completionErr != nil {
		t.Fatal(completionErr)
	}
	derivedID, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	derived := atomicFacetType(value.PrimitiveString, value.FacetSpec{})
	derived.Base = baseID
	err = builder.Complete(derivedID, derived)
	if !errors.Is(err, value.ErrFacet) || err.Error() != "fixed whiteSpace facet cannot change" {
		t.Fatalf("Complete() = %v; want fixed whiteSpace rejection", err)
	}
	if _, available := builder.InputRequirements(derivedID); available {
		t.Fatal("failed completion exposed type metadata")
	}
	derived.Whitespace = value.WhitespacePreserve
	if completionErr := builder.Complete(derivedID, derived); completionErr != nil {
		t.Fatalf("Complete() retry = %v", completionErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	const lexical = " \ta\n "
	result, err := program.Validate(derivedID, lexical, value.Resolver{}, value.NeedCanonical, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalText() != lexical || result.Type() != derivedID || result.SelectedType() != value.NoType {
		t.Fatalf("Validate() = canonical %q, type %d, selection %d; want %q, %d, NoType", result.CanonicalText(), result.Type(), result.SelectedType(), lexical, derivedID)
	}
}

func TestInheritedCardinalityConflictsAndRetry(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		primitive value.PrimitiveKind
		base      value.FacetSpec
		own       value.FacetSpec
		wantError string
		retry     bool
	}{
		{
			name:      "inherited length below local minimum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{Length: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{MinLength: value.CardinalityFacet{Value: 4, Present: true}},
			wantError: "length cannot be less than minLength",
		},
		{
			name:      "local length above inherited maximum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 2, Present: true}},
			own:       value.FacetSpec{Length: value.CardinalityFacet{Value: 3, Present: true}},
			wantError: "length cannot exceed maxLength",
		},
		{
			name:      "inherited minimum above local maximum",
			primitive: value.PrimitiveString,
			base:      value.FacetSpec{MinLength: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 2, Present: true}},
			wantError: "minLength cannot exceed maxLength",
			retry:     true,
		},
		{
			name:      "local fraction digits above inherited total",
			primitive: value.PrimitiveDecimal,
			base:      value.FacetSpec{TotalDigits: value.CardinalityFacet{Value: 3, Present: true}},
			own:       value.FacetSpec{FractionDigits: value.CardinalityFacet{Value: 4, Present: true}},
			wantError: "fractionDigits cannot exceed totalDigits",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			builder := value.NewBuilder(value.BuilderOptions{})
			baseID, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completionErr := builder.Complete(baseID, atomicFacetType(test.primitive, test.base)); completionErr != nil {
				t.Fatal(completionErr)
			}
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(test.primitive, test.own)
			spec.Base = baseID
			err = builder.Complete(id, spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.wantError {
				t.Fatalf("Complete() = %v; want ErrFacet with %q", err, test.wantError)
			}
			if _, available := builder.InputRequirements(id); available {
				t.Fatal("failed type is available")
			}
			if !test.retry {
				return
			}

			spec.Facets = value.FacetSpec{
				MinLength: value.CardinalityFacet{Value: 999, Present: false},
				MaxLength: value.CardinalityFacet{Value: 5, Present: true},
			}
			if retryErr := builder.Complete(id, spec); retryErr != nil {
				t.Fatalf("Complete() retry = %v", retryErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, lexical := range []string{"abc", "abcde"} {
				if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); err != nil {
					t.Fatalf("Validate(%q) = %v", lexical, err)
				}
			}
			for _, lexical := range []string{"ab", "abcdef"} {
				if _, err := program.Validate(id, lexical, value.Resolver{}, 0, 16<<20, nil); !errors.Is(err, value.ErrFacet) {
					t.Fatalf("Validate(%q) = %v; want ErrFacet", lexical, err)
				}
			}
		})
	}
}

func TestProgramLexicalPayloadLengthFacets(t *testing.T) {
	for _, tc := range []struct {
		name      string
		primitive value.PrimitiveKind
		length    uint32
		accepted  string
		canonical string
		rejected  []string
	}{
		{"URI characters", value.PrimitiveAnyURI, 2, "éx", "éx", []string{"é", "éxy"}},
		{"base64 octets", value.PrimitiveBase64Binary, 1, " Y Q == ", "YQ==", []string{"", "YWI="}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, reserveErr := builder.Reserve()
			if reserveErr != nil {
				t.Fatal(reserveErr)
			}
			if completeErr := builder.Complete(id, value.TypeSpec{
				Variety:           value.Atomic,
				Primitive:         tc.primitive,
				Whitespace:        value.WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              value.NoType,
				ListItem:          value.NoType,
				Facets: value.FacetSpec{
					Length: value.CardinalityFacet{Value: tc.length, Present: true},
				},
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, sealErr := builder.Seal()
			if sealErr != nil {
				t.Fatal(sealErr)
			}
			for _, form := range []string{"string", "bytes"} {
				for _, needs := range []value.Needs{0, value.NeedCanonical} {
					validate := func(input string) (value.Value, error) {
						if form == "bytes" {
							return program.ValidateBytes(id, []byte(input), value.Resolver{}, needs, 16<<20, nil)
						}
						return program.Validate(id, input, value.Resolver{}, needs, 16<<20, nil)
					}
					accepted, validationErr := validate(tc.accepted)
					if validationErr != nil {
						t.Fatalf("%s needs=%d accepted input: %v", form, needs, validationErr)
					}
					wantCanonical := ""
					if needs == value.NeedCanonical {
						wantCanonical = tc.canonical
					}
					if accepted.CanonicalText() != wantCanonical {
						t.Fatalf("%s needs=%d canonical = %q, want %q", form, needs, accepted.CanonicalText(), wantCanonical)
					}
					for _, input := range tc.rejected {
						if _, rejectedErr := validate(input); !errors.Is(rejectedErr, value.ErrFacet) {
							t.Fatalf("%s needs=%d input=%q error = %v, want ErrFacet", form, needs, input, rejectedErr)
						}
					}
				}
			}
		})
	}
}
