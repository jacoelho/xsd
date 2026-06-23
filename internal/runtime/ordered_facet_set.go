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
		if err := validateDecimalFacetRestriction(st.Facets, baseFacets, step); err != nil {
			return err
		}
		if err := validateDecimalFacetBounds(st.Facets); err != nil {
			return err
		}
	case PrimitiveFloat, PrimitiveDouble:
		if err := ValidateFloatFacetSetBounds(st.Primitive, st.Facets); err != nil {
			return err
		}
	case PrimitiveDuration:
		if err := validateDurationFacetBounds(st.Facets); err != nil {
			return err
		}
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		if err := validateGValueFacetBounds(st.Primitive, st.Facets); err != nil {
			return err
		}
	case PrimitiveDate, PrimitiveDateTime:
		if err := validateTemporalFacetBounds(st.Primitive, st.Facets); err != nil {
			return err
		}
	case PrimitiveTime:
		if err := validateTimeFacetRestriction(st.Facets, baseFacets, step); err != nil {
			return err
		}
		if err := validateTemporalFacetBounds(st.Primitive, st.Facets); err != nil {
			return err
		}
	default:
	}
	return nil
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
		return floatOrderedFacetsRestrict(primitive, facets, base)
	case PrimitiveDuration:
		return durationOrderedFacetsRestrict(facets, base)
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return gValueOrderedFacetsRestrict(primitive, facets, base)
	case PrimitiveDate, PrimitiveDateTime:
		return temporalOrderedFacetsRestrict(primitive, facets, base)
	case PrimitiveTime:
		return timeOrderedFacetsRestrict(facets, base)
	default:
		return true
	}
}

