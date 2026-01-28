package value

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CanonicalDecimalBytes returns the canonical lexical form for a decimal value.
// The input may include XML whitespace; output uses optional sign, at least one
// digit before and after the decimal point, and no redundant zeros.
func CanonicalDecimalBytes(lexical, dst []byte) []byte {
	lexical = TrimXMLWhitespace(lexical)
	if len(lexical) == 0 {
		return dst[:0]
	}

	sign := byte(0)
	switch lexical[0] {
	case '+':
		lexical = lexical[1:]
	case '-':
		sign = '-'
		lexical = lexical[1:]
	}
	if len(lexical) == 0 {
		return dst[:0]
	}

	intPart := lexical
	fracPart := []byte{}
	if dot := indexByte(lexical, '.'); dot >= 0 {
		intPart = lexical[:dot]
		fracPart = lexical[dot+1:]
	}

	intPart = trimLeftZeros(intPart)
	if len(intPart) == 0 {
		intPart = []byte{'0'}
	}

	fracPart = trimRightZeros(fracPart)
	if len(fracPart) == 0 {
		fracPart = []byte{'0'}
	}

	if allZeros(intPart) && allZeros(fracPart) {
		sign = 0
		intPart = []byte{'0'}
		fracPart = []byte{'0'}
	}

	need := len(intPart) + 1 + len(fracPart)
	if sign != 0 {
		need++
	}
	out := dst[:0]
	if cap(out) < need {
		out = make([]byte, 0, need)
	}
	if sign != 0 {
		out = append(out, sign)
	}
	out = append(out, intPart...)
	out = append(out, '.')
	out = append(out, fracPart...)
	return out
}

// CanonicalFloat returns the canonical lexical form for float/double values.
func CanonicalFloat(value float64, bits int) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "INF"
	}
	if math.IsInf(value, -1) {
		return "-INF"
	}
	if value == 0 {
		return "0.0E0"
	}
	raw := formatFloat(value, bits)
	exponent := "0"
	mantissa := raw
	if e := strings.IndexByte(raw, 'E'); e >= 0 {
		mantissa = raw[:e]
		exponent = raw[e+1:]
	}
	if dot := strings.IndexByte(mantissa, '.'); dot == -1 {
		mantissa += ".0"
	} else {
		i := len(mantissa) - 1
		for i > dot+1 && mantissa[i] == '0' {
			i--
		}
		mantissa = mantissa[:i+1]
	}
	expVal, err := strconv.Atoi(exponent)
	if err != nil {
		return mantissa + "E" + exponent
	}
	return mantissa + "E" + strconv.Itoa(expVal)
}

// CanonicalDateTimeString formats a time value into the canonical lexical form
// for the given XML Schema temporal kind.
func CanonicalDateTimeString(value time.Time, kind string, hasTZ bool) string {
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	fraction := formatFraction(value.Nanosecond())
	tz := ""
	if hasTZ {
		tz = formatTimezone(value)
	}

	switch kind {
	case "dateTime":
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s%s", year, int(month), day, hour, minute, second, fraction, tz)
	case "date":
		return fmt.Sprintf("%04d-%02d-%02d%s", year, int(month), day, tz)
	case "time":
		return fmt.Sprintf("%02d:%02d:%02d%s%s", hour, minute, second, fraction, tz)
	case "gYearMonth":
		return fmt.Sprintf("%04d-%02d%s", year, int(month), tz)
	case "gYear":
		return fmt.Sprintf("%04d%s", year, tz)
	case "gMonthDay":
		return fmt.Sprintf("--%02d-%02d%s", int(month), day, tz)
	case "gMonth":
		return fmt.Sprintf("--%02d%s", int(month), tz)
	case "gDay":
		return fmt.Sprintf("---%02d%s", day, tz)
	default:
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s%s", year, int(month), day, hour, minute, second, fraction, tz)
	}
}

// HasTimezone reports whether a lexical temporal value includes a timezone.
func HasTimezone(lexical []byte) bool {
	lexical = TrimXMLWhitespace(lexical)
	if len(lexical) == 0 {
		return false
	}
	last := lexical[len(lexical)-1]
	if last == 'Z' {
		return true
	}
	if len(lexical) >= 6 {
		tz := lexical[len(lexical)-6:]
		return (tz[0] == '+' || tz[0] == '-') && tz[3] == ':'
	}
	return false
}

func formatFloat(value float64, bits int) string {
	prec := -1
	if bits == 32 {
		return strconv.FormatFloat(value, 'E', prec, 32)
	}
	return strconv.FormatFloat(value, 'E', prec, 64)
}

func formatFraction(nanos int) string {
	if nanos == 0 {
		return ""
	}
	frac := fmt.Sprintf("%09d", nanos)
	frac = strings.TrimRight(frac, "0")
	return "." + frac
}

func formatTimezone(value time.Time) string {
	_, offset := value.Zone()
	if offset == 0 {
		return "Z"
	}
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func allZeros(b []byte) bool {
	for i := range b {
		if b[i] != '0' {
			return false
		}
	}
	return true
}

func trimLeftZeros(b []byte) []byte {
	i := 0
	for i < len(b) && b[i] == '0' {
		i++
	}
	return b[i:]
}

func trimRightZeros(b []byte) []byte {
	j := len(b)
	for j > 0 && b[j-1] == '0' {
		j--
	}
	return b[:j]
}

func indexByte(b []byte, needle byte) int {
	for i := range b {
		if b[i] == needle {
			return i
		}
	}
	return -1
}
