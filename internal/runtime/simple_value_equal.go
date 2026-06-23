package runtime

import (
	"errors"
	"slices"
)

// EqualSimpleValueReads reports whether two simple-value read projections
// expose the same validation-facing facts.
func EqualSimpleValueReads(a, b SimpleValueRead) bool {
	if a.Present != b.Present {
		return false
	}
	if !a.Present {
		return true
	}
	return EqualSimpleValueTypes(a.Type, b.Type) &&
		EqualSimpleValueFacets(a.Facets, b.Facets)
}

// EqualSimpleValueReadProjectionTable reports whether reads expose the same
// validation-facing simple values as shapes.
func EqualSimpleValueReadProjectionTable(reads []SimpleValueRead, shapes []SimpleValueReadShape) bool {
	if len(reads) != len(shapes) {
		return false
	}
	for i := range reads {
		if !EqualSimpleValueReads(reads[i], NewSimpleValueRead(shapes[i])) {
			return false
		}
	}
	return true
}

// EqualSimpleValueReadProjectionForTypes reports whether reads expose the same
// validation-facing simple values as frozen simple types.
func EqualSimpleValueReadProjectionForTypes(reads []SimpleValueRead, simpleTypes []SimpleType) bool {
	if len(reads) != len(simpleTypes) {
		return false
	}
	for i := range reads {
		if !EqualSimpleValueReads(reads[i], NewSimpleValueReadForSimpleType(simpleTypes[i])) {
			return false
		}
	}
	return true
}

// ValidateSimpleValueReadProjectionForTypes validates simple-value read
// projections against frozen simple types.
func ValidateSimpleValueReadProjectionForTypes(reads []SimpleValueRead, simpleTypes []SimpleType) error {
	if len(reads) != len(simpleTypes) {
		return errors.New("simple value read projection count does not match types")
	}
	if !EqualSimpleValueReadProjectionForTypes(reads, simpleTypes) {
		return errors.New("simple value read projection does not match type")
	}
	return nil
}

// EqualSimpleValueTypeReadProjectionForTypes reports whether reads expose the
// same hot simple-value type facts as frozen simple types.
func EqualSimpleValueTypeReadProjectionForTypes(reads []SimpleValueTypeRead, simpleTypes []SimpleType) bool {
	if len(reads) != len(simpleTypes) {
		return false
	}
	for i := range reads {
		st := simpleTypes[i]
		if reads[i].Present == st.Missing {
			return false
		}
		if reads[i].Present && !EqualSimpleValueTypeForSimpleType(reads[i].Type, st) {
			return false
		}
	}
	return true
}

// ValidateSimpleValueTypeReadProjectionForTypes validates hot simple-value type
// projections against frozen simple types.
func ValidateSimpleValueTypeReadProjectionForTypes(reads []SimpleValueTypeRead, simpleTypes []SimpleType) error {
	if len(reads) != len(simpleTypes) {
		return errors.New("simple value type read projection count does not match types")
	}
	if !EqualSimpleValueTypeReadProjectionForTypes(reads, simpleTypes) {
		return errors.New("simple value type read projection does not match type")
	}
	return nil
}

// EqualSimpleValueFacetReadProjectionForTypes reports whether full facet reads
// expose the same cold simple-value facet facts as frozen simple types.
func EqualSimpleValueFacetReadProjectionForTypes(reads SimpleValueFacetReadTable, simpleTypes []SimpleType) bool {
	if len(reads.Index) != len(simpleTypes) {
		return false
	}
	next := uint32(0)
	for i, st := range simpleTypes {
		idx := reads.Index[i]
		if st.Missing || st.Facets.Present == 0 {
			if idx != invalidID {
				return false
			}
			continue
		}
		if idx != next || !ValidUint32Index(idx, len(reads.Values)) {
			return false
		}
		if !EqualSimpleValueFacetsForFacetSet(reads.Values[idx], st.Facets) {
			return false
		}
		next++
	}
	return int(next) == len(reads.Values)
}

// ValidateSimpleValueFacetReadProjectionForTypes validates full facet reads
// against frozen simple types.
func ValidateSimpleValueFacetReadProjectionForTypes(reads SimpleValueFacetReadTable, simpleTypes []SimpleType) error {
	if len(reads.Index) != len(simpleTypes) {
		return errors.New("simple value facet read projection count does not match types")
	}
	if !EqualSimpleValueFacetReadProjectionForTypes(reads, simpleTypes) {
		return errors.New("simple value facet read projection does not match type")
	}
	return nil
}

