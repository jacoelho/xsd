package runtime

import (
	"reflect"
	"testing"
)

func TestEqualSimpleValueTypes(t *testing.T) {
	t.Parallel()

	requireProjectionFields[SimpleValueType](t, []string{
		"DecimalMinInclusive",
		"DecimalMaxInclusive",
		"UnionMembers",
		"StringFacets",
		"DecimalFacets",
		"LengthFacets",
		"ListItem",
		"Facets",
		"Variety",
		"Primitive",
		"Builtin",
		"Whitespace",
		"Identity",
		"Fast",
		"RawBypass",
	})

	base := equalTestSimpleValueType()
	if !EqualSimpleValueTypes(base, equalTestSimpleValueType()) {
		t.Fatal("EqualSimpleValueTypes() rejected equal projections")
	}

	tests := []struct {
		name   string
		mutate func(*SimpleValueType)
	}{
		{"decimal min", func(v *SimpleValueType) { v.DecimalMinInclusive.Int = "2" }},
		{"decimal max", func(v *SimpleValueType) { v.DecimalMaxInclusive.Frac = "9" }},
		{"union", func(v *SimpleValueType) { v.UnionMembers[1] = 9 }},
		{"string facets", func(v *SimpleValueType) { v.StringFacets.Patterns[0].Patterns[0] = equalTestPattern("other") }},
		{"decimal facets", func(v *SimpleValueType) { v.DecimalFacets.TotalDigits.Value = 4 }},
		{"length facets", func(v *SimpleValueType) { v.LengthFacets.MinLength.Value = 4 }},
		{"list item", func(v *SimpleValueType) { v.ListItem = 9 }},
		{"facets", func(v *SimpleValueType) { v.Facets = FacetEnumeration }},
		{"variety", func(v *SimpleValueType) { v.Variety = SimpleVarietyAtomic }},
		{"primitive", func(v *SimpleValueType) { v.Primitive = PrimitiveBoolean }},
		{"builtin", func(v *SimpleValueType) { v.Builtin = BuiltinValidationNCName }},
		{"whitespace", func(v *SimpleValueType) { v.Whitespace = WhitespacePreserve }},
		{"identity", func(v *SimpleValueType) { v.Identity = SimpleIdentityNone }},
		{"fast", func(v *SimpleValueType) { v.Fast = SimpleFastNone }},
		{"raw bypass", func(v *SimpleValueType) { v.RawBypass = SimpleValueBypassAcceptString }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := equalTestSimpleValueType()
			test.mutate(&got)
			if EqualSimpleValueTypes(got, base) {
				t.Fatal("EqualSimpleValueTypes() accepted mismatched projections")
			}
		})
	}
}

func TestEqualRawSimpleValueTypes(t *testing.T) {
	t.Parallel()

	requireProjectionFields[RawSimpleValueType](t, []string{
		"DecimalMinInclusive",
		"DecimalMaxInclusive",
		"StringPatterns",
		"ListItem",
		"Facets",
		"Variety",
		"Primitive",
		"Builtin",
		"Whitespace",
		"Identity",
		"Fast",
	})

	base := equalTestRawSimpleValueType()
	if !EqualRawSimpleValueTypes(base, equalTestRawSimpleValueType()) {
		t.Fatal("EqualRawSimpleValueTypes() rejected equal projections")
	}

	tests := []struct {
		name   string
		mutate func(*RawSimpleValueType)
	}{
		{"decimal min", func(v *RawSimpleValueType) { v.DecimalMinInclusive.Negative = true }},
		{"decimal max", func(v *RawSimpleValueType) { v.DecimalMaxInclusive.Int = "7" }},
		{"patterns", func(v *RawSimpleValueType) { v.StringPatterns[0].Patterns[0] = equalTestPattern("other") }},
		{"list item", func(v *RawSimpleValueType) { v.ListItem = 9 }},
		{"facets", func(v *RawSimpleValueType) { v.Facets = FacetEnumeration }},
		{"variety", func(v *RawSimpleValueType) { v.Variety = SimpleVarietyAtomic }},
		{"primitive", func(v *RawSimpleValueType) { v.Primitive = PrimitiveBoolean }},
		{"builtin", func(v *RawSimpleValueType) { v.Builtin = BuiltinValidationNCName }},
		{"whitespace", func(v *RawSimpleValueType) { v.Whitespace = WhitespacePreserve }},
		{"identity", func(v *RawSimpleValueType) { v.Identity = SimpleIdentityNone }},
		{"fast", func(v *RawSimpleValueType) { v.Fast = SimpleFastNone }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := equalTestRawSimpleValueType()
			test.mutate(&got)
			if EqualRawSimpleValueTypes(got, base) {
				t.Fatal("EqualRawSimpleValueTypes() accepted mismatched projections")
			}
		})
	}
}

