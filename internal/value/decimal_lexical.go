package value

import (
	"cmp"
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
)

type rawEvaluationKind uint8

const (
	rawPlanNone rawEvaluationKind = iota
	rawPlanString
	rawPlanBoolean
	rawPlanPrimitive
	rawPlanDecimal
	rawPlanInteger
	rawPlanStringEnumeration
	rawPlanNMTokens
	rawPlanTextFacets
)

type rawEvaluationPlan struct {
	kind rawEvaluationKind
}

// prepareRawDecimalFastPath records the only decimal facet shape that can be
// admitted directly from borrowed bytes. Every other shape stays on the
// typed evaluator so its full value-space rules remain authoritative.
func prepareRawDecimalFastPath(t *typeDef) {
	f := &t.facets
	if t.variety != Atomic || t.primitive != PrimitiveDecimal || t.builtin != BuiltinNone {
		return
	}
	if f.present&^(FacetMinInclusive|FacetMaxInclusive) != 0 {
		return
	}
	if !rawNumericBoundsAllowed(f.lower, rawPlanDecimal) || (f.present&FacetMinInclusive != 0) != (len(f.lower) != 0) {
		return
	}
	if !rawNumericBoundsAllowed(f.upper, rawPlanDecimal) || (f.present&FacetMaxInclusive != 0) != (len(f.upper) != 0) {
		return
	}
	f.raw.kind = rawPlanDecimal
}

func prepareRawIntegerFastPath(t *typeDef) {
	if t.variety != Atomic || t.primitive != PrimitiveDecimal || t.builtin != BuiltinInteger || t.carriers != 0 {
		return
	}
	// Every BuiltinInteger inherits fractionDigits=0. That metadata is the
	// datatype's lexical rule, rather than a decimal facet that needs a second
	// evaluator.
	if t.facets.present&^(FacetFractionDigits|FacetMinInclusive|FacetMaxInclusive) != 0 {
		return
	}
	f := &t.facets
	if !rawNumericBoundsAllowed(f.lower, rawPlanInteger) || (f.present&FacetMinInclusive != 0) != (len(f.lower) != 0) {
		return
	}
	if !rawNumericBoundsAllowed(f.upper, rawPlanInteger) || (f.present&FacetMaxInclusive != 0) != (len(f.upper) != 0) {
		return
	}
	t.facets.raw.kind = rawPlanInteger
}

func rawNumericBoundsAllowed(bounds []boundValue, kind rawEvaluationKind) bool {
	if len(bounds) == 0 {
		return true
	}
	if len(bounds) != 1 {
		return false
	}
	bound := &bounds[0]
	return !bound.exclusive && bound.value.kind == PrimitiveDecimal &&
		(kind != rawPlanInteger || bound.value.decimal.IntegerLexical && bound.value.decimal.fracStart == bound.value.decimal.fracTrimEnd)
}

// Numeric plan admission establishes at most one compatible bound per endpoint.
func admittedRawNumericBound(bounds []boundValue) *DecimalValue {
	if len(bounds) == 0 {
		return nil
	}
	return &bounds[0].value.decimal
}

type integerSign bool

func validateRawIntegerBytes(f *facetProgram, whitespace WhitespaceMode, raw []byte) error {
	input := trimIntegerWhitespace(raw, whitespace)
	scan, err := scanIntegerText(input)
	if err != nil {
		return err
	}
	digitsStart := skipLeadingZeros(input, scan.start, scan.intEnd)
	negative := integerSign(scan.negative && digitsStart != scan.intEnd)
	if err := validateIntegerLowerBound(input, digitsStart, scan.intEnd, negative, admittedRawNumericBound(f.lower)); err != nil {
		return err
	}
	return validateIntegerUpperBound(input, digitsStart, scan.intEnd, negative, admittedRawNumericBound(f.upper))
}

func trimIntegerWhitespace(raw []byte, whitespace WhitespaceMode) []byte {
	if whitespace != WhitespaceCollapse {
		return raw
	}
	return lex.TrimXMLWhitespaceBytes(raw)
}

func validateIntegerLowerBound(raw []byte, digitsStart, digitsEnd int, negative integerSign, bound *DecimalValue) error {
	if bound == nil || compareIntegerBytesToBound(raw, digitsStart, digitsEnd, negative, bound) >= 0 {
		return nil
	}
	return errMinInclusiveFacet
}

func validateIntegerUpperBound(raw []byte, digitsStart, digitsEnd int, negative integerSign, bound *DecimalValue) error {
	if bound == nil || compareIntegerBytesToBound(raw, digitsStart, digitsEnd, negative, bound) <= 0 {
		return nil
	}
	return errMaxInclusiveFacet
}

func compareIntegerBytesToBound(raw []byte, digitsStart, digitsEnd int, negative integerSign, bound *DecimalValue) int {
	if bool(negative) != bound.negative {
		if negative {
			return -1
		}
		return 1
	}
	comparison := compareIntegerMagnitudeText(raw[digitsStart:digitsEnd], bound.text[bound.intTrimStart:bound.intEnd])
	if negative {
		return -comparison
	}
	return comparison
}

