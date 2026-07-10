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

func equalSimpleValueFacetLiteralForCompiled(got SimpleValueFacetLiteral, want CompiledLiteral, present bool) bool {
	if !present {
		return got == SimpleValueFacetLiteral{}
	}
	return got.Present &&
		got.Lexical == want.Lexical &&
		got.Canonical == want.Canonical &&
		EqualPrimitiveActualValues(got.Actual, got.Canonical, want.Actual, want.Canonical)
}
