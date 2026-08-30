package runtime

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/lex"
)

const runtimeLengthFacetMask = FacetLength | FacetMinLength | FacetMaxLength

// AtomicSimpleValueInput is the runtime-owned fallback validation projection
// for atomic simple values that cannot use a no-output fast path.
type AtomicSimpleValueInput struct {
	ResolveQName func(string) (ns, local string, ok bool)
	Normalized   string
	Facets       SimpleValueFacets
	Type         SimpleValueType
	Needs        PrimitiveValueNeed
	Present      bool
}

type simpleValueNotationReader interface {
	simpleValueNotation(ns, local string) (bool, bool)
}

func validateAtomicSimpleValueFallbackWithReader[R simpleValueNotationReader](reader R, in AtomicSimpleValueInput) (AtomicSimpleValueResult, error) {
	return validateAtomicSimpleValueFallbackWithReaderAndScratch(reader, in, nil)
}

func validateAtomicSimpleValueFallbackWithReaderAndScratch[R simpleValueNotationReader](reader R, in AtomicSimpleValueInput, scratch *StringPatternScratch) (AtomicSimpleValueResult, error) {
	if !in.Present {
		return AtomicSimpleValueResult{}, ErrSimpleValueMetadata
	}
	typ := in.Type
	facets := in.Facets
	parsed, err := validateAtomicPrimitiveActual(reader, typ.Primitive, in.Normalized, in.ResolveQName, in.Needs)
	if err != nil {
		return AtomicSimpleValueResult{}, err
	}
	canon := atomicSimpleCanonical(typ, in.Needs, parsed)
	if err := applyAtomicValueFacets(typ, facets, in.Normalized, canon, parsed.Actual, scratch); err != nil {
		return AtomicSimpleValueResult{}, err
	}
	return AtomicSimpleValueResult{
		Canonical:         canon,
		IdentityCanonical: atomicIdentityCanonical(typ.Primitive, parsed.Actual, in.Needs),
	}, nil
}

func atomicSimpleCanonical(typ SimpleValueType, needs PrimitiveValueNeed, parsed PrimitiveActualResult) string {
	if typ.Builtin == BuiltinValidationInteger && needs.Has(PrimitiveNeedCanonical) {
		return parsed.Actual.Decimal.IntegerCanonicalText()
	}
	return parsed.Canonical
}

func applyAtomicValueFacets(typ SimpleValueType, facets SimpleValueFacets, normalized, canonical string, actual PrimitiveActualValue, scratch *StringPatternScratch) error {
	if facets.Facets == 0 {
		return nil
	}
	if err := applyAtomicFacets(typ.Primitive, typ.Builtin, facets, normalized, actual); err != nil {
		return err
	}
	return applyPatternAndEnumeration(facets, normalized, canonical, actual, scratch)
}

func atomicIdentityCanonical(primitive PrimitiveKind, actual PrimitiveActualValue, needs PrimitiveValueNeed) string {
	if !needs.Has(PrimitiveNeedIdentity) || !actual.Valid || actual.Kind != primitive {
		return ""
	}
	switch primitive {
	case PrimitiveDecimal:
		return actual.Decimal.CanonicalText()
	case PrimitiveDuration:
		return durationIdentityCanonical(actual.Duration)
	case PrimitiveString, PrimitiveBoolean, PrimitiveFloat, PrimitiveDouble,
		PrimitiveDateTime, PrimitiveTime, PrimitiveDate,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return ""
	default:
	}
	return ""
}

func validateAtomicPrimitiveActual[R simpleValueNotationReader](reader R, kind PrimitiveKind, normalized string, resolve func(string) (string, string, bool), needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	actual := PrimitiveActualValue{Kind: kind, Valid: true}
	switch kind {
	case PrimitiveQName:
		canon, err := validateQNamePrimitive(normalized, resolve, needs)
		return PrimitiveActualResult{Canonical: canon, Actual: actual}, err
	case PrimitiveNotation:
		canon, err := validateNotationPrimitive(reader, normalized, resolve, needs)
		return PrimitiveActualResult{Canonical: canon, Actual: actual}, err
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble,
		PrimitiveDuration, PrimitiveDateTime, PrimitiveTime, PrimitiveDate,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI:
		return ParsePrimitiveActual(kind, normalized, needs)
	default:
	}
	return ParsePrimitiveActual(kind, normalized, needs)
}

func validateQNamePrimitive(normalized string, resolve func(string) (string, string, bool), needs PrimitiveValueNeed) (string, error) {
	if resolve == nil {
		if !lex.IsNCName(normalized) {
			return "", fmt.Errorf("invalid QName")
		}
		if !needs.Has(PrimitiveNeedCanonical) {
			return "", nil
		}
		return normalized, nil
	}
	ns, local, ok := resolve(normalized)
	if !ok {
		return "", fmt.Errorf("unresolved QName")
	}
	if !needs.Has(PrimitiveNeedCanonical) {
		return "", nil
	}
	return FormatExpandedName(ns, local), nil
}

