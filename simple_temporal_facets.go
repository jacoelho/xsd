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
	return applyPartialBoundsParsed(&f, value, parse, compareXSDTemporal, actualTemporalLiteral(kind))
}

// actualTemporalLiteral trusts cached values only for the primitive being checked.
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

// Temporal lower and upper facets use different partial-order acceptance rules.
func temporalLowerBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBound(f.MinInclusive, f.MinExclusive, facetCanonical, parse, func(other, out xsdTemporalValue) bool {
		return partialCompareForMinInclusive(compareXSDTemporal(other, out))
	})
}

// temporalUpperBound applies the max-facet rule for partial temporal order.
func temporalUpperBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBound(f.MaxInclusive, f.MaxExclusive, facetCanonical, parse, func(other, out xsdTemporalValue) bool {
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
		if err := validateTimeLowerRestriction(xsdFacetMinInclusive, f.MinInclusive, facetInclusive, baseLower); err != nil {
			return err
		}
	}
	if step.minExclusive && baseLower.present() {
		if err := validateTimeLowerRestriction(xsdFacetMinExclusive, f.MinExclusive, facetExclusive, baseLower); err != nil {
			return err
		}
	}
	if step.maxInclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction(xsdFacetMaxInclusive, f.MaxInclusive, facetInclusive, baseUpper); err != nil {
			return err
		}
	}
	if step.maxExclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction(xsdFacetMaxExclusive, f.MaxExclusive, facetExclusive, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeLowerRestriction(name string, lit *compiledLiteral, style facetBoundStyle, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareLess || cmp == partialCompareEqual && style == facetInclusive && base.exclusive() {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateTimeUpperRestriction(name string, lit *compiledLiteral, style facetBoundStyle, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareGreater || cmp == partialCompareEqual && style == facetInclusive && base.exclusive() {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

// Time lower and upper facets use different partial-order acceptance rules.
func timeLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBound(f.MinInclusive, f.MinExclusive, facetCanonical, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

// timeUpperBound applies the max-facet rule for partial time order.
func timeUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBound(f.MaxInclusive, f.MaxExclusive, facetCanonical, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

// Raw time bounds preserve lexical timezone absence during restriction checks.
func timeRawLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBound(f.MinInclusive, f.MinExclusive, facetLexical, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

// timeRawUpperBound preserves lexical timezone absence for max-facet checks.
func timeRawUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBound(f.MaxInclusive, f.MaxExclusive, facetLexical, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

// applyTimeBounds reuses parsed time values when validation already has them.
func applyTimeBounds(f facetSet, norm string, actual actualValue) error {
	value := actual.Time
	if !actual.Valid || actual.Kind != primTime {
		var err error
		value, err = parseXSDTimeValue(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(&f, value, parseXSDTimeValue, compareXSDTimePartial, actualTimeLiteral)
}

func actualTimeLiteral(l *compiledLiteral) (xsdTimeValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == primTime {
		return l.Actual.Time, true
	}
	return xsdTimeValue{}, false
}

func applyPartialBoundsParsed[T any](f *facetSet, value T, parse func(string) (T, error), compare func(T, T) partialCompareResult, actual func(*compiledLiteral) (T, bool)) error {
	if err := applyPartialBound(f.MinInclusive, xsdFacetMinInclusive, value, parse, compare, actual, partialCompareForMinInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MaxInclusive, xsdFacetMaxInclusive, value, parse, compare, actual, partialCompareForMaxInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MinExclusive, xsdFacetMinExclusive, value, parse, compare, actual, partialCompareForMinExclusive); err != nil {
		return err
	}
	return applyPartialBound(f.MaxExclusive, xsdFacetMaxExclusive, value, parse, compare, actual, partialCompareForMaxExclusive)
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
	var limit T
	var ok bool
	if actual != nil {
		limit, ok = actual(lit)
	}
	if !ok {
		var err error
		limit, err = parse(lit.Canonical)
		if err != nil {
			return err
		}
	}
	if !accept(compare(value, limit)) {
		return fmt.Errorf("%s facet failed", name)
	}
	return nil
}