func validateDecimalFacetRestriction(f, base FacetSet, step OrderedFacetStep) error {
	baseLower, err := decimalLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := decimalUpperBound(base)
	if err != nil {
		return err
	}
	if step.MinInclusive {
		lit, present := BoundFacet(f, FacetMinInclusive)
		if err := validateDecimalLowerRestriction(vocab.XSDFacetMinInclusive, lit, present, OrderedFacetBoundInclusive, baseLower); err != nil {
			return err
		}
	}
	if step.MinExclusive {
		lit, present := BoundFacet(f, FacetMinExclusive)
		if err := validateDecimalLowerRestriction(vocab.XSDFacetMinExclusive, lit, present, OrderedFacetBoundExclusive, baseLower); err != nil {
			return err
		}
	}
	if step.MaxInclusive {
		lit, present := BoundFacet(f, FacetMaxInclusive)
		if err := validateDecimalUpperRestriction(vocab.XSDFacetMaxInclusive, lit, present, OrderedFacetBoundInclusive, baseUpper); err != nil {
			return err
		}
	}
	if step.MaxExclusive {
		lit, present := BoundFacet(f, FacetMaxExclusive)
		if err := validateDecimalUpperRestriction(vocab.XSDFacetMaxExclusive, lit, present, OrderedFacetBoundExclusive, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateDecimalLowerRestriction(name string, lit CompiledLiteral, litPresent bool, kind OrderedFacetBoundKind, base typedFacetBound[DecimalValue]) error {
	if !litPresent || !base.present() {
		return nil
	}
	relation := orderedFacetRelationFromInt(CompareDecimalValues(lit.Actual.Decimal, base.value))
	return ValidateOrderedFacetLowerRestriction(OrderedFacetBoundRestriction{
		Facet:    name,
		Derived:  OrderedFacetBound{Kind: kind},
		Base:     base.bound,
		Relation: relation,
	})
}

func validateDecimalUpperRestriction(name string, lit CompiledLiteral, litPresent bool, kind OrderedFacetBoundKind, base typedFacetBound[DecimalValue]) error {
	if !litPresent || !base.present() {
		return nil
	}
	relation := orderedFacetRelationFromInt(CompareDecimalValues(lit.Actual.Decimal, base.value))
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
	return validateOrderedFacetBounds(PrimitiveDecimal, lower, upper, func(lower, upper DecimalValue) OrderedFacetRelation {
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
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(
		inclusive, inclusivePresent,
		exclusive, exclusivePresent,
		facetCanonical, ParseDecimalValue, func(other, out DecimalValue) bool {
			return CompareDecimalValues(other, out) >= 0
		})
}

func decimalUpperBound(f FacetSet) (typedFacetBound[DecimalValue], error) {
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(
		inclusive, inclusivePresent,
		exclusive, exclusivePresent,
		facetCanonical, ParseDecimalValue, func(other, out DecimalValue) bool {
			return CompareDecimalValues(other, out) <= 0
		})
}

func lowerBoundFacets(f FacetSet) (CompiledLiteral, bool, CompiledLiteral, bool) {
	inclusive, inclusivePresent := BoundFacet(f, FacetMinInclusive)
	exclusive, exclusivePresent := BoundFacet(f, FacetMinExclusive)
	return inclusive, inclusivePresent, exclusive, exclusivePresent
}

func upperBoundFacets(f FacetSet) (CompiledLiteral, bool, CompiledLiteral, bool) {
	inclusive, inclusivePresent := BoundFacet(f, FacetMaxInclusive)
	exclusive, exclusivePresent := BoundFacet(f, FacetMaxExclusive)
	return inclusive, inclusivePresent, exclusive, exclusivePresent
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

func floatOrderedFacetsRestrict(kind PrimitiveKind, f, base FacetSet) bool {
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
	minInclusive, err := floatFacetValue(kind, minInclusiveLit, hasMinInclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxInclusive, err := floatFacetValue(kind, maxInclusiveLit, hasMaxInclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	minExclusive, err := floatFacetValue(kind, minExclusiveLit, hasMinExclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxExclusive, err := floatFacetValue(kind, maxExclusiveLit, hasMaxExclusive)
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

func floatFacetValue(kind PrimitiveKind, lit CompiledLiteral, present bool) (FloatFacetValue, error) {
	if !present {
		return FloatFacetValue{}, nil
	}
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
	return validateOrderedFacetBounds(PrimitiveDuration, lower, upper, CompareDurationValues)
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
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, ParseDurationValue, func(other, out DurationValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareDurationValues(other, out),
		)
	})
}

func durationUpperBound(f FacetSet) (typedFacetBound[DurationValue], error) {
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, ParseDurationValue, func(other, out DurationValue) bool {
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
	return validateOrderedFacetBounds(kind, lower, upper, CompareGValues)
}

func gValueLowerBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[GValue], error) {
	parse, ok := gValueFacet(kind)
	if !ok {
		return typedFacetBound[GValue]{}, nil
	}
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
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
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
}

func gValueOrderedFacetsRestrict(kind PrimitiveKind, f, base FacetSet) bool {
	lower, baseLower, upper, baseUpper, ok := gValueBounds(kind, f, base)
	if !ok {
		return false
	}
	return orderedFacetLowerRestrictsCompared(lower, baseLower, CompareGValues) &&
		orderedFacetUpperRestrictsCompared(upper, baseUpper, CompareGValues)
}

func gValueBounds(kind PrimitiveKind, f, base FacetSet) (
	typedFacetBound[GValue],
	typedFacetBound[GValue],
	typedFacetBound[GValue],
	typedFacetBound[GValue],
	bool,
) {
	parse, ok := gValueFacet(kind)
	if !ok {
		return typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, false
	}
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	lower, err := facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, false
	}
	inclusive, inclusivePresent, exclusive, exclusivePresent = lowerBoundFacets(base)
	baseLower, err := facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, false
	}
	inclusive, inclusivePresent, exclusive, exclusivePresent = upperBoundFacets(f)
	upper, err := facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, false
	}
	inclusive, inclusivePresent, exclusive, exclusivePresent = upperBoundFacets(base)
	baseUpper, err := facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out GValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareGValues(other, out),
		)
	})
	if err != nil {
		return typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, typedFacetBound[GValue]{}, false
	}
	return lower, baseLower, upper, baseUpper, true
}

func gValueFacet(kind PrimitiveKind) (func(string) (GValue, error), bool) {
	switch kind {
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return func(s string) (GValue, error) {
			return ParseGValue(kind, s)
		}, true
	default:
		return nil, false
	}
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
	return validateOrderedFacetBounds(kind, lower, upper, CompareTemporalValues)
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
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out TemporalValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTemporalValues(other, out),
		)
	})
}

