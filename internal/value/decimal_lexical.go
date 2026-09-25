package value

import "errors"

// RawDecimalBound is a schema-projected inclusive decimal bound for raw decimal
// fast-path validation. Int is the trimmed non-negative integer part; Frac is
// the trimmed fractional part.
type RawDecimalBound struct {
	Int      string
	Frac     string
	Present  bool
	Negative bool
}

// RawDecimalFastPathShape is the frozen facet projection needed to decide
// whether runtime can validate raw decimal bytes without full value
// construction.
type RawDecimalFastPathShape struct {
	MinInclusive RawDecimalBound
	MaxInclusive RawDecimalBound
	Facets       FacetMask
}

// RawIntegerBound is the canonical sign and magnitude projection of an
// inclusive integer bound. Digits contains no sign and has no leading zeros.
type RawIntegerBound struct {
	Digits   string
	Present  bool
	Negative bool
}

// RawIntegerFastPathShape is the frozen facet projection needed to validate
// BuiltinInteger values directly from admitted bytes.
type RawIntegerFastPathShape struct {
	MinInclusive RawIntegerBound
	MaxInclusive RawIntegerBound
	Facets       FacetMask
}

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
	decimal RawDecimalFastPathShape
	integer RawIntegerFastPathShape
	kind    rawEvaluationKind
}

// prepareRawDecimalFastPath records the only decimal facet shape that can be
// admitted directly from borrowed bytes. Every other shape stays on the
// typed evaluator so its full value-space rules remain authoritative.
func prepareRawDecimalFastPath(t *typeDef) {
	if t == nil {
		return
	}
	f := &t.facets
	f.raw.decimal = RawDecimalFastPathShape{}
	if t.variety != Atomic || t.primitive != PrimitiveDecimal || t.builtin != BuiltinNone {
		return
	}
	if f.present&^(FacetMinInclusive|FacetMaxInclusive) != 0 {
		return
	}
	shape := RawDecimalFastPathShape{Facets: f.present}
	lowerBound, ok := rawDecimalFacetBound(f.lower)
	if !ok || (f.present&FacetMinInclusive != 0) != (len(f.lower) != 0) {
		return
	}
	upperBound, ok := rawDecimalFacetBound(f.upper)
	if !ok || (f.present&FacetMaxInclusive != 0) != (len(f.upper) != 0) {
		return
	}
	shape.MinInclusive, shape.MaxInclusive = lowerBound, upperBound
	f.raw.decimal = shape
	f.raw.kind = rawPlanDecimal
}

func prepareRawIntegerFastPath(t *typeDef) {
	if t == nil {
		return
	}
	t.facets.raw.integer = RawIntegerFastPathShape{}
	if t.variety != Atomic || t.primitive != PrimitiveDecimal || t.builtin != BuiltinInteger || t.identity != IdentityNone {
		return
	}
	// Every BuiltinInteger inherits fractionDigits=0. That metadata is the
	// datatype's lexical rule, rather than a decimal facet that needs a second
	// evaluator.
	if t.facets.present&^(FacetFractionDigits|FacetMinInclusive|FacetMaxInclusive) != 0 {
		return
	}
	shape := RawIntegerFastPathShape{Facets: t.facets.present}
	lower, ok := rawIntegerFacetBound(t.facets.lower)
	if !ok || (t.facets.present&FacetMinInclusive != 0) != lower.Present {
		return
	}
	upper, ok := rawIntegerFacetBound(t.facets.upper)
	if !ok || (t.facets.present&FacetMaxInclusive != 0) != upper.Present {
		return
	}
	shape.MinInclusive, shape.MaxInclusive = lower, upper
	t.facets.raw.integer = shape
	t.facets.raw.kind = rawPlanInteger
}

func rawIntegerFacetBound(bounds []boundValue) (RawIntegerBound, bool) {
	if len(bounds) == 0 {
		return RawIntegerBound{}, true
	}
	if len(bounds) != 1 {
		return RawIntegerBound{}, false
	}
	bound := bounds[0]
	if bound.exclusive || bound.value.isList || bound.value.atom.kind != PrimitiveDecimal {
		return RawIntegerBound{}, false
	}
	decimal := bound.value.atom.decimal
	if !decimal.IntegerLexical || decimal.fracDigits() != 0 {
		return RawIntegerBound{}, false
	}
	digits := decimal.IntegerCanonicalText()
	negative := decimal.IsNegative()
	if negative && digits != "" && digits[0] == '-' {
		digits = digits[1:]
	}
	return RawIntegerBound{Digits: digits, Present: true, Negative: negative}, true
}

