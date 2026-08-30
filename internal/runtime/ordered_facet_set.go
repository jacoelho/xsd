package runtime

import "github.com/jacoelho/xsd/internal/vocab"

// ValidatePrimitiveFacetRestrictions validates primitive-specific ordered facet
// consistency and restriction rules for one derived simple type.
func ValidatePrimitiveFacetRestrictions(st SimpleType, baseFacets FacetSet, step OrderedFacetStep) error {
	if err := checkPrimitiveFacetRestrictions(st, baseFacets, step); err != nil {
		return err
	}
	return ValidateOrderedFacetBaseRestriction(OrderedFacetBaseRestriction{
		Step:                 step,
		DerivedRestrictsBase: OrderedFacetSetRestricts(st.Variety, st.Primitive, st.Facets, baseFacets),
	})
}

func checkPrimitiveFacetRestrictions(st SimpleType, baseFacets FacetSet, step OrderedFacetStep) error {
	switch st.Primitive {
	case PrimitiveDecimal:
		return validateDecimalPrimitiveFacets(st.Facets, baseFacets, step)
	case PrimitiveFloat, PrimitiveDouble:
		return ValidateFloatFacetSetBounds(st.Primitive, st.Facets)
	case PrimitiveDuration:
		return validateDurationFacetBounds(st.Facets)
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return validateGValueFacetBounds(st.Primitive, st.Facets)
	case PrimitiveDate, PrimitiveDateTime:
		return validateTemporalFacetBounds(st.Primitive, st.Facets)
	case PrimitiveTime:
		return validateTimePrimitiveFacets(st.Facets, baseFacets, step)
	case PrimitiveString, PrimitiveBoolean, PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return nil
	default:
	}
	return nil
}

func validateDecimalPrimitiveFacets(facets, base FacetSet, step OrderedFacetStep) error {
	if err := validateDecimalFacetRestriction(facets, base, step); err != nil {
		return err
	}
	return validateDecimalFacetBounds(facets)
}

func validateTimePrimitiveFacets(facets, base FacetSet, step OrderedFacetStep) error {
	if err := validateTimeFacetRestriction(facets, base, step); err != nil {
		return err
	}
	return validateTemporalFacetBounds(PrimitiveTime, facets)
}

// OrderedFacetSetRestricts reports whether the derived facet set is at least
// as restrictive as the base facet set for the primitive's ordered facets.
func OrderedFacetSetRestricts(variety SimpleVariety, primitive PrimitiveKind, facets, base FacetSet) bool {
	if !FacetAllowedForSimpleType(variety, primitive, FacetMinInclusive) {
		return true
	}
	switch primitive {
	case PrimitiveDecimal:
		return decimalOrderedFacetsRestrict(facets, base)
	case PrimitiveFloat, PrimitiveDouble:
		return floatFacetSetRestricts(primitive, facets, base)
	case PrimitiveDuration:
		return durationOrderedFacetsRestrict(facets, base)
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return gValueOrderedFacetsRestrict(primitive, facets, base)
	case PrimitiveDate, PrimitiveDateTime:
		return temporalOrderedFacetsRestrict(primitive, facets, base)
	case PrimitiveTime:
		return timeOrderedFacetsRestrict(facets, base)
	case PrimitiveString, PrimitiveBoolean, PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return true
	default:
	}
	return true
}

func validateDecimalFacetRestriction(f, base FacetSet, step OrderedFacetStep) error {
	return validateOrderedFacetRestrictions(f, base, step, orderedFacetRestrictionOps[DecimalValue]{
		lowerBound: decimalLowerBound,
		upperBound: decimalUpperBound,
		lower:      validateDecimalLowerRestriction,
		upper:      validateDecimalUpperRestriction,
	})
}

type orderedFacetRestrictionOps[T any] struct {
	lowerBound func(FacetSet) (typedFacetBound[T], error)
	upperBound func(FacetSet) (typedFacetBound[T], error)
	lower      func(string, optionalCompiledLiteral, OrderedFacetBoundKind, typedFacetBound[T]) error
	upper      func(string, optionalCompiledLiteral, OrderedFacetBoundKind, typedFacetBound[T]) error
}