func validateNotationPrimitive[R simpleValueNotationReader](reader R, normalized string, resolve func(string) (string, string, bool), needs PrimitiveValueNeed) (string, error) {
	if resolve == nil {
		if !lex.IsNCName(normalized) {
			return "", fmt.Errorf("invalid NOTATION")
		}
		return validateResolvedNotation(reader, "", normalized, normalized, needs)
	}
	ns, local, ok := resolve(normalized)
	if !ok {
		return "", fmt.Errorf("unresolved NOTATION")
	}
	return validateResolvedNotation(reader, ns, local, FormatExpandedName(ns, local), needs)
}

func validateResolvedNotation[R simpleValueNotationReader](reader R, ns, local, canonical string, needs PrimitiveValueNeed) (string, error) {
	declared, known := reader.simpleValueNotation(ns, local)
	if !known {
		return "", ErrSimpleValueMetadata
	}
	if !declared {
		return "", fmt.Errorf("undeclared notation")
	}
	if !needs.Has(PrimitiveNeedCanonical) {
		return "", nil
	}
	return canonical, nil
}

func applyAtomicFacets(primitive PrimitiveKind, builtin BuiltinValidationKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	if f.Facets&runtimeLengthFacetMask != 0 && !SimpleValueAtomicLengthFacets(AtomicLengthFacetShape{
		Facets:    f.Facets,
		Primitive: primitive,
		Builtin:   builtin,
	}) {
		if err := applyAtomicLengthFacets(primitive, f, normalized, actual); err != nil {
			return err
		}
	}
	if primitive == PrimitiveDecimal {
		return applyAtomicDecimalFacets(f.DecimalFacets, normalized, actual)
	}
	return applyPrimitiveBounds(primitive, f, normalized, actual)
}

func applyAtomicDecimalFacets(facets DecimalFacetValues, normalized string, actual PrimitiveActualValue) error {
	value := actual.Decimal
	if !actual.Valid || actual.Kind != PrimitiveDecimal {
		var err error
		value, err = ParseDecimalValue(normalized)
		if err != nil {
			return err
		}
	}
	return ValidateDecimalFacets(facets, value)
}

func applyAtomicLengthFacets(primitive PrimitiveKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	if primitive == PrimitiveQName || primitive == PrimitiveNotation {
		return nil
	}
	length := actual.Length
	if !actual.Valid || actual.Kind != primitive {
		var err error
		length, err = PrimitiveLength(primitive, normalized)
		if err != nil {
			return err
		}
	}
	return ValidateLengthFacets(f.LengthFacets, length)
}

func applyPrimitiveBounds(kind PrimitiveKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	switch kind {
	case PrimitiveFloat, PrimitiveDouble:
		return applyFloatBounds(kind, f, normalized, actual)
	case PrimitiveDuration:
		return applyDurationBounds(f, normalized, actual)
	case PrimitiveGDay, PrimitiveGMonthDay, PrimitiveGMonth, PrimitiveGYearMonth, PrimitiveGYear:
		return applyGValueBounds(kind, f, normalized, actual)
	case PrimitiveDate, PrimitiveDateTime, PrimitiveTime:
		return applyTemporalBounds(kind, f, normalized, actual)
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return nil
	default:
	}
	return nil
}

func applyPatternAndEnumeration(f SimpleValueFacets, normalized, canonical string, actual PrimitiveActualValue, scratch *StringPatternScratch) error {
	if err := f.StringFacets.validatePatterns(normalized, scratch); err != nil {
		return err
	}
	if len(f.enumerationReads) != 0 {
		return validatePrivateAtomicEnumeration(f.enumerationReads, actual, canonical)
	}
	if len(f.Enumeration) != 0 {
		return validatePublishedAtomicEnumeration(f.Enumeration, actual, canonical)
	}
	return nil
}

func validatePrivateAtomicEnumeration(enumeration []simpleValueLiteralRead, actual PrimitiveActualValue, canonical string) error {
	for _, lit := range enumeration {
		if EqualPrimitiveActualValues(actual, canonical, lit.actual, lit.canonical) {
			return nil
		}
	}
	return errors.New("enumeration facet failed")
}

func validatePublishedAtomicEnumeration(enumeration []SimpleValueFacetLiteral, actual PrimitiveActualValue, canonical string) error {
	for _, lit := range enumeration {
		if EqualPrimitiveActualValues(actual, canonical, lit.Actual, lit.Canonical) {
			return nil
		}
	}
	return errors.New("enumeration facet failed")
}

func applyFloatBounds(kind PrimitiveKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	value := actual.Float
	if !actual.Valid || actual.Kind != kind {
		parsed, err := ParseFloatValue(kind, normalized, 0)
		if err != nil {
			return err
		}
		value = parsed.Value
	}

	facets, err := runtimeFloatFacetValues(kind, f)
	if err != nil {
		return err
	}
	return ValidateFloatFacets(facets, value)
}