type integerSign uint8

const (
	integerNonNegative integerSign = iota
	integerNegative
)

func validateRawIntegerBytes(f *facetProgram, whitespace WhitespaceMode, raw []byte) error {
	input := trimIntegerWhitespace(raw, whitespace)
	scan, err := scanDecimalText(input)
	if err != nil {
		return err
	}
	if scan.dot {
		return errors.New(rawDecimalErrInvalidInteger)
	}
	digitsStart := skipLeadingZeros(input, scan.start, scan.intEnd)
	sign := integerLexicalSign(scan, digitsStart, scan.intEnd)
	shape := f.raw.integer
	if err := validateIntegerLowerBound(input, digitsStart, scan.intEnd, sign, shape.MinInclusive); err != nil {
		return err
	}
	return validateIntegerUpperBound(input, digitsStart, scan.intEnd, sign, shape.MaxInclusive)
}

func trimIntegerWhitespace(raw []byte, whitespace WhitespaceMode) []byte {
	if whitespace != WhitespaceCollapse {
		return raw
	}
	start, end := 0, len(raw)
	for start < end && isXMLWhitespaceByte(raw[start]) {
		start++
	}
	for end > start && isXMLWhitespaceByte(raw[end-1]) {
		end--
	}
	return raw[start:end]
}

func integerLexicalSign(scan decimalTextScan, digitsStart, digitsEnd int) integerSign {
	if scan.negative && digitsStart != digitsEnd {
		return integerNegative
	}
	return integerNonNegative
}

func validateIntegerLowerBound(raw []byte, digitsStart, digitsEnd int, sign integerSign, bound RawIntegerBound) error {
	if !bound.Present || compareIntegerBytesToBound(raw, digitsStart, digitsEnd, sign, bound) >= 0 {
		return nil
	}
	return errors.New(rawDecimalErrMinInclusive)
}

func validateIntegerUpperBound(raw []byte, digitsStart, digitsEnd int, sign integerSign, bound RawIntegerBound) error {
	if !bound.Present || compareIntegerBytesToBound(raw, digitsStart, digitsEnd, sign, bound) <= 0 {
		return nil
	}
	return errors.New(rawDecimalErrMaxInclusive)
}