func validateOrderedFacetRestrictions[T any](f, base FacetSet, step OrderedFacetStep, ops orderedFacetRestrictionOps[T]) error {
	baseLower, err := ops.lowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := ops.upperBound(base)
	if err != nil {
		return err
	}
	if err := validateDeclaredFacetRestriction(facetRestrictionRequest[T]{
		validate: ops.lower,
		base:     baseLower,
		literal:  optionalBoundFacet(f, FacetMinInclusive),
		name:     vocab.XSDFacetMinInclusive,
		kind:     OrderedFacetBoundInclusive,
		present:  step.MinInclusive,
	}); err != nil {
		return err
	}
	if err := validateDeclaredFacetRestriction(facetRestrictionRequest[T]{
		validate: ops.lower,
		base:     baseLower,
		literal:  optionalBoundFacet(f, FacetMinExclusive),
		name:     vocab.XSDFacetMinExclusive,
		kind:     OrderedFacetBoundExclusive,
		present:  step.MinExclusive,
	}); err != nil {
		return err
	}
	if err := validateDeclaredFacetRestriction(facetRestrictionRequest[T]{
		validate: ops.upper,
		base:     baseUpper,
		literal:  optionalBoundFacet(f, FacetMaxInclusive),
		name:     vocab.XSDFacetMaxInclusive,
		kind:     OrderedFacetBoundInclusive,
		present:  step.MaxInclusive,
	}); err != nil {
		return err
	}
	return validateDeclaredFacetRestriction(facetRestrictionRequest[T]{
		validate: ops.upper,
		base:     baseUpper,
		literal:  optionalBoundFacet(f, FacetMaxExclusive),
		name:     vocab.XSDFacetMaxExclusive,
		kind:     OrderedFacetBoundExclusive,
		present:  step.MaxExclusive,
	})
}

type facetRestrictionRequest[T any] struct {
	validate func(string, optionalCompiledLiteral, OrderedFacetBoundKind, typedFacetBound[T]) error
	base     typedFacetBound[T]
	name     string
	literal  optionalCompiledLiteral
	kind     OrderedFacetBoundKind
	present  bool
}

func validateDeclaredFacetRestriction[T any](request facetRestrictionRequest[T]) error {
	if !request.present {
		return nil
	}
	return request.validate(request.name, request.literal, request.kind, request.base)
}

func optionalBoundFacet(facets FacetSet, flag FacetMask) optionalCompiledLiteral {
	literal, present := BoundFacet(facets, flag)
	return optionalCompiledLiteral{value: literal, present: present}
}

func validateDecimalLowerRestriction(name string, literal optionalCompiledLiteral, kind OrderedFacetBoundKind, base typedFacetBound[DecimalValue]) error {
	if !literal.present || !base.present() {
		return nil
	}
	relation := orderedFacetRelationFromInt(CompareDecimalValues(literal.value.Actual.Decimal, base.value))
	return ValidateOrderedFacetLowerRestriction(OrderedFacetBoundRestriction{
		Facet:    name,
		Derived:  OrderedFacetBound{Kind: kind},
		Base:     base.bound,
		Relation: relation,
	})
}

func validateDecimalUpperRestriction(name string, literal optionalCompiledLiteral, kind OrderedFacetBoundKind, base typedFacetBound[DecimalValue]) error {
	if !literal.present || !base.present() {
		return nil
	}
	relation := orderedFacetRelationFromInt(CompareDecimalValues(literal.value.Actual.Decimal, base.value))
	return ValidateOrderedFacetUpperRestriction(OrderedFacetBoundRestriction{
		Facet:    name,
		Derived:  OrderedFacetBound{Kind: kind},
		Base:     base.bound,
		Relation: relation,
	})
}

func validateDecimalFacetBounds(f FacetSet) error {
	lower, err := decimalLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := decimalUpperBound(f)
	if err != nil {
		return err
	}
	if !lower.present() || !upper.present() {
		return nil
	}
	return checkOrderedFacetBounds(PrimitiveDecimal, lower, upper, func(lower, upper DecimalValue) OrderedFacetRelation {
		return orderedFacetRelationFromInt(CompareDecimalValues(lower, upper))
	})
}