// ValidateIntegerLexical validates raw as an xs:integer lexical value.
func ValidateIntegerLexical[T byteText](raw T) error {
	_, err := scanIntegerText(raw)
	return err
}

// scanIntegerText owns the integer-family syntax reason: every lexical
// failure, including a decimal point, is "invalid integer".
func scanIntegerText[T byteText](raw T) (decimalTextScan, error) {
	scan, err := scanDecimalText(raw)
	if err != nil || scan.dot {
		return decimalTextScan{}, errors.New(msgInvalidInteger)
	}
	return scan, nil
}

func validateFastDecimalLexical[T byteText](f *facetProgram, raw T) (bool, error) {
	minBound := admittedRawNumericBound(f.lower)
	maxBound := admittedRawNumericBound(f.upper)
	if minBound == nil && maxBound == nil {
		_, err := scanDecimalText(raw)
		return true, err
	}
	if minBound != nil && minBound.negative || maxBound != nil && maxBound.negative {
		return false, nil
	}
	return true, validateDecimalTextNonNegativeBounds(raw, minBound, maxBound)
}

type decimalTextScan struct {
	start     int
	intEnd    int
	fracStart int
	negative  bool
	dot       bool
}

func scanDecimalText[T byteText](raw T) (decimalTextScan, error) {
	start, negative, err := scanDecimalSign(raw)
	if err != nil {
		return decimalTextScan{}, err
	}
	dot, digits, err := scanDecimalBody(raw, start)
	if err != nil {
		return decimalTextScan{}, err
	}
	if digits == 0 {
		return decimalTextScan{}, errors.New(msgInvalidDecimal)
	}

	intEnd := len(raw)
	fracStart := len(raw)
	if dot >= 0 {
		intEnd = dot
		fracStart = dot + 1
	}
	return decimalTextScan{
		start:     start,
		intEnd:    intEnd,
		fracStart: fracStart,
		negative:  negative,
		dot:       dot >= 0,
	}, nil
}

func scanDecimalSign[T byteText](raw T) (int, bool, error) {
	if len(raw) == 0 {
		return 0, false, errors.New(msgInvalidDecimal)
	}
	if raw[0] != '+' && raw[0] != '-' {
		return 0, false, nil
	}
	if len(raw) == 1 {
		return 0, false, errors.New(msgInvalidDecimal)
	}
	return 1, raw[0] == '-', nil
}

func scanDecimalBody[T byteText](raw T, start int) (dot, digits int, err error) {
	dot = -1
	for i := start; i < len(raw); i++ {
		switch c := raw[i]; {
		case c == '.' && dot < 0:
			dot = i
		case c >= '0' && c <= '9':
			digits++
		default:
			return 0, 0, errors.New(msgInvalidDecimal)
		}
	}
	return dot, digits, nil
}

func validateDecimalTextNonNegativeBounds[T byteText](raw T, minBound, maxBound *DecimalValue) error {
	scan, err := scanDecimalText(raw)
	if err != nil {
		return err
	}
	intTrimStart := skipLeadingZeros(raw, scan.start, scan.intEnd)
	fracTrimEnd := trimTrailingZeros(raw, scan.fracStart, len(raw))
	nonZero := intTrimStart < scan.intEnd || fracTrimEnd > scan.fracStart
	if scan.negative && nonZero {
		if minBound != nil {
			return errMinInclusiveFacet
		}
		return nil
	}
	if minBound != nil && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, minBound) < 0 {
		return errMinInclusiveFacet
	}
	if maxBound != nil && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, maxBound) > 0 {
		return errMaxInclusiveFacet
	}
	return nil
}

func comparePositiveDecimalTextToBound[T byteText](raw T, intTrimStart, intEnd, fracStart, fracTrimEnd int, bound *DecimalValue) int {
	if order := compareIntegerMagnitudeText(raw[intTrimStart:intEnd], bound.text[bound.intTrimStart:bound.intEnd]); order != 0 {
		return order
	}
	return compareDecimalFractionText(raw, fracStart, fracTrimEnd, bound.text[bound.fracStart:bound.fracTrimEnd])
}

// Integer magnitudes have no leading zeros; an empty span denotes zero.
func compareIntegerMagnitudeText[T byteText](raw T, bound string) int {
	if comparison := cmp.Compare(len(raw), len(bound)); comparison != 0 {
		return comparison
	}
	for i := range len(raw) {
		if comparison := cmp.Compare(raw[i], bound[i]); comparison != 0 {
			return comparison
		}
	}
	return 0
}

func compareDecimalFractionText[T byteText](raw T, start, end int, bound string) int {
	fracDigits := end - start
	common := min(fracDigits, len(bound))
	for i := range common {
		if raw[start+i] < bound[i] {
			return -1
		}
		if raw[start+i] > bound[i] {
			return 1
		}
	}
	if fracDigits < len(bound) {
		return -1
	}
	if fracDigits > len(bound) {
		return 1
	}
	return 0
}

func trimTrailingZeros[T byteText](raw T, start, end int) int {
	for end > start && raw[end-1] == '0' {
		end--
	}
	return end
}