func TestEqualSimpleValueFacets(t *testing.T) {
	t.Parallel()

	requireProjectionFields[SimpleValueFacets](t, []string{
		"MinInclusive",
		"MaxInclusive",
		"MinExclusive",
		"MaxExclusive",
		"Enumeration",
		"StringFacets",
		"DecimalFacets",
		"LengthFacets",
		"Facets",
	})
	requireProjectionFields[SimpleValueFacetLiteral](t, []string{
		"Lexical",
		"Canonical",
		"Actual",
		"Present",
	})

	base := equalTestSimpleValueFacets()
	if !EqualSimpleValueFacets(base, equalTestSimpleValueFacets()) {
		t.Fatal("EqualSimpleValueFacets() rejected equal projections")
	}

	tests := []struct {
		name   string
		mutate func(*SimpleValueFacets)
	}{
		{"min inclusive", func(v *SimpleValueFacets) { v.MinInclusive.Lexical = "false" }},
		{"max inclusive", func(v *SimpleValueFacets) { v.MaxInclusive.Present = false }},
		{"min exclusive", func(v *SimpleValueFacets) { v.MinExclusive.Canonical = "false" }},
		{"max exclusive actual", func(v *SimpleValueFacets) { v.MaxExclusive.Actual.Boolean = false }},
		{"enumeration", func(v *SimpleValueFacets) { v.Enumeration[0].Actual.Boolean = false }},
		{"string facets", func(v *SimpleValueFacets) { v.StringFacets.HasEnumeration = false }},
		{"decimal facets", func(v *SimpleValueFacets) { v.DecimalFacets.FractionDigits.Value = 4 }},
		{"length facets", func(v *SimpleValueFacets) { v.LengthFacets.MaxLength.Value = 4 }},
		{"facets", func(v *SimpleValueFacets) { v.Facets = FacetPattern }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := equalTestSimpleValueFacets()
			test.mutate(&got)
			if EqualSimpleValueFacets(got, base) {
				t.Fatal("EqualSimpleValueFacets() accepted mismatched projections")
			}
		})
	}
}

func TestEqualStringPatternGroupsComparesFacetProjection(t *testing.T) {
	t.Parallel()

	requireProjectionFields[StringFacetValues](t, []string{
		"Patterns",
		"CanonicalEnumeration",
		"HasEnumeration",
	})
	requireProjectionFields[StringPatternGroup](t, []string{"Patterns"})

	a := []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}}
	b := []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}}
	if !EqualStringPatternGroups(a, b) {
		t.Fatal("EqualStringPatternGroups() rejected equivalent pattern projections")
	}

	b[0].Patterns[0] = equalTestPattern("other")
	if EqualStringPatternGroups(a, b) {
		t.Fatal("EqualStringPatternGroups() accepted mismatched pattern projections")
	}
}

