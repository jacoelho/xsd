package runtime

// PrimitiveActualValue is the parsed value-space projection cached for
// compiled literals and runtime atomic values.
type PrimitiveActualValue struct {
	Time     TimeValue
	Duration DurationValue
	Temporal TemporalValue
	Decimal  DecimalValue
	G        GValue
	Float    float64
	Length   uint32
	Kind     PrimitiveKind
	Valid    bool
	Boolean  bool
}

// PrimitiveActualResult is the parsed primitive value and canonical text.
type PrimitiveActualResult struct {
	Canonical string
	Actual    PrimitiveActualValue
}

// ParsePrimitiveActual parses namespace-independent primitive values into the
// cached actual-value projection used by schema literals and runtime
// validation. QName and NOTATION require schema namespace state.
func ParsePrimitiveActual(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	switch kind {
	case PrimitiveString, PrimitiveAnyURI:
		return parseTextPrimitiveActual(kind, normalized, needs)
	case PrimitiveBoolean:
		return parseBooleanPrimitiveActual(normalized)
	case PrimitiveDecimal:
		return parseDecimalPrimitiveActual(normalized, needs)
	case PrimitiveFloat, PrimitiveDouble:
		return parseFloatPrimitiveActual(kind, normalized, needs)
	case PrimitiveDuration:
		return parseDurationPrimitiveActual(normalized)
	case PrimitiveDate:
		return parseDatePrimitiveActual(normalized, needs)
	case PrimitiveDateTime:
		return parseDateTimePrimitiveActual(normalized, needs)
	case PrimitiveTime:
		return parseTimePrimitiveActual(normalized, needs)
	case PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		return parseGPrimitiveActual(kind, normalized, needs)
	case PrimitiveHexBinary, PrimitiveBase64Binary:
		return parseBinaryPrimitiveActual(kind, normalized, needs)
	default:
		return PrimitiveActualResult{}, ErrSimpleValueMetadata
	}
}

func newPrimitiveActual(kind PrimitiveKind) PrimitiveActualValue {
	return PrimitiveActualValue{Kind: kind, Valid: true}
}

func parseTextPrimitiveActual(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseTextValue(kind, normalized, needs)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(kind)
	actual.Length = value.Length
	return PrimitiveActualResult{Canonical: value.Canonical, Actual: actual}, nil
}

func parseBooleanPrimitiveActual(normalized string) (PrimitiveActualResult, error) {
	value, err := ParseBooleanValue(normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveBoolean)
	actual.Boolean = value
	return PrimitiveActualResult{Canonical: BooleanCanonical(value), Actual: actual}, nil
}

func parseDecimalPrimitiveActual(normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	var dec DecimalValue
	var err error
	if needs.Has(PrimitiveNeedCanonical) {
		dec, err = ParseDecimalCanonical(normalized)
	} else {
		dec, err = ParseDecimalValue(normalized)
	}
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveDecimal)
	actual.Decimal = dec
	return PrimitiveActualResult{Canonical: dec.Canonical, Actual: actual}, nil
}

func parseFloatPrimitiveActual(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseFloatValue(kind, normalized, needs)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(kind)
	actual.Float = value.Value
	return PrimitiveActualResult{Canonical: value.Canonical, Actual: actual}, nil
}

func parseDurationPrimitiveActual(normalized string) (PrimitiveActualResult, error) {
	value, err := ParseDurationValue(normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveDuration)
	actual.Duration = value
	return PrimitiveActualResult{Canonical: normalized, Actual: actual}, nil
}

func parseDatePrimitiveActual(normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseDateValue(normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveDate)
	actual.Temporal = value.Temporal()
	return PrimitiveActualResult{Canonical: canonicalIfNeeded(needs, value.CanonicalText), Actual: actual}, nil
}

func parseDateTimePrimitiveActual(normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseDateTimeValue(normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveDateTime)
	actual.Temporal = value.Temporal()
	return PrimitiveActualResult{Canonical: canonicalIfNeeded(needs, value.CanonicalText), Actual: actual}, nil
}

func parseTimePrimitiveActual(normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseTimeValue(normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(PrimitiveTime)
	actual.Time = value
	return PrimitiveActualResult{Canonical: canonicalIfNeeded(needs, value.CanonicalText), Actual: actual}, nil
}

func parseGPrimitiveActual(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseGValue(kind, normalized)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(kind)
	actual.G = value
	return PrimitiveActualResult{Canonical: canonicalIfNeeded(needs, value.CanonicalText), Actual: actual}, nil
}

func parseBinaryPrimitiveActual(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (PrimitiveActualResult, error) {
	value, err := ParseBinaryValue(kind, normalized, needs)
	if err != nil {
		return PrimitiveActualResult{}, err
	}
	actual := newPrimitiveActual(kind)
	actual.Length = value.Length
	return PrimitiveActualResult{Canonical: value.Canonical, Actual: actual}, nil
}

func canonicalIfNeeded(needs PrimitiveValueNeed, canonical func() string) string {
	if !needs.Has(PrimitiveNeedCanonical) {
		return ""
	}
	return canonical()
}

// EqualPrimitiveActualValues reports whether two primitive actual projections
// are equal, falling back to canonical text when either actual projection is
// absent or the primitive kind has no specialized value equality.
func EqualPrimitiveActualValues(actual PrimitiveActualValue, canonical string, literal PrimitiveActualValue, literalCanonical string) bool {
	if !actual.Valid || !literal.Valid || actual.Kind != literal.Kind {
		return literalCanonical == canonical
	}
	switch actual.Kind {
	case PrimitiveBoolean:
		return actual.Boolean == literal.Boolean
	case PrimitiveDecimal:
		return CompareDecimalValues(actual.Decimal, literal.Decimal) == 0
	case PrimitiveFloat, PrimitiveDouble:
		return EqualFloatValues(actual.Float, literal.Float)
	case PrimitiveDuration:
		return EqualDurationValues(actual.Duration, literal.Duration)
	case PrimitiveDate, PrimitiveDateTime:
		return EqualTemporalValues(actual.Temporal, literal.Temporal)
	case PrimitiveTime:
		return EqualTimeValues(actual.Time, literal.Time)
	case PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		return EqualGValues(actual.G, literal.G)
	default:
		return literalCanonical == canonical
	}
}
