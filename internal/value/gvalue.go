package value

import (
	"errors"
	"fmt"
	"strings"
)

// gValue is the value-space projection for xs:gYearMonth, xs:gYear,
// xs:gMonthDay, xs:gDay, and xs:gMonth.
type gValue struct {
	year    xsdYear
	instant xsdDateTimePoint
	tz      xsdTimezone
	month   int
	day     int
	kind    PrimitiveKind
}

// parseGValue parses s as one of the XML Schema g* primitive values.
func parseGValue(kind PrimitiveKind, s string) (gValue, error) {
	//nolint:exhaustive // Only the five g* primitives are valid; all others share one error.
	switch kind {
	case primitiveGYearMonth:
		return parseGYearMonthValue(s)
	case primitiveGYear:
		return parseGYearValue(s)
	case primitiveGMonthDay:
		return parseGMonthDayValue(s)
	case primitiveGDay:
		return parseGDayValue(s)
	case primitiveGMonth:
		return parseGMonthValue(s)
	default:
		return gValue{}, errors.New(msgInvalidGPrimitive)
	}
}

// CanonicalText returns the XML Schema canonical lexical form for v.
func (v gValue) CanonicalText() string {
	switch v.kind {
	case primitiveGYearMonth:
		return fmt.Sprintf("%s-%02d%s", formatYear(v.year), v.month, formatTimezoneSuffix(v.tz))
	case primitiveGYear:
		return formatYear(v.year) + formatTimezoneSuffix(v.tz)
	case primitiveGMonthDay:
		return fmt.Sprintf("--%02d-%02d%s", v.month, v.day, formatTimezoneSuffix(v.tz))
	case primitiveGDay:
		return fmt.Sprintf("---%02d%s", v.day, formatTimezoneSuffix(v.tz))
	case primitiveGMonth:
		return fmt.Sprintf("--%02d%s", v.month, formatTimezoneSuffix(v.tz))
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble,
		PrimitiveDuration, PrimitiveDateTime, primitiveTime, primitiveDate,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return ""
	default:
	}
	return ""
}

// identityCanonical returns the value-space identity projection for v. The
// lexical calendar fields are not sufficient here: a timezone can move a
// gMonthDay or gDay value across a calendar boundary while preserving the
// same normalized instant. Keep timezone presence separate because XML Schema
// treats a value with no timezone as unequal to one with a timezone, even when
// their normalized coordinates happen to match.
func (v gValue) identityCanonical() string {
	canonical := formatXSDDateTimePoint(v.instant)
	if v.tz.present {
		return canonical + "Z"
	}
	return canonical
}

// compareGValues compares g* values using the XML Schema partial order.
func compareGValues(a, b gValue) orderedFacetRelation {
	return compareTemporalValues(
		TemporalValue{instant: a.instant, hasTZ: a.tz.present},
		TemporalValue{instant: b.instant, hasTZ: b.tz.present},
	)
}

// equalGValues reports XML Schema equality for g* values, including
// timezone-presence equivalence.
func equalGValues(a, b gValue) bool {
	return a.tz.present == b.tz.present && compareXSDDateTimePoint(a.instant, b.instant) == 0
}

func parseGYearMonthValue(s string) (gValue, error) {
	year, next, err := parseXSDYear(s)
	if err != nil || next >= len(s) || s[next] != '-' {
		if err != nil {
			return gValue{}, err
		}
		return gValue{}, errors.New(msgInvalidGYearMonth)
	}
	month, next, ok := parseTwoDateDigits(s, next+1)
	if !ok || month < 1 || month > 12 {
		return gValue{}, errors.New(msgInvalidGYearMonth)
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gYearMonth")
	if err != nil {
		return gValue{}, err
	}
	return newGValue(primitiveGYearMonth, year, month, 1, tz), nil
}

func parseGYearValue(s string) (gValue, error) {
	year, next, err := parseXSDYear(s)
	if err != nil {
		return gValue{}, err
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gYear")
	if err != nil {
		return gValue{}, err
	}
	return newGValue(primitiveGYear, year, 1, 1, tz), nil
}

func parseGMonthDayValue(s string) (gValue, error) {
	if !strings.HasPrefix(s, "--") {
		return gValue{}, errors.New(msgInvalidGMonthDay)
	}
	month, next, ok := parseTwoDateDigits(s, 2)
	if !ok || next >= len(s) || s[next] != '-' {
		return gValue{}, errors.New(msgInvalidGMonthDay)
	}
	day, next, ok := parseTwoDateDigits(s, next+1)
	if !ok || month < 1 || month > 12 || day < 1 || day > daysInMonthForLeap(month, true) {
		return gValue{}, errors.New(msgInvalidGMonthDay)
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gMonthDay")
	if err != nil {
		return gValue{}, err
	}
	return newGValue(primitiveGMonthDay, xsdYear{digits: "2000"}, month, day, tz), nil
}

func parseGDayValue(s string) (gValue, error) {
	if !strings.HasPrefix(s, "---") {
		return gValue{}, errors.New(msgInvalidGDay)
	}
	day, next, ok := parseTwoDateDigits(s, 3)
	if !ok || day < 1 || day > 31 {
		return gValue{}, errors.New(msgInvalidGDay)
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gDay")
	if err != nil {
		return gValue{}, err
	}
	return newGValue(primitiveGDay, xsdYear{digits: "2000"}, 1, day, tz), nil
}

func parseGMonthValue(s string) (gValue, error) {
	if !strings.HasPrefix(s, "--") {
		return gValue{}, errors.New(msgInvalidGMonth)
	}
	month, next, ok := parseTwoDateDigits(s, 2)
	if !ok || month < 1 || month > 12 {
		return gValue{}, errors.New(msgInvalidGMonth)
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gMonth")
	if err != nil {
		return gValue{}, err
	}
	return newGValue(primitiveGMonth, xsdYear{digits: "2000"}, month, 1, tz), nil
}

func newGValue(kind PrimitiveKind, year xsdYear, month, day int, tz xsdTimezone) gValue {
	instant := xsdDateTimePoint{year: year, month: month, day: day}
	if tz.present {
		instant = addMinutes(instant, -tz.minutes)
	}
	return gValue{instant: instant, tz: tz, year: year, kind: kind, month: month, day: day}
}

func formatTimezoneSuffix(tz xsdTimezone) string {
	if !tz.present {
		return ""
	}
	return formatTimezone(tz.minutes)
}
