package xsd

import "fmt"

func applyTemporalBounds(kind primitiveKind, f facetSet, norm string, actual actualValue) error {
	if kind == primTime {
		return applyTimeBounds(f, norm, actual)
	}
	value := actual.Temporal
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = parseXSDTemporalValue(kind, norm)
		if err != nil {
			return err
		}
	}
	parse := func(s string) (xsdTemporalValue, error) {
		return parseXSDTemporalValue(kind, s)
	}
	return applyPartialBoundsParsed(f, value, parse, compareXSDTemporal, actualTemporalLiteral(kind))
}

func actualTemporalLiteral(kind primitiveKind) func(*compiledLiteral) (xsdTemporalValue, bool) {
	return func(l *compiledLiteral) (xsdTemporalValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.Temporal, true
		}
		return xsdTemporalValue{}, false
	}
}

func validateTemporalFacetBounds(kind primitiveKind, f facetSet) error {
	if kind == primTime {
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
	if partialFacetBoundsInvalid(lower, upper, compareXSDTemporal) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func temporalLowerBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out xsdTemporalValue) bool {
		return partialCompareForMinInclusive(compareXSDTemporal(other, out))
	})
}

func temporalUpperBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out xsdTemporalValue) bool {
		return partialCompareForMaxInclusive(compareXSDTemporal(other, out))
	})
}

func validateTimeFacetBounds(f facetSet) error {
	lower, err := timeLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := timeUpperBound(f)
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDTimePartial) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func validateTimeFacetRestriction(f, base facetSet, step orderedFacetStep) error {
	baseLower, err := timeRawLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := timeRawUpperBound(base)
	if err != nil {
		return err
	}
	if step.minInclusive && baseLower.present() {
		if err := validateTimeLowerRestriction("minInclusive", f.MinInclusive, false, baseLower); err != nil {
			return err
		}
	}
	if step.minExclusive && baseLower.present() {
		if err := validateTimeLowerRestriction("minExclusive", f.MinExclusive, true, baseLower); err != nil {
			return err
		}
	}
	if step.maxInclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction("maxInclusive", f.MaxInclusive, false, baseUpper); err != nil {
			return err
		}
	}
	if step.maxExclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction("maxExclusive", f.MaxExclusive, true, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeLowerRestriction(name string, lit *compiledLiteral, exclusive bool, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareLess || cmp == partialCompareEqual && !exclusive && base.exclusive() {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateTimeUpperRestriction(name string, lit *compiledLiteral, exclusive bool, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareGreater || cmp == partialCompareEqual && !exclusive && base.exclusive() {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

func timeLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

func timeUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

func timeRawLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundLexical(f.MinInclusive, f.MinExclusive, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

func timeRawUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundLexical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

func applyTimeBounds(f facetSet, norm string, actual actualValue) error {
	value := actual.Time
	if !actual.Valid || actual.Kind != primTime {
		var err error
		value, err = parseXSDTimeValue(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, parseXSDTimeValue, compareXSDTimePartial, actualTimeLiteral)
}

func actualTimeLiteral(l *compiledLiteral) (xsdTimeValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == primTime {
		return l.Actual.Time, true
	}
	return xsdTimeValue{}, false
}

func applyPartialBoundsParsed[T any](f facetSet, value T, parse func(string) (T, error), compare func(T, T) partialCompareResult, actual func(*compiledLiteral) (T, bool)) error {
	if err := applyPartialBound(f.MinInclusive, "minInclusive", value, parse, compare, actual, partialCompareForMinInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MaxInclusive, "maxInclusive", value, parse, compare, actual, partialCompareForMaxInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MinExclusive, "minExclusive", value, parse, compare, actual, partialCompareForMinExclusive); err != nil {
		return err
	}
	return applyPartialBound(f.MaxExclusive, "maxExclusive", value, parse, compare, actual, partialCompareForMaxExclusive)
}

func applyPartialBound[T any](
	lit *compiledLiteral,
	name string,
	value T,
	parse func(string) (T, error),
	compare func(T, T) partialCompareResult,
	actual func(*compiledLiteral) (T, bool),
	accept func(partialCompareResult) bool,
) error {
	if lit == nil {
		return nil
	}
	limit, err := facetLiteralValue(lit, facetCanonical, parse, actual)
	if err != nil {
		return err
	}
	if !accept(compare(value, limit)) {
		return fmt.Errorf("%s facet failed", name)
	}
	return nil
}