func decimalOrderedFacetsRestrict(f, base FacetSet) bool {
	lower, err := decimalLowerBound(f)
	if err != nil {
		return false
	}
	baseLower, err := decimalLowerBound(base)
	if err != nil {
		return false
	}
	upper, err := decimalUpperBound(f)
	if err != nil {
		return false
	}
	baseUpper, err := decimalUpperBound(base)
	if err != nil {
		return false
	}
	relation := func(got, base DecimalValue) OrderedFacetRelation {
		return orderedFacetRelationFromInt(CompareDecimalValues(got, base))
	}
	return orderedFacetLowerRestrictsCompared(lower, baseLower, relation) &&
		orderedFacetUpperRestrictsCompared(upper, baseUpper, relation)
}

func decimalLowerBound(f FacetSet) (typedFacetBound[DecimalValue], error) {
	bounds := lowerBoundFacets(f)
	return facetBound(
		bounds,
		facetCanonical, ParseDecimalValue, func(other, out DecimalValue) bool {
			return CompareDecimalValues(other, out) >= 0
		})
}

func decimalUpperBound(f FacetSet) (typedFacetBound[DecimalValue], error) {
	bounds := upperBoundFacets(f)
	return facetBound(
		bounds,
		facetCanonical, ParseDecimalValue, func(other, out DecimalValue) bool {
			return CompareDecimalValues(other, out) <= 0
		})
}

type optionalCompiledLiteral struct {
	value   CompiledLiteral
	present bool
}

type facetBoundLiterals struct {
	inclusive optionalCompiledLiteral
	exclusive optionalCompiledLiteral
}

func lowerBoundFacets(f FacetSet) facetBoundLiterals {
	inclusive, inclusivePresent := BoundFacet(f, FacetMinInclusive)
	exclusive, exclusivePresent := BoundFacet(f, FacetMinExclusive)
	return facetBoundLiterals{
		inclusive: optionalCompiledLiteral{value: inclusive, present: inclusivePresent},
		exclusive: optionalCompiledLiteral{value: exclusive, present: exclusivePresent},
	}
}

func upperBoundFacets(f FacetSet) facetBoundLiterals {
	inclusive, inclusivePresent := BoundFacet(f, FacetMaxInclusive)
	exclusive, exclusivePresent := BoundFacet(f, FacetMaxExclusive)
	return facetBoundLiterals{
		inclusive: optionalCompiledLiteral{value: inclusive, present: inclusivePresent},
		exclusive: optionalCompiledLiteral{value: exclusive, present: exclusivePresent},
	}
}

// ValidateFloatFacetSetBounds validates effective lower/upper float bound
// consistency for xs:float/xs:double facets stored in a FacetSet.
func ValidateFloatFacetSetBounds(kind PrimitiveKind, f FacetSet) error {
	facets, err := floatFacetValues(kind, f)
	if err != nil {
		return err
	}
	return ValidateFloatFacetBounds(kind, facets)
}

func floatFacetSetRestricts(kind PrimitiveKind, f, base FacetSet) bool {
	facets, err := floatFacetValues(kind, f)
	if err != nil {
		return false
	}
	baseFacets, err := floatFacetValues(kind, base)
	if err != nil {
		return false
	}
	return FloatOrderedFacetsRestrict(facets, baseFacets)
}

func floatFacetValues(kind PrimitiveKind, f FacetSet) (FloatFacetValues, error) {
	minInclusiveLit, hasMinInclusive := BoundFacet(f, FacetMinInclusive)
	maxInclusiveLit, hasMaxInclusive := BoundFacet(f, FacetMaxInclusive)
	minExclusiveLit, hasMinExclusive := BoundFacet(f, FacetMinExclusive)
	maxExclusiveLit, hasMaxExclusive := BoundFacet(f, FacetMaxExclusive)
	minInclusive, err := floatFacetValue(kind, optionalCompiledLiteral{value: minInclusiveLit, present: hasMinInclusive})
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxInclusive, err := floatFacetValue(kind, optionalCompiledLiteral{value: maxInclusiveLit, present: hasMaxInclusive})
	if err != nil {
		return FloatFacetValues{}, err
	}
	minExclusive, err := floatFacetValue(kind, optionalCompiledLiteral{value: minExclusiveLit, present: hasMinExclusive})
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxExclusive, err := floatFacetValue(kind, optionalCompiledLiteral{value: maxExclusiveLit, present: hasMaxExclusive})
	if err != nil {
		return FloatFacetValues{}, err
	}
	return FloatFacetValues{
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
		MinExclusive: minExclusive,
		MaxExclusive: maxExclusive,
		Facets:       f.Present,
	}, nil
}