func isXMLWhitespaceByte(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func compareIntegerBytesToBound(raw []byte, digitsStart, digitsEnd int, sign integerSign, bound RawIntegerBound) int {
	if comparison := compareIntegerSigns(sign, bound); comparison != 0 {
		return comparison
	}
	return compareIntegerMagnitudes(raw, digitsStart, digitsEnd, sign, bound.Digits)
}

func compareIntegerSigns(sign integerSign, bound RawIntegerBound) int {
	boundNegative := bound.Negative && bound.Digits != "0"
	if sign == integerNegative && !boundNegative {
		return -1
	}
	if sign == integerNonNegative && boundNegative {
		return 1
	}
	return 0
}

func compareIntegerMagnitudes(raw []byte, digitsStart, digitsEnd int, sign integerSign, boundDigits string) int {
	leftDigits := digitsEnd - digitsStart
	if leftDigits == 0 {
		leftDigits = 1
	}
	if comparison := compareIntegerMagnitudeLengths(leftDigits, len(boundDigits), sign); comparison != 0 {
		return comparison
	}
	for i := range leftDigits {
		digit := byte('0')
		if digitsEnd > digitsStart {
			digit = raw[digitsStart+i]
		}
		if comparison := compareIntegerDigits(digit, boundDigits[i], sign); comparison != 0 {
			return comparison
		}
	}
	return 0
}

func compareIntegerMagnitudeLengths(left, right int, sign integerSign) int {
	if left == right {
		return 0
	}
	if left < right {
		if sign == integerNegative {
			return 1
		}
		return -1
	}
	if sign == integerNegative {
		return -1
	}
	return 1
}

func compareIntegerDigits(left, right byte, sign integerSign) int {
	if left == right {
		return 0
	}
	if left < right {
		if sign == integerNegative {
			return 1
		}
		return -1
	}
	if sign == integerNegative {
		return -1
	}
	return 1
}

func rawDecimalFacetBound(bounds []boundValue) (RawDecimalBound, bool) {
	if len(bounds) == 0 {
		return RawDecimalBound{}, true
	}
	if len(bounds) != 1 {
		return RawDecimalBound{}, false
	}
	bound := bounds[0]
	if bound.exclusive || bound.value.isList || bound.value.atom.kind != PrimitiveDecimal {
		return RawDecimalBound{}, false
	}
	return bound.value.atom.decimal.RawBound(), true
}

// ValidateIntegerLexical validates raw as an xs:integer lexical value while
// preserving xs:decimal lexical diagnostics for non-decimal text.
func ValidateIntegerLexical[T byteText](raw T) error {
	scan, err := scanDecimalText(raw)
	if err != nil {
		return err
	}
	if scan.dot {
		return errors.New(rawDecimalErrInvalidInteger)
	}
	return nil
}

// ValidateFastDecimalLexical validates the supported raw xs:decimal fast path.
// It returns handled=false when the frozen facet shape needs the full decimal
// parser/facet executor.
func ValidateFastDecimalLexical[T byteText](shape RawDecimalFastPathShape, raw T) (bool, error) {
	if shape.Facets&(FacetTotalDigits|FacetFractionDigits|FacetMinExclusive|FacetMaxExclusive|FacetEnumeration|FacetPattern) != 0 {
		return false, nil
	}
	if err := validateRawDecimalBoundProjection(shape); err != nil {
		return false, err
	}
	if !shape.MinInclusive.Present && !shape.MaxInclusive.Present {
		if _, err := scanDecimalText(raw); err != nil {
			return true, err
		}
		return true, nil
	}
	if shape.MinInclusive.Negative || shape.MaxInclusive.Negative {
		return false, nil
	}
	return true, validateDecimalTextNonNegativeBounds(raw, shape.MinInclusive, shape.MaxInclusive)
}

func validateRawDecimalBoundProjection(shape RawDecimalFastPathShape) error {
	hasMin := shape.Facets&FacetMinInclusive != 0
	hasMax := shape.Facets&FacetMaxInclusive != 0
	if hasMin != shape.MinInclusive.Present || hasMax != shape.MaxInclusive.Present {
		return ErrMetadata
	}
	for _, bound := range []RawDecimalBound{shape.MinInclusive, shape.MaxInclusive} {
		if !bound.Present || bound.Negative {
			continue
		}
		if bound.Int == "" || !asciiDigits(bound.Int) || !asciiDigits(bound.Frac) {
			return ErrMetadata
		}
	}
	return nil
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
		return decimalTextScan{}, errors.New(rawDecimalErrInvalidDecimal)
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
		return 0, false, errors.New(rawDecimalErrInvalidDecimal)
	}
	if raw[0] != '+' && raw[0] != '-' {
		return 0, false, nil
	}
	if len(raw) == 1 {
		return 0, false, errors.New(rawDecimalErrInvalidDecimal)
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
			return 0, 0, errors.New(rawDecimalErrInvalidDecimal)
		}
	}
	return dot, digits, nil
}

func validateDecimalTextNonNegativeBounds[T byteText](raw T, minBound, maxBound RawDecimalBound) error {
	scan, err := scanDecimalText(raw)
	if err != nil {
		return err
	}
	intTrimStart := skipLeadingZeros(raw, scan.start, scan.intEnd)
	fracTrimEnd := trimTrailingZeros(raw, scan.fracStart, len(raw))
	nonZero := intTrimStart < scan.intEnd || fracTrimEnd > scan.fracStart
	if scan.negative && nonZero {
		if minBound.Present {
			return errors.New(rawDecimalErrMinInclusive)
		}
		return nil
	}
	if minBound.Present && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, minBound) < 0 {
		return errors.New(rawDecimalErrMinInclusive)
	}
	if maxBound.Present && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, maxBound) > 0 {
		return errors.New(rawDecimalErrMaxInclusive)
	}
	return nil
}

func comparePositiveDecimalTextToBound[T byteText](raw T, intTrimStart, intEnd, fracStart, fracTrimEnd int, bound RawDecimalBound) int {
	if order := compareDecimalIntegerText(raw, intTrimStart, intEnd, bound.Int); order != 0 {
		return order
	}
	return compareDecimalFractionText(raw, fracStart, fracTrimEnd, bound.Frac)
}

func compareDecimalIntegerText[T byteText](raw T, start, end int, bound string) int {
	intDigits := end - start
	if intDigits == 0 {
		intDigits = 1
	}
	if intDigits < len(bound) {
		return -1
	}
	if intDigits > len(bound) {
		return 1
	}
	for i := range intDigits {
		digit := byte('0')
		if end > start {
			digit = raw[start+i]
		}
		if digit < bound[i] {
			return -1
		}
		if digit > bound[i] {
			return 1
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

func asciiDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
