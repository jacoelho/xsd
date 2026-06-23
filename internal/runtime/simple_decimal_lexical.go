package runtime

import "errors"

const (
	fastDecimalErrInvalid      = "invalid decimal"
	fastDecimalErrMinInclusive = "minInclusive facet failed"
	fastDecimalErrMaxInclusive = "maxInclusive facet failed"
)

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

// ValidateIntegerLexical validates raw as an xs:integer lexical value while
// preserving xs:decimal lexical diagnostics for non-decimal text.
func ValidateIntegerLexical[T byteText](raw T) error {
	scan, err := scanDecimalText(raw)
	if err != nil {
		return err
	}
	if scan.dot {
		return errors.New(fastIntErrInvalidInteger)
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
	if shape.MinInclusive.Negative || shape.MaxInclusive.Negative {
		return false, nil
	}
	return true, validateDecimalTextNonNegativeBounds(raw, shape.MinInclusive, shape.MaxInclusive)
}

func validateFastDecimalLexicalPublished(shape RawDecimalFastPathShape, raw []byte) (bool, error) {
	if shape.Facets&(FacetTotalDigits|FacetFractionDigits|FacetMinExclusive|FacetMaxExclusive|FacetEnumeration|FacetPattern) != 0 {
		return false, nil
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
		return ErrSimpleValueMetadata
	}
	for _, bound := range []RawDecimalBound{shape.MinInclusive, shape.MaxInclusive} {
		if !bound.Present || bound.Negative {
			continue
		}
		if bound.Int == "" || !asciiDigits(bound.Int) || !asciiDigits(bound.Frac) {
			return ErrSimpleValueMetadata
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
	if len(raw) == 0 {
		return decimalTextScan{}, errors.New(fastDecimalErrInvalid)
	}
	start := 0
	negative := false
	if raw[0] == '+' || raw[0] == '-' {
		negative = raw[0] == '-'
		start = 1
	}
	if start == len(raw) {
		return decimalTextScan{}, errors.New(fastDecimalErrInvalid)
	}
	dot := -1
	digits := 0
	for i := start; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '.':
			if dot >= 0 {
				return decimalTextScan{}, errors.New(fastDecimalErrInvalid)
			}
			dot = i
		case c >= '0' && c <= '9':
			digits++
		default:
			return decimalTextScan{}, errors.New(fastDecimalErrInvalid)
		}
	}
	if digits == 0 {
		return decimalTextScan{}, errors.New(fastDecimalErrInvalid)
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
			return errors.New(fastDecimalErrMinInclusive)
		}
		return nil
	}
	if minBound.Present && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, minBound) < 0 {
		return errors.New(fastDecimalErrMinInclusive)
	}
	if maxBound.Present && comparePositiveDecimalTextToBound(raw, intTrimStart, scan.intEnd, scan.fracStart, fracTrimEnd, maxBound) > 0 {
		return errors.New(fastDecimalErrMaxInclusive)
	}
	return nil
}

func comparePositiveDecimalTextToBound[T byteText](raw T, intTrimStart, intEnd, fracStart, fracTrimEnd int, bound RawDecimalBound) int {
	intDigits := intEnd - intTrimStart
	if intDigits == 0 {
		intDigits = 1
	}
	if intDigits < len(bound.Int) {
		return -1
	}
	if intDigits > len(bound.Int) {
		return 1
	}
	for i := range intDigits {
		digit := byte('0')
		if intEnd > intTrimStart {
			digit = raw[intTrimStart+i]
		}
		if digit < bound.Int[i] {
			return -1
		}
		if digit > bound.Int[i] {
			return 1
		}
	}
	fracDigits := fracTrimEnd - fracStart
	common := min(fracDigits, len(bound.Frac))
	for i := range common {
		if raw[fracStart+i] < bound.Frac[i] {
			return -1
		}
		if raw[fracStart+i] > bound.Frac[i] {
			return 1
		}
	}
	if fracDigits < len(bound.Frac) {
		return -1
	}
	if fracDigits > len(bound.Frac) {
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
