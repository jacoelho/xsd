package value

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

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
func CanonicalDateTimeString(value time.Time, kind string, tzKind TimezoneKind) string {
	if tzKind == TZKnown {
		value = value.UTC()
	}
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	fraction := FormatFraction(value.Nanosecond())
	tz := ""
	if tzKind == TZKnown {
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
	return TimezoneKindFromLexical(lexical) != TZNone
}

func formatFloat(value float64, bits int) string {
	prec := -1
	if bits == 32 {
		return strconv.FormatFloat(value, 'E', prec, 32)
	}
	return strconv.FormatFloat(value, 'E', prec, 64)
}

// FormatFraction renders fractional seconds without trailing zeros.
func FormatFraction(nanos int) string {
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

// UpperHex renders src as uppercase hexadecimal into dst.
// It reuses dst when capacity allows and returns the resulting slice.
func UpperHex(dst, src []byte) []byte {
	size := hex.EncodedLen(len(src))
	if cap(dst) < size {
		dst = make([]byte, size)
	} else {
		dst = dst[:size]
	}
	hex.Encode(dst, src)
	for i := range dst {
		if dst[i] >= 'a' && dst[i] <= 'f' {
			dst[i] -= 'a' - 'A'
		}
	}
	return dst
}