// EqualSimpleValueQNameResolverNeedsForSimpleTypes reports whether reads expose
// the QName/NOTATION namespace-resolution needs for frozen simple types.
func EqualSimpleValueQNameResolverNeedsForSimpleTypes(reads []bool, simpleTypes []SimpleType) bool {
	if len(reads) != len(simpleTypes) {
		return false
	}
	expected := NewSimpleValueQNameResolverNeedsForSimpleTypes(simpleTypes)
	return slices.Equal(reads, expected)
}

// ValidateSimpleValueQNameResolverNeedsForSimpleTypes validates QName resolver
// need projections against frozen simple types.
func ValidateSimpleValueQNameResolverNeedsForSimpleTypes(reads []bool, simpleTypes []SimpleType) error {
	if len(reads) != len(simpleTypes) {
		return errors.New("simple value QName resolver projection count does not match types")
	}
	if !EqualSimpleValueQNameResolverNeedsForSimpleTypes(reads, simpleTypes) {
		return errors.New("simple value QName resolver projection does not match type")
	}
	return nil
}

// EqualSimpleValueTypes reports whether two full simple-value read projections
// expose the same validation-facing type facts.
func EqualSimpleValueTypes(a, b SimpleValueType) bool {
	return a.DecimalMinInclusive == b.DecimalMinInclusive &&
		a.DecimalMaxInclusive == b.DecimalMaxInclusive &&
		slices.Equal(a.UnionMembers, b.UnionMembers) &&
		EqualStringFacetValues(a.StringFacets, b.StringFacets) &&
		a.DecimalFacets == b.DecimalFacets &&
		a.LengthFacets == b.LengthFacets &&
		a.ListItem == b.ListItem &&
		a.Facets == b.Facets &&
		a.Variety == b.Variety &&
		a.Primitive == b.Primitive &&
		a.Builtin == b.Builtin &&
		a.Whitespace == b.Whitespace &&
		a.Identity == b.Identity &&
		a.Fast == b.Fast &&
		a.RawBypass == b.RawBypass
}

// EqualSimpleValueTypeForSimpleType reports whether read exposes the same type
// facts as st without constructing another read projection.
func EqualSimpleValueTypeForSimpleType(read SimpleValueType, st SimpleType) bool {
	f := st.Facets
	return read.DecimalMinInclusive == rawDecimalBoundFacet(f, FacetMinInclusive) &&
		read.DecimalMaxInclusive == rawDecimalBoundFacet(f, FacetMaxInclusive) &&
		slices.Equal(read.UnionMembers, st.Union) &&
		EqualStringFacetValues(read.StringFacets, stringFacetValues(f)) &&
		read.DecimalFacets == decimalFacetValues(f) &&
		read.LengthFacets == lengthFacetValues(f) &&
		read.ListItem == st.ListItem &&
		read.Facets == f.Present &&
		read.Variety == st.Variety &&
		read.Primitive == st.Primitive &&
		read.Builtin == st.Builtin &&
		read.Whitespace == st.Whitespace &&
		read.Identity == st.Identity &&
		read.Fast == st.Fast &&
		read.RawBypass == SimpleValueBypass(simpleValueAtomicBypassShape(&read, 0))
}

// EqualRawSimpleValueTypes reports whether two raw simple-value read
// projections expose the same raw-fast-path type facts.
func EqualRawSimpleValueTypes(a, b RawSimpleValueType) bool {
	return a.DecimalMinInclusive == b.DecimalMinInclusive &&
		a.DecimalMaxInclusive == b.DecimalMaxInclusive &&
		EqualStringPatternGroups(a.StringPatterns, b.StringPatterns) &&
		a.ListItem == b.ListItem &&
		a.Facets == b.Facets &&
		a.Variety == b.Variety &&
		a.Primitive == b.Primitive &&
		a.Builtin == b.Builtin &&
		a.Whitespace == b.Whitespace &&
		a.Identity == b.Identity &&
		a.Fast == b.Fast
}