func runtimeFloatFacetValues(kind PrimitiveKind, f SimpleValueFacets) (FloatFacetValues, error) {
	minInclusive, err := runtimeFloatFacetValue(kind, f.MinInclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxInclusive, err := runtimeFloatFacetValue(kind, f.MaxInclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	minExclusive, err := runtimeFloatFacetValue(kind, f.MinExclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	maxExclusive, err := runtimeFloatFacetValue(kind, f.MaxExclusive)
	if err != nil {
		return FloatFacetValues{}, err
	}
	return FloatFacetValues{
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
		MinExclusive: minExclusive,
		MaxExclusive: maxExclusive,
		Facets:       f.Facets,
	}, nil
}

func runtimeFloatFacetValue(kind PrimitiveKind, lit SimpleValueFacetLiteral) (FloatFacetValue, error) {
	if !lit.Present {
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

func applyDurationBounds(f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	value := actual.Duration
	if !actual.Valid || actual.Kind != PrimitiveDuration {
		var err error
		value, err = ParseDurationValue(normalized)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, ParseDurationValue, CompareDurationValues, actualDurationFacetLiteral)
}

func actualDurationFacetLiteral(l SimpleValueFacetLiteral) (DurationValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == PrimitiveDuration {
		return l.Actual.Duration, true
	}
	return DurationValue{}, false
}

func applyGValueBounds(kind PrimitiveKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	value := actual.G
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = ParseGValue(kind, normalized)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, func(s string) (GValue, error) {
		return ParseGValue(kind, s)
	}, CompareGValues, actualGValueFacetLiteral(kind))
}

func actualGValueFacetLiteral(kind PrimitiveKind) func(SimpleValueFacetLiteral) (GValue, bool) {
	return func(l SimpleValueFacetLiteral) (GValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.G, true
		}
		return GValue{}, false
	}
}

func applyTemporalBounds(kind PrimitiveKind, f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	if kind == PrimitiveTime {
		return applyTimeBounds(f, normalized, actual)
	}
	value := actual.Temporal
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = ParseTemporalValue(kind, normalized)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, func(s string) (TemporalValue, error) {
		return ParseTemporalValue(kind, s)
	}, CompareTemporalValues, actualTemporalFacetLiteral(kind))
}

func actualTemporalFacetLiteral(kind PrimitiveKind) func(SimpleValueFacetLiteral) (TemporalValue, bool) {
	return func(l SimpleValueFacetLiteral) (TemporalValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.Temporal, true
		}
		return TemporalValue{}, false
	}
}

func applyTimeBounds(f SimpleValueFacets, normalized string, actual PrimitiveActualValue) error {
	value := actual.Time
	if !actual.Valid || actual.Kind != PrimitiveTime {
		var err error
		value, err = ParseTimeValue(normalized)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, ParseTimeValue, CompareTimePartial, actualTimeFacetLiteral)
}

func actualTimeFacetLiteral(l SimpleValueFacetLiteral) (TimeValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == PrimitiveTime {
		return l.Actual.Time, true
	}
	return TimeValue{}, false
}

func applyPartialBoundsParsed[T any](f SimpleValueFacets, value T, parse func(string) (T, error), compare func(T, T) OrderedFacetRelation, actual func(SimpleValueFacetLiteral) (T, bool)) error {
	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	reader := partialBoundReader[T]{value: value, parse: parse, compare: compare, actual: actual}
	if err := reader.apply(f.MinInclusive, "minInclusive", inclusive, OrderedFacetLowerBoundAccepts); err != nil {
		return err
	}
	if err := reader.apply(f.MaxInclusive, "maxInclusive", inclusive, OrderedFacetUpperBoundAccepts); err != nil {
		return err
	}
	if err := reader.apply(f.MinExclusive, "minExclusive", exclusive, OrderedFacetLowerBoundAccepts); err != nil {
		return err
	}
	return reader.apply(f.MaxExclusive, "maxExclusive", exclusive, OrderedFacetUpperBoundAccepts)
}

type partialBoundReader[T any] struct {
	value   T
	parse   func(string) (T, error)
	compare func(T, T) OrderedFacetRelation
	actual  func(SimpleValueFacetLiteral) (T, bool)
}

func (r partialBoundReader[T]) apply(
	lit SimpleValueFacetLiteral,
	name string,
	bound OrderedFacetBound,
	accept func(OrderedFacetBound, OrderedFacetRelation) bool,
) error {
	if !lit.Present {
		return nil
	}
	var limit T
	var ok bool
	if r.actual != nil {
		limit, ok = r.actual(lit)
	}
	if !ok {
		var err error
		limit, err = r.parse(lit.Canonical)
		if err != nil {
			return err
		}
	}
	if !accept(bound, r.compare(r.value, limit)) {
		return fmt.Errorf("%s facet failed", name)
	}
	return nil
}