func floatFacetValue(kind PrimitiveKind, literal optionalCompiledLiteral) (FloatFacetValue, error) {
	if !literal.present {
		return FloatFacetValue{}, nil
	}
	lit := literal.value
	if lit.Actual.Valid && lit.Actual.Kind == kind {
		return FloatFacetValue{Value: lit.Actual.Float, Present: true}, nil
	}
	parsed, err := ParseFloatValue(kind, lit.Canonical, 0)
	if err != nil {
		return FloatFacetValue{}, err
	}
	return FloatFacetValue{Value: parsed.Value, Present: true}, nil
}

func validateDurationFacetBounds(f FacetSet) error {
	lower, err := durationLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := durationUpperBound(f)
	if err != nil {
		return err
	}
	return checkOrderedFacetBounds(PrimitiveDuration, lower, upper, CompareDurationValues)
}

func durationOrderedFacetsRestrict(f, base FacetSet) bool {
	lower, err := durationLowerBound(f)
	if err != nil {
		return false
	}
	baseLower, err := durationLowerBound(base)
	if err != nil {
		return false
	}
	upper, err := durationUpperBound(f)
	if err != nil {
		return false
	}
	baseUpper, err := durationUpperBound(base)
	if err != nil {
		return false
	}
	return orderedFacetLowerRestrictsCompared(lower, baseLower, CompareDurationValues) &&
		orderedFacetUpperRestrictsCompared(upper, baseUpper, CompareDurationValues)
}

func durationLowerBound(f FacetSet) (typedFacetBound[DurationValue], error) {
	bounds := lowerBoundFacets(f)
	return facetBound(bounds, facetCanonical, ParseDurationValue, func(other, out DurationValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareDurationValues(other, out),
		)
	})
}

func durationUpperBound(f FacetSet) (typedFacetBound[DurationValue], error) {
	bounds := upperBoundFacets(f)
	return facetBound(bounds, facetCanonical, ParseDurationValue, func(other, out DurationValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareDurationValues(other, out),
		)
	})
}

func validateGValueFacetBounds(kind PrimitiveKind, f FacetSet) error {
	lower, err := gValueLowerBound(kind, f)
	if err != nil {
		return err
	}
	upper, err := gValueUpperBound(kind, f)
	if err != nil {
		return err
	}
	return checkOrderedFacetBounds(kind, lower, upper, CompareGValues)
}

func gValueLowerBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[GValue], error) {
	parse, ok := gValueFacet(kind)
	if !ok {
		return typedFacetBound[GValue]{}, nil
	}
	bounds := lowerBoundFacets(f)
	return facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
}

func gValueUpperBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[GValue], error) {
	parse, ok := gValueFacet(kind)
	if !ok {
		return typedFacetBound[GValue]{}, nil
	}
	bounds := upperBoundFacets(f)
	return facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
}

func gValueOrderedFacetsRestrict(kind PrimitiveKind, f, base FacetSet) bool {
	bounds, ok := gValueBounds(kind, f, base)
	if !ok {
		return false
	}
	return orderedFacetLowerRestrictsCompared(bounds.derivedLower, bounds.baseLower, CompareGValues) &&
		orderedFacetUpperRestrictsCompared(bounds.derivedUpper, bounds.baseUpper, CompareGValues)
}

type gValueFacetBounds struct {
	derivedLower typedFacetBound[GValue]
	baseLower    typedFacetBound[GValue]
	derivedUpper typedFacetBound[GValue]
	baseUpper    typedFacetBound[GValue]
}

