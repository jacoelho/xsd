package runtime

import (
	"errors"
	"slices"
)

func validateSimpleValueRouteReadProjectionForTypes(reads []simpleValueRouteRead, types []SimpleType) error {
	if len(reads) != len(types) {
		return errors.New("simple value route projection count does not match types")
	}
	for i := range reads {
		expected := newSimpleValueRouteReadForSimpleType(types[i])
		if reads[i] != expected {
			return errors.New("simple value route projection does not match type")
		}
	}
	return nil
}

func validateSimpleValueColdReadProjectionForTypes(reads simpleValueColdReadTable, types []SimpleType) error {
	if len(reads.index) != len(types) {
		return errors.New("simple value cold projection count does not match types")
	}
	next := uint32(0)
	for i := range types {
		idx := reads.index[i]
		if !simpleValueTypeNeedsColdRead(types[i]) {
			if idx != invalidID {
				return errors.New("simple value cold projection stores unexpected type")
			}
			continue
		}
		if idx != next || !ValidUint32Index(idx, len(reads.values)) {
			return errors.New("simple value cold projection index does not match type")
		}
		read := reads.values[idx]
		if !slices.Equal(read.union, types[i].Union) ||
			!equalFacetSetsForPublication(read.facets, types[i].Facets) ||
			!equalColdEnumerationProjection(read, types[i].Facets.Enumeration) {
			return errors.New("simple value cold projection does not match type")
		}
		next++
	}
	if int(next) != len(reads.values) {
		return errors.New("simple value cold projection value count does not match types")
	}
	return nil
}

func equalColdEnumerationProjection(read simpleValueColdRead, enumeration []CompiledLiteral) bool {
	if len(read.enumeration) != len(enumeration) {
		return false
	}
	for i := range enumeration {
		if !equalSimpleValueFacetLiteralForCompiled(read.enumeration[i], enumeration[i], true) {
			return false
		}
	}
	return true
}

func equalFacetSetsForPublication(a, b FacetSet) bool {
	if a.Length != b.Length || a.MinLength != b.MinLength || a.MaxLength != b.MaxLength ||
		a.TotalDigits != b.TotalDigits || a.FractionDigits != b.FractionDigits ||
		a.Present != b.Present || a.Fixed != b.Fixed ||
		len(a.Enumeration) != len(b.Enumeration) || !EqualStringPatternGroups(a.Patterns, b.Patterns) {
		return false
	}
	for i := range a.bounds {
		if !equalCompiledLiteralForPublication(a.bounds[i], b.bounds[i]) {
			return false
		}
	}
	for i := range a.Enumeration {
		if !equalCompiledLiteralForPublication(&a.Enumeration[i], &b.Enumeration[i]) {
			return false
		}
	}
	return true
}

func equalCompiledLiteralForPublication(a, b *CompiledLiteral) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Lexical == b.Lexical &&
		a.Canonical == b.Canonical &&
		EqualPrimitiveActualValues(a.Actual, a.Canonical, b.Actual, b.Canonical)
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
	for i := range reads {
		if reads[i] != SimpleTypeNeedsQNameResolver(simpleTypes, SimpleTypeID(i)) {
			return false
		}
	}
	return true
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
		equalStringFacetValuesForFacetSet(read.StringFacets, f) &&
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
	if a.xsdSource != b.xsdSource || a.goSource != b.goSource || (a.re == nil) != (b.re == nil) {
		return false
	}
	if a.re != nil && a.re.String() != b.re.String() {
		return false
	}
	return equalSimplePattern(a.fast, b.fast)
}

func equalSimplePattern(a, b *SimplePattern) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.variable != b.variable || len(a.atoms) != len(b.atoms) {
		return false
	}
	for i := range a.atoms {
		aa, ba := a.atoms[i], b.atoms[i]
		if aa.min != ba.min || aa.max != ba.max || aa.class.digit != ba.class.digit ||
			!slices.Equal(aa.class.ranges, ba.class.ranges) {
			return false
		}
	}
	return true
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
	return equalStringFacetValuesForFacetSet(read.StringFacets, f) &&
		read.DecimalFacets == decimalFacetValues(f) &&
		read.LengthFacets == lengthFacetValues(f) &&
		read.Facets == f.Present
}

func equalStringFacetValuesForFacetSet(read StringFacetValues, f FacetSet) bool {
	if read.HasEnumeration != (len(f.Enumeration) != 0) ||
		!EqualStringPatternGroups(read.Patterns, f.Patterns) {
		return false
	}
	if len(read.CanonicalEnumeration) == 0 {
		return true
	}
	if len(read.CanonicalEnumeration) != len(f.Enumeration) {
		return false
	}
	for i := range f.Enumeration {
		if read.CanonicalEnumeration[i] != f.Enumeration[i].Canonical {
			return false
		}
	}
	return true
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
