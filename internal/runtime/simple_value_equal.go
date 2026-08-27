package runtime

import (
	"errors"
	"slices"
)

func validateSimpleValueRouteReadProjectionForTypes(reads []simpleValueRouteRead, types []SimpleType) error {
	if len(reads) != len(types) {
		return errors.New("simple value route projection count does not match types")
	}
	expected := newSimpleValueRouteReadsForSimpleTypes(types)
	for i := range reads {
		if reads[i] != expected[i] {
			return errors.New("simple value route projection does not match type")
		}
		if reads[i].availability == simpleTypeAvailabilityInvalid {
			return errors.New("simple value route projection has invalid availability")
		}
	}
	return nil
}

func validateSimpleTypeColdReadProjectionForTypes(reads *simpleTypeColdReadTable, types []SimpleType) error {
	if reads == nil {
		return errors.New("simple type cold projection is missing")
	}
	if len(reads.index) != len(types) {
		return errors.New("simple value cold projection count does not match types")
	}
	boundIndexes := simpleValueBoundReadIndexes(types)
	if err := validateSimpleValueBoundReadPool(reads.boundReads, boundIndexes); err != nil {
		return err
	}
	var enumerationPool simpleValueEnumerationPoolAudit
	var patternPool simpleValuePatternPoolAudit
	next := uint32(0)
	for i := range types {
		if err := validateSimpleTypeColdReadAt(reads, types[i], i, boundIndexes, &enumerationPool, &patternPool, &next); err != nil {
			return err
		}
	}
	if int(next) != len(reads.values) {
		return errors.New("simple value cold projection value count does not match types")
	}
	return nil
}

func validateSimpleValueBoundReadPool(reads []simpleValueLiteralRead, indexes map[*CompiledLiteral]uint32) error {
	if len(reads) != len(indexes) {
		return errors.New("simple value bound read pool does not match types")
	}
	for source, index := range indexes {
		if !ValidUint32Index(index, len(reads)) || !equalSimpleValueLiteralReadForCompiled(reads[index], *source) {
			return errors.New("simple value bound read pool does not match types")
		}
	}
	return nil
}

func validateSimpleTypeColdReadAt(reads *simpleTypeColdReadTable, typ SimpleType, typeIndex int, boundIndexes map[*CompiledLiteral]uint32, enumerationPool *simpleValueEnumerationPoolAudit, patternPool *simpleValuePatternPoolAudit, next *uint32) error {
	idx := reads.index[typeIndex]
	if !simpleTypeNeedsColdRead(typ) {
		if idx != invalidID {
			return errors.New("simple value cold projection stores unexpected type")
		}
		return nil
	}
	if idx != *next || !ValidUint32Index(idx, len(reads.values)) {
		return errors.New("simple value cold projection index does not match type")
	}
	read := reads.values[idx]
	if !simpleTypeColdReadMatches(read, typ, reads.boundReads, boundIndexes, enumerationPool, patternPool) {
		return errors.New("simple value cold projection does not match type")
	}
	*next++
	return nil
}

func simpleTypeColdReadMatches(read simpleValueColdRead, typ SimpleType, boundReads []simpleValueLiteralRead, boundIndexes map[*CompiledLiteral]uint32, enumerationPool *simpleValueEnumerationPoolAudit, patternPool *simpleValuePatternPoolAudit) bool {
	return slices.Equal(read.union, typ.Union) &&
		equalColdFacetProjection(read.facets, typ.Facets) &&
		equalColdBoundPoolProjection(read.facets, typ.Facets, boundReads, boundIndexes) &&
		enumerationPool.matches(typ.Facets.Enumeration, read.enumeration) &&
		patternPool.matches(typ.Facets.patterns, read.facets.patterns)
}

type simpleValuePatternPoolAudit struct {
	reads   map[*stringPatternStep]*stringPatternStepRead
	sources map[*stringPatternStepRead]*stringPatternStep
}

func (a *simpleValuePatternPoolAudit) matches(source stringPatternSteps, read *stringPatternStepRead) bool {
	for sourceStep := source.tail; sourceStep != nil; sourceStep = sourceStep.parent {
		switch a.matchStep(sourceStep, read) {
		case patternPoolStepInvalid:
			return false
		case patternPoolStepKnown:
			return true
		case patternPoolStepNew:
			read = read.parent
		}
	}
	return read == nil
}

type patternPoolStepMatch uint8

const (
	patternPoolStepInvalid patternPoolStepMatch = iota
	patternPoolStepNew
	patternPoolStepKnown
)

func (a *simpleValuePatternPoolAudit) matchStep(source *stringPatternStep, read *stringPatternStepRead) patternPoolStepMatch {
	if read == nil {
		return patternPoolStepInvalid
	}
	if existing, ok := a.reads[source]; ok {
		if existing == read && a.sources[read] == source {
			return patternPoolStepKnown
		}
		return patternPoolStepInvalid
	}
	if read.count != source.count || !equalStringPatternStepReadForSource(read, source) {
		return patternPoolStepInvalid
	}
	if existing, ok := a.sources[read]; ok && existing != source {
		return patternPoolStepInvalid
	}
	if a.reads == nil {
		a.reads = make(map[*stringPatternStep]*stringPatternStepRead)
		a.sources = make(map[*stringPatternStepRead]*stringPatternStep)
	}
	a.reads[source] = read
	a.sources[read] = source
	return patternPoolStepNew
}

