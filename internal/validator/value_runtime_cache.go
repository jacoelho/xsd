package validator

import (
	"unicode/utf8"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

// ValueCache stores parsed primitive values and derived facet measurements for one validation.
type ValueCache struct {
	intVal         num.Int
	decVal         num.Dec
	fractionDigits int
	totalDigits    int
	length         int
	float64Val     float64
	float32Val     float32
	decSet         bool
	intSet         bool
	float32Set     bool
	float64Set     bool
	digitsSet      bool
	lengthSet      bool
	float32Class   num.FloatClass
	float64Class   num.FloatClass
}

// SetDecimal stores one parsed decimal and its digit counts.
func (c *ValueCache) SetDecimal(val num.Dec) {
	if c == nil {
		return
	}
	c.decVal = val
	c.decSet = true
	c.totalDigits = len(val.Coef)
	c.fractionDigits = int(val.Scale)
	c.digitsSet = true
}

// SetInteger stores one parsed integer and its digit counts.
func (c *ValueCache) SetInteger(val num.Int) {
	if c == nil {
		return
	}
	c.intVal = val
	c.intSet = true
	c.totalDigits = len(val.Digits)
	c.fractionDigits = 0
	c.digitsSet = true
}

// SetFloat32 stores one parsed float.
func (c *ValueCache) SetFloat32(val float32, class num.FloatClass) {
	if c == nil {
		return
	}
	c.float32Val = val
	c.float32Class = class
	c.float32Set = true
}

// SetFloat64 stores one parsed double.
func (c *ValueCache) SetFloat64(val float64, class num.FloatClass) {
	if c == nil {
		return
	}
	c.float64Val = val
	c.float64Class = class
	c.float64Set = true
}

// SetLength stores one previously computed length.
func (c *ValueCache) SetLength(length int) {
	if c == nil {
		return
	}
	c.length = length
	c.lengthSet = true
}

// SetListLength stores the item count for one canonicalized list value.
func (c *ValueCache) SetListLength(count int) {
	c.SetLength(count)
}

// Decimal returns one parsed decimal, reusing cached state when present.
func (c *ValueCache) Decimal(canonical []byte) (num.Dec, error) {
	if c != nil && c.decSet {
		return c.decVal, nil
	}
	val, perr := num.ParseDec(canonical)
	if perr != nil {
		return num.Dec{}, xsderrors.Invalid("invalid decimal")
	}
	if c != nil {
		c.decVal = val
		c.decSet = true
	}
	return val, nil
}

// Integer returns one parsed integer, reusing cached state when present.
func (c *ValueCache) Integer(canonical []byte) (num.Int, error) {
	if c != nil && c.intSet {
		return c.intVal, nil
	}
	val, perr := num.ParseInt(canonical)
	if perr != nil {
		return num.Int{}, xsderrors.Invalid("invalid integer")
	}
	if c != nil {
		c.intVal = val
		c.intSet = true
	}
	return val, nil
}

// Float32 returns one parsed float, reusing cached state when present.
func (c *ValueCache) Float32(canonical []byte) (float32, num.FloatClass, error) {
	if c != nil && c.float32Set {
		return c.float32Val, c.float32Class, nil
	}
	val, class, perr := num.ParseFloat32(canonical)
	if perr != nil {
		return 0, num.FloatFinite, xsderrors.Invalid("invalid float")
	}
	if c != nil {
		c.float32Val = val
		c.float32Class = class
		c.float32Set = true
	}
	return val, class, nil
}

// Float64 returns one parsed double, reusing cached state when present.
func (c *ValueCache) Float64(canonical []byte) (float64, num.FloatClass, error) {
	if c != nil && c.float64Set {
		return c.float64Val, c.float64Class, nil
	}
	val, class, perr := num.ParseFloat(canonical, 64)
	if perr != nil {
		return 0, num.FloatFinite, xsderrors.Invalid("invalid double")
	}
	if c != nil {
		c.float64Val = val
		c.float64Class = class
		c.float64Set = true
	}
	return val, class, nil
}

// Length returns the facet length for one normalized lexical value.
func (c *ValueCache) Length(kind runtime.ValidatorKind, normalized []byte) (int, error) {
	if c != nil && c.lengthSet {
		return c.length, nil
	}

	var length int
	switch kind {
	case runtime.VList:
		length = countListItems(normalized)
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary(normalized)
		if err != nil {
			return 0, xsderrors.Invalid("invalid hexBinary")
		}
		length = len(decoded)
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary(normalized)
		if err != nil {
			return 0, xsderrors.Invalid("invalid base64Binary")
		}
		length = len(decoded)
	default:
		length = utf8.RuneCount(normalized)
	}
	if c != nil {
		c.length = length
		c.lengthSet = true
	}
	return length, nil
}

// DigitCounts returns totalDigits and fractionDigits for one canonical numeric value.
func (c *ValueCache) DigitCounts(kind runtime.ValidatorKind, canonical []byte) (int, int, error) {
	if c != nil && c.digitsSet {
		return c.totalDigits, c.fractionDigits, nil
	}

	total := 0
	fraction := 0
	switch kind {
	case runtime.VDecimal:
		val, err := c.Decimal(canonical)
		if err != nil {
			return 0, 0, err
		}
		total = len(val.Coef)
		fraction = int(val.Scale)
	case runtime.VInteger:
		val, err := c.Integer(canonical)
		if err != nil {
			return 0, 0, err
		}
		total = len(val.Digits)
	default:
		return 0, 0, xsderrors.Invalid("digits facet not applicable")
	}
	if c != nil {
		c.totalDigits = total
		c.fractionDigits = fraction
		c.digitsSet = true
	}
	return total, fraction, nil
}

func countListItems(normalized []byte) int {
	count := 0
	inItem := false
	for _, b := range normalized {
		if isXMLWhitespaceByte(b) {
			inItem = false
			continue
		}
		if !inItem {
			count++
			inItem = true
		}
	}
	return count
}

func isXMLWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