func gValueBounds(kind PrimitiveKind, f, base FacetSet) (gValueFacetBounds, bool) {
	parse, ok := gValueFacet(kind)
	if !ok {
		return gValueFacetBounds{}, false
	}
	bounds := lowerBoundFacets(f)
	lower, err := facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return gValueFacetBounds{}, false
	}
	bounds = lowerBoundFacets(base)
	baseLower, err := facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return gValueFacetBounds{}, false
	}
	bounds = upperBoundFacets(f)
	upper, err := facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return gValueFacetBounds{}, false
	}
	bounds = upperBoundFacets(base)
	baseUpper, err := facetBound(bounds, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return gValueFacetBounds{}, false
	}
	return gValueFacetBounds{
		derivedLower: lower,
		baseLower:    baseLower,
		derivedUpper: upper,
		baseUpper:    baseUpper,
	}, true
}

func gValueFacet(kind PrimitiveKind) (func(string) (GValue, error), bool) {
	switch kind {
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return func(s string) (GValue, error) {
			return ParseGValue(kind, s)
		}, true
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble, PrimitiveDuration,
		PrimitiveDateTime, PrimitiveTime, PrimitiveDate, PrimitiveHexBinary, PrimitiveBase64Binary,
		PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return nil, false
	default:
	}
	return nil, false
}

func validateTemporalFacetBounds(kind PrimitiveKind, f FacetSet) error {
	if kind == PrimitiveTime {
		return validateTimeFacetBounds(f)
	}
	lower, err := temporalLowerBound(kind, f)
	if err != nil {
		return err
	}
	upper, err := temporalUpperBound(kind, f)
	if err != nil {
		return err
	}
	return checkOrderedFacetBounds(kind, lower, upper, CompareTemporalValues)
}

func temporalOrderedFacetsRestrict(kind PrimitiveKind, f, base FacetSet) bool {
	lower, err := temporalLowerBound(kind, f)
	if err != nil {
		return false
	}
	baseLower, err := temporalLowerBound(kind, base)
	if err != nil {
		return false
	}
	upper, err := temporalUpperBound(kind, f)
	if err != nil {
		return false
	}
	baseUpper, err := temporalUpperBound(kind, base)
	if err != nil {
		return false
	}
	return orderedFacetLowerRestrictsCompared(lower, baseLower, CompareTemporalValues) &&
		orderedFacetUpperRestrictsCompared(upper, baseUpper, CompareTemporalValues)
}

func temporalLowerBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[TemporalValue], error) {
	parse := func(s string) (TemporalValue, error) { return ParseTemporalValue(kind, s) }
	bounds := lowerBoundFacets(f)
	return facetBound(bounds, facetCanonical, parse, func(other, out TemporalValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTemporalValues(other, out),
		)
	})
}

func temporalUpperBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[TemporalValue], error) {
	parse := func(s string) (TemporalValue, error) { return ParseTemporalValue(kind, s) }
	bounds := upperBoundFacets(f)
	return facetBound(bounds, facetCanonical, parse, func(other, out TemporalValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTemporalValues(other, out),
		)
	})
}

func validateTimeFacetBounds(f FacetSet) error {
	lower, err := timeLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := timeUpperBound(f)
	if err != nil {
		return err
	}
	return checkOrderedFacetBounds(PrimitiveTime, lower, upper, CompareTimePartial)
}

func validateTimeFacetRestriction(f, base FacetSet, step OrderedFacetStep) error {
	return validateOrderedFacetRestrictions(f, base, step, orderedFacetRestrictionOps[TimeValue]{
		lowerBound: timeRawLowerBound,
		upperBound: timeRawUpperBound,
		lower:      validateTimeLowerRestriction,
		upper:      validateTimeUpperRestriction,
	})
}

func validateTimeLowerRestriction(name string, literal optionalCompiledLiteral, kind OrderedFacetBoundKind, base typedFacetBound[TimeValue]) error {
	if !literal.present || !base.present() {
		return nil
	}
	value, err := ParseTimeRawValue(literal.value.Lexical)
	if err != nil {
		return err
	}
	relation := CompareTimePartial(value, base.value)
	return ValidateOrderedFacetLowerRestriction(OrderedFacetBoundRestriction{
		Facet:    name,
		Derived:  OrderedFacetBound{Kind: kind},
		Base:     base.bound,
		Relation: relation,
	})
}