type simpleValueEnumerationPoolAudit struct {
	reads   map[simpleValueEnumerationSource]simpleValueEnumerationRead
	sources map[simpleValueEnumerationRead]simpleValueEnumerationSource
}

func (a *simpleValueEnumerationPoolAudit) matches(source []CompiledLiteral, read []simpleValueLiteralRead) bool {
	sourceID, sourcePresent := simpleValueEnumerationSourceForLiterals(source)
	readID, readPresent := simpleValueEnumerationReadForLiterals(read)
	if sourcePresent != readPresent {
		return false
	}
	if !sourcePresent {
		return true
	}
	if existing, ok := a.reads[sourceID]; ok {
		return existing == readID && a.sources[readID] == sourceID
	}
	if !equalSimpleValueEnumerationReadForSource(read, source) {
		return false
	}
	if _, ok := a.sources[readID]; ok {
		return false
	}
	if a.reads == nil {
		a.reads = make(map[simpleValueEnumerationSource]simpleValueEnumerationRead)
		a.sources = make(map[simpleValueEnumerationRead]simpleValueEnumerationSource)
	}
	a.reads[sourceID] = readID
	a.sources[readID] = sourceID
	return true
}

func equalColdBoundPoolProjection(read simpleValueFacetRead, facets FacetSet, pool []simpleValueLiteralRead, indexes map[*CompiledLiteral]uint32) bool {
	for i, source := range facets.bounds {
		if source == nil {
			if read.bounds[i] != nil {
				return false
			}
			continue
		}
		index, ok := indexes[source]
		if !ok || !ValidUint32Index(index, len(pool)) || read.bounds[i] != &pool[index] {
			return false
		}
	}
	return true
}

func equalSimpleValueEnumerationReadForSource(read []simpleValueLiteralRead, source []CompiledLiteral) bool {
	if len(read) != len(source) {
		return false
	}
	for i := range source {
		if !equalSimpleValueLiteralReadForCompiled(read[i], source[i]) {
			return false
		}
	}
	return true
}

func equalColdFacetProjection(read simpleValueFacetRead, facets FacetSet) bool {
	if read.length != facets.Length || read.minLength != facets.MinLength || read.maxLength != facets.MaxLength ||
		read.totalDigits != facets.TotalDigits || read.fractionDigits != facets.FractionDigits ||
		read.present != facets.Present {
		return false
	}
	for i := range read.bounds {
		if !equalSimpleValueBoundReadForCompiled(read.bounds[i], facets.bounds[i]) {
			return false
		}
	}
	return true
}

func equalSimpleValueBoundReadForCompiled(a *simpleValueLiteralRead, b *CompiledLiteral) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return equalSimpleValueLiteralReadForCompiled(*a, *b)
}

func equalSimpleValueLiteralReadForCompiled(got simpleValueLiteralRead, want CompiledLiteral) bool {
	return got.actual.Valid == want.Actual.Valid &&
		(!got.actual.Valid || got.actual.Kind == want.Actual.Kind) &&
		got.canonical == want.Canonical &&
		EqualPrimitiveActualValues(got.actual, got.canonical, want.Actual, want.Canonical)
}

func equalStringPatternStepReadForSource(read *stringPatternStepRead, source *stringPatternStep) bool {
	if len(read.patterns) != len(source.patterns) {
		return false
	}
	for i, patternRead := range read.patterns {
		pattern := source.patterns[i]
		if (patternRead.re == nil) != (pattern.re == nil) ||
			patternRead.re != nil && patternRead.re.String() != pattern.re.String() ||
			!equalSimplePattern(patternRead.fast, pattern.fast) {
			return false
		}
	}
	return true
}

// equalSimpleValueQNameResolverNeedsForSimpleTypes reports whether reads expose
// the QName/NOTATION namespace-resolution needs for completed compiler-owned simple types.
func equalSimpleValueQNameResolverNeedsForSimpleTypes(reads []bool, simpleTypes []SimpleType) bool {
	return slices.Equal(reads, newSimpleValueQNameResolverNeedsForSimpleTypes(simpleTypes))
}

// validateSimpleValueQNameResolverNeedsForSimpleTypes validates QName resolver
// need projections against completed compiler-owned simple types.
func validateSimpleValueQNameResolverNeedsForSimpleTypes(reads []bool, simpleTypes []SimpleType) error {
	if len(reads) != len(simpleTypes) {
		return errors.New("simple value QName resolver projection count does not match types")
	}
	if !equalSimpleValueQNameResolverNeedsForSimpleTypes(reads, simpleTypes) {
		return errors.New("simple value QName resolver projection does not match type")
	}
	return nil
}