func equalTestSimpleValueType() SimpleValueType {
	return SimpleValueType{
		DecimalMinInclusive: RawDecimalBound{Int: "1", Present: true},
		DecimalMaxInclusive: RawDecimalBound{Int: "9", Frac: "5", Present: true},
		UnionMembers:        []SimpleTypeID{2, 3},
		StringFacets:        equalTestStringFacetValues(),
		DecimalFacets: DecimalFacetValues{
			TotalDigits:    FacetCardinalityValue{Value: 3, Present: true},
			FractionDigits: FacetCardinalityValue{Value: 1, Present: true},
			Facets:         FacetTotalDigits | FacetFractionDigits,
		},
		LengthFacets: LengthFacetValues{
			MinLength: FacetCardinalityValue{Value: 1, Present: true},
			MaxLength: FacetCardinalityValue{Value: 8, Present: true},
		},
		ListItem:   4,
		Facets:     FacetPattern | FacetTotalDigits,
		Variety:    SimpleVarietyList,
		Primitive:  PrimitiveString,
		Builtin:    BuiltinValidationName,
		Whitespace: WhitespaceCollapse,
		Identity:   SimpleIdentityIDREF,
		Fast:       SimpleFastInt,
	}
}

func equalTestRawSimpleValueType() RawSimpleValueType {
	return RawSimpleValueType{
		DecimalMinInclusive: RawDecimalBound{Int: "1", Present: true},
		DecimalMaxInclusive: RawDecimalBound{Int: "9", Present: true},
		StringPatterns:      []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}},
		ListItem:            4,
		Facets:              FacetPattern | FacetTotalDigits,
		Variety:             SimpleVarietyList,
		Primitive:           PrimitiveString,
		Builtin:             BuiltinValidationName,
		Whitespace:          WhitespaceCollapse,
		Identity:            SimpleIdentityIDREF,
		Fast:                SimpleFastInt,
	}
}

func equalTestSimpleValueFacets() SimpleValueFacets {
	return SimpleValueFacets{
		MinInclusive: equalTestLiteral(),
		MaxInclusive: equalTestLiteral(),
		MinExclusive: equalTestLiteral(),
		MaxExclusive: equalTestLiteral(),
		Enumeration:  []SimpleValueFacetLiteral{equalTestLiteral()},
		StringFacets: equalTestStringFacetValues(),
		DecimalFacets: DecimalFacetValues{
			TotalDigits:    FacetCardinalityValue{Value: 3, Present: true},
			FractionDigits: FacetCardinalityValue{Value: 1, Present: true},
			Facets:         FacetTotalDigits | FacetFractionDigits,
		},
		LengthFacets: LengthFacetValues{
			Length:    FacetCardinalityValue{Value: 2, Present: true},
			MaxLength: FacetCardinalityValue{Value: 8, Present: true},
		},
		Facets: FacetEnumeration | FacetTotalDigits,
	}
}

func equalTestStringFacetValues() StringFacetValues {
	return StringFacetValues{
		Patterns:       []StringPatternGroup{{Patterns: []StringPattern{equalTestPattern("ok")}}},
		HasEnumeration: true,
	}
}

func equalTestLiteral() SimpleValueFacetLiteral {
	return SimpleValueFacetLiteral{
		Lexical:   "true",
		Canonical: "true",
		Actual: PrimitiveActualValue{
			Kind:    PrimitiveBoolean,
			Valid:   true,
			Boolean: true,
		},
		Present: true,
	}
}

func equalTestPattern(source string) StringPattern {
	return NewFastStringPattern(source, CompileSimpleStringPattern(source))
}

func requireProjectionFields[T any](t *testing.T, want []string) {
	t.Helper()
	typ := reflect.TypeFor[T]()
	if typ.NumField() != len(want) {
		t.Fatalf("%s field count = %d, want %d", typ.Name(), typ.NumField(), len(want))
	}
	for i, name := range want {
		if got := typ.Field(i).Name; got != name {
			t.Fatalf("%s field %d = %s, want %s", typ.Name(), i, got, name)
		}
	}
}