func validateTimeUpperRestriction(name string, literal optionalCompiledLiteral, kind OrderedFacetBoundKind, base typedFacetBound[TimeValue]) error {
	if !literal.present || !base.present() {
		return nil
	}
	value, err := ParseTimeRawValue(literal.value.Lexical)
	if err != nil {
		return err
	}
	relation := CompareTimePartial(value, base.value)
	return ValidateOrderedFacetUpperRestriction(OrderedFacetBoundRestriction{
		Facet:    name,
		Derived:  OrderedFacetBound{Kind: kind},
		Base:     base.bound,
		Relation: relation,
	})
}

func timeOrderedFacetsRestrict(f, base FacetSet) bool {
	lower, err := timeRawLowerBound(f)
	if err != nil {
		return false
	}
	baseLower, err := timeRawLowerBound(base)
	if err != nil {
		return false
	}
	upper, err := timeRawUpperBound(f)
	if err != nil {
		return false
	}
	baseUpper, err := timeRawUpperBound(base)
	if err != nil {
		return false
	}
	return orderedFacetLowerRestrictsCompared(lower, baseLower, CompareTimePartial) &&
		orderedFacetUpperRestrictsCompared(upper, baseUpper, CompareTimePartial)
}

func timeLowerBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	bounds := lowerBoundFacets(f)
	return facetBound(bounds, facetCanonical, ParseTimeValue, func(other, out TimeValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeUpperBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	bounds := upperBoundFacets(f)
	return facetBound(bounds, facetCanonical, ParseTimeValue, func(other, out TimeValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeRawLowerBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	bounds := lowerBoundFacets(f)
	return facetBound(bounds, facetLexical, ParseTimeRawValue, func(other, out TimeValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeRawUpperBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	bounds := upperBoundFacets(f)
	return facetBound(bounds, facetLexical, ParseTimeRawValue, func(other, out TimeValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

type typedFacetBound[T any] struct {
	value T
	bound OrderedFacetBound
}

func (b typedFacetBound[T]) present() bool {
	return b.bound.present()
}

func facetBound[T any](
	bounds facetBoundLiterals,
	text func(CompiledLiteral) string,
	parse func(string) (T, error),
	preferExclusive func(T, T) bool,
) (typedFacetBound[T], error) {
	if !bounds.inclusive.present {
		if !bounds.exclusive.present {
			return typedFacetBound[T]{}, nil
		}
		out, err := parse(text(bounds.exclusive.value))
		if err != nil {
			return typedFacetBound[T]{}, err
		}
		return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundExclusive}}, nil
	}

	out, err := parse(text(bounds.inclusive.value))
	if err != nil {
		return typedFacetBound[T]{}, err
	}
	if !bounds.exclusive.present {
		return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundInclusive}}, nil
	}
	other, err := parse(text(bounds.exclusive.value))
	if err != nil {
		return typedFacetBound[T]{}, err
	}
	if preferExclusive(other, out) {
		return typedFacetBound[T]{value: other, bound: OrderedFacetBound{Kind: OrderedFacetBoundExclusive}}, nil
	}
	return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundInclusive}}, nil
}

func checkOrderedFacetBounds[T any](kind PrimitiveKind, lower, upper typedFacetBound[T], relation func(T, T) OrderedFacetRelation) error {
	cmp := OrderedFacetIncomparable
	if lower.present() && upper.present() {
		cmp = relation(lower.value, upper.value)
	}
	return ValidateOrderedFacetBounds(OrderedFacetBoundsValidation{
		Primitive: kind,
		Lower:     lower.bound,
		Upper:     upper.bound,
		Relation:  cmp,
	})
}

func orderedFacetLowerRestrictsCompared[T any](got, base typedFacetBound[T], relation func(T, T) OrderedFacetRelation) bool {
	cmp := OrderedFacetIncomparable
	if got.present() && base.present() {
		cmp = relation(got.value, base.value)
	}
	return OrderedFacetLowerRestricts(got.bound, base.bound, cmp)
}

func orderedFacetUpperRestrictsCompared[T any](got, base typedFacetBound[T], relation func(T, T) OrderedFacetRelation) bool {
	cmp := OrderedFacetIncomparable
	if got.present() && base.present() {
		cmp = relation(got.value, base.value)
	}
	return OrderedFacetUpperRestricts(got.bound, base.bound, cmp)
}

func facetCanonical(l CompiledLiteral) string {
	return l.Canonical
}

func facetLexical(l CompiledLiteral) string {
	return l.Lexical
}