// EqualStringFacetValues reports whether two string facet projections expose
// the same pattern and canonical enumeration facts.
func EqualStringFacetValues(a, b StringFacetValues) bool {
	return a.HasEnumeration == b.HasEnumeration &&
		slices.Equal(a.CanonicalEnumeration, b.CanonicalEnumeration) &&
		EqualStringPatternGroups(a.Patterns, b.Patterns)
}

// EqualStringPatternGroups reports whether two compiled string pattern group
// projections expose the same freeze-comparable pattern facets.
func EqualStringPatternGroups(a, b []StringPatternGroup) bool {
	return slices.EqualFunc(a, b, equalStringPatternGroup)
}

func equalStringPatternGroup(a, b StringPatternGroup) bool {
	return slices.EqualFunc(a.Patterns, b.Patterns, equalStringPattern)
}

func equalStringPattern(a, b StringPattern) bool {
	return a.FacetProjection() == b.FacetProjection()
}

// EqualSimpleValueFacets reports whether two simple-value facet read
// projections expose the same validation-facing facet facts.
func EqualSimpleValueFacets(a, b SimpleValueFacets) bool {
	return equalSimpleValueFacetLiteral(a.MinInclusive, b.MinInclusive) &&
		equalSimpleValueFacetLiteral(a.MaxInclusive, b.MaxInclusive) &&
		equalSimpleValueFacetLiteral(a.MinExclusive, b.MinExclusive) &&
		equalSimpleValueFacetLiteral(a.MaxExclusive, b.MaxExclusive) &&
		slices.EqualFunc(a.Enumeration, b.Enumeration, equalSimpleValueFacetLiteral) &&
		EqualStringFacetValues(a.StringFacets, b.StringFacets) &&
		a.DecimalFacets == b.DecimalFacets &&
		a.LengthFacets == b.LengthFacets &&
		a.Facets == b.Facets
}

// EqualSimpleValueFacetsForFacetSet reports whether read exposes the same
// facet facts as f without constructing another read projection.
func EqualSimpleValueFacetsForFacetSet(read SimpleValueFacets, f FacetSet) bool {
	if !equalSimpleValueFacetLiteralForBound(read.MinInclusive, f, FacetMinInclusive) ||
		!equalSimpleValueFacetLiteralForBound(read.MaxInclusive, f, FacetMaxInclusive) ||
		!equalSimpleValueFacetLiteralForBound(read.MinExclusive, f, FacetMinExclusive) ||
		!equalSimpleValueFacetLiteralForBound(read.MaxExclusive, f, FacetMaxExclusive) {
		return false
	}
	if len(read.Enumeration) != len(f.Enumeration) {
		return false
	}
	for i := range f.Enumeration {
		if !equalSimpleValueFacetLiteralForCompiled(read.Enumeration[i], f.Enumeration[i], true) {
			return false
		}
	}
	return EqualStringFacetValues(read.StringFacets, StringFacetValues{
		Patterns:             f.Patterns,
		CanonicalEnumeration: canonicalEnumerationValues(f.Enumeration),
		HasEnumeration:       len(f.Enumeration) != 0,
	}) &&
		read.DecimalFacets == decimalFacetValues(f) &&
		read.LengthFacets == lengthFacetValues(f) &&
		read.Facets == f.Present
}

func equalSimpleValueFacetLiteral(a, b SimpleValueFacetLiteral) bool {
	return a.Present == b.Present &&
		a.Lexical == b.Lexical &&
		a.Canonical == b.Canonical &&
		EqualPrimitiveActualValues(a.Actual, a.Canonical, b.Actual, b.Canonical)
}

func equalSimpleValueFacetLiteralForBound(got SimpleValueFacetLiteral, f FacetSet, flag FacetMask) bool {
	lit, present := BoundFacet(f, flag)
	return equalSimpleValueFacetLiteralForCompiled(got, lit, present)
}

func equalSimpleValueFacetLiteralForCompiled(got SimpleValueFacetLiteral, want CompiledLiteral, present bool) bool {
	if !present {
		return got == SimpleValueFacetLiteral{}
	}
	return got.Present &&
		got.Lexical == want.Lexical &&
		got.Canonical == want.Canonical &&
		EqualPrimitiveActualValues(got.Actual, got.Canonical, want.Actual, want.Canonical)
}