func temporalUpperBound(kind PrimitiveKind, f FacetSet) (typedFacetBound[TemporalValue], error) {
	parse := func(s string) (TemporalValue, error) { return ParseTemporalValue(kind, s) }
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, parse, func(other, out TemporalValue) bool {
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
	return validateOrderedFacetBounds(PrimitiveTime, lower, upper, CompareTimePartial)
}

func validateTimeFacetRestriction(f, base FacetSet, step OrderedFacetStep) error {
	baseLower, err := timeRawLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := timeRawUpperBound(base)
	if err != nil {
		return err
	}
	if step.MinInclusive && baseLower.present() {
		lit, present := BoundFacet(f, FacetMinInclusive)
		if err := validateTimeLowerRestriction(vocab.XSDFacetMinInclusive, lit, present, OrderedFacetBoundInclusive, baseLower); err != nil {
			return err
		}
	}
	if step.MinExclusive && baseLower.present() {
		lit, present := BoundFacet(f, FacetMinExclusive)
		if err := validateTimeLowerRestriction(vocab.XSDFacetMinExclusive, lit, present, OrderedFacetBoundExclusive, baseLower); err != nil {
			return err
		}
	}
	if step.MaxInclusive && baseUpper.present() {
		lit, present := BoundFacet(f, FacetMaxInclusive)
		if err := validateTimeUpperRestriction(vocab.XSDFacetMaxInclusive, lit, present, OrderedFacetBoundInclusive, baseUpper); err != nil {
			return err
		}
	}
	if step.MaxExclusive && baseUpper.present() {
		lit, present := BoundFacet(f, FacetMaxExclusive)
		if err := validateTimeUpperRestriction(vocab.XSDFacetMaxExclusive, lit, present, OrderedFacetBoundExclusive, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeLowerRestriction(name string, lit CompiledLiteral, litPresent bool, kind OrderedFacetBoundKind, base typedFacetBound[TimeValue]) error {
	if !litPresent {
		return nil
	}
	value, err := ParseTimeRawValue(lit.Lexical)
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

func validateTimeUpperRestriction(name string, lit CompiledLiteral, litPresent bool, kind OrderedFacetBoundKind, base typedFacetBound[TimeValue]) error {
	if !litPresent {
		return nil
	}
	value, err := ParseTimeRawValue(lit.Lexical)
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
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, ParseTimeValue, func(other, out TimeValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeUpperBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetCanonical, ParseTimeValue, func(other, out TimeValue) bool {
		return OrderedFacetUpperBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeRawLowerBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	inclusive, inclusivePresent, exclusive, exclusivePresent := lowerBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetLexical, ParseTimeRawValue, func(other, out TimeValue) bool {
		return OrderedFacetLowerBoundAccepts(
			OrderedFacetBound{Kind: OrderedFacetBoundInclusive},
			CompareTimePartial(other, out),
		)
	})
}

func timeRawUpperBound(f FacetSet) (typedFacetBound[TimeValue], error) {
	inclusive, inclusivePresent, exclusive, exclusivePresent := upperBoundFacets(f)
	return facetBound(inclusive, inclusivePresent, exclusive, exclusivePresent, facetLexical, ParseTimeRawValue, func(other, out TimeValue) bool {
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
	inclusive CompiledLiteral,
	inclusivePresent bool,
	exclusive CompiledLiteral,
	exclusivePresent bool,
	text func(CompiledLiteral) string,
	parse func(string) (T, error),
	preferExclusive func(T, T) bool,
) (typedFacetBound[T], error) {
	if !inclusivePresent {
		if !exclusivePresent {
			return typedFacetBound[T]{}, nil
		}
		out, err := parse(text(exclusive))
		if err != nil {
			return typedFacetBound[T]{}, err
		}
		return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundExclusive}}, nil
	}

	out, err := parse(text(inclusive))
	if err != nil {
		return typedFacetBound[T]{}, err
	}
	if !exclusivePresent {
		return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundInclusive}}, nil
	}
	other, err := parse(text(exclusive))
	if err != nil {
		return typedFacetBound[T]{}, err
	}
	if preferExclusive(other, out) {
		return typedFacetBound[T]{value: other, bound: OrderedFacetBound{Kind: OrderedFacetBoundExclusive}}, nil
	}
	return typedFacetBound[T]{value: out, bound: OrderedFacetBound{Kind: OrderedFacetBoundInclusive}}, nil
}

func validateOrderedFacetBounds[T any](kind PrimitiveKind, lower, upper typedFacetBound[T], relation func(T, T) OrderedFacetRelation) error {
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
