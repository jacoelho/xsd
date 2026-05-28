package xsd

import (
	"fmt"
	"strings"
)

//nolint:govet // Field order keeps raw fields and normalized instant grouped.
type xsdGValue struct {
	instant xsdDateTimePoint
	tz      xsdTimezone
	year    xsdYear
	month   int
	day     int
}

func parseXSDGYearMonthValue(s string) (xsdGValue, error) {
	year, next, err := parseXSDYear(s)
	if err != nil || next >= len(s) || s[next] != '-' {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gYearMonth")
	}
	month, next, ok := parseTwoDigits(s, next+1)
	if !ok || month < 1 || month > 12 {
		return xsdGValue{}, fmt.Errorf("invalid gYearMonth")
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gYearMonth")
	if err != nil {
		return xsdGValue{}, err
	}
	return newXSDGValue(year, month, 1, tz), nil
}

func parseXSDGYearValue(s string) (xsdGValue, error) {
	year, next, err := parseXSDYear(s)
	if err != nil {
		return xsdGValue{}, err
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gYear")
	if err != nil {
		return xsdGValue{}, err
	}
	return newXSDGValue(year, 1, 1, tz), nil
}

func parseXSDGMonthDayValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "--") {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	month, next, ok := parseTwoDigits(s, 2)
	if !ok || next >= len(s) || s[next] != '-' {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	day, next, ok := parseTwoDigits(s, next+1)
	if !ok || month < 1 || month > 12 || day < 1 || day > maxGMonthDayOfMonth(month) {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gMonthDay")
	if err != nil {
		return xsdGValue{}, err
	}
	return newXSDGValue(xsdYear{digits: "2000"}, month, day, tz), nil
}

func parseXSDGDayValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "---") {
		return xsdGValue{}, fmt.Errorf("invalid gDay")
	}
	day, next, ok := parseTwoDigits(s, 3)
	if !ok || day < 1 || day > 31 {
		return xsdGValue{}, fmt.Errorf("invalid gDay")
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gDay")
	if err != nil {
		return xsdGValue{}, err
	}
	return newXSDGValue(xsdYear{digits: "2000"}, 1, day, tz), nil
}

func parseXSDGMonthValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "--") {
		return xsdGValue{}, fmt.Errorf("invalid gMonth")
	}
	month, next, ok := parseTwoDigits(s, 2)
	if !ok || month < 1 || month > 12 {
		return xsdGValue{}, fmt.Errorf("invalid gMonth")
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "gMonth")
	if err != nil {
		return xsdGValue{}, err
	}
	return newXSDGValue(xsdYear{digits: "2000"}, month, 1, tz), nil
}

func newXSDGValue(year xsdYear, month, day int, tz xsdTimezone) xsdGValue {
	instant := xsdDateTimePoint{year: year, month: month, day: day}
	if tz.present {
		instant = addMinutes(instant, -tz.minutes)
	}
	return xsdGValue{instant: instant, tz: tz, year: year, month: month, day: day}
}

func compareXSDGValue(a, b xsdGValue) partialCompareResult {
	return compareXSDTemporal(
		xsdTemporalValue{instant: a.instant, hasTZ: a.tz.present},
		xsdTemporalValue{instant: b.instant, hasTZ: b.tz.present},
	)
}

func equalXSDGValue(a, b xsdGValue) bool {
	return a.tz.present == b.tz.present && compareXSDDateTimePoint(a.instant, b.instant) == 0
}

func formatXSDGYearMonth(v xsdGValue) string {
	return fmt.Sprintf("%s-%02d%s", formatYear(v.year), v.month, formatTimezoneSuffix(v.tz))
}

func formatXSDGYear(v xsdGValue) string {
	return formatYear(v.year) + formatTimezoneSuffix(v.tz)
}

func formatXSDGMonthDay(v xsdGValue) string {
	return fmt.Sprintf("--%02d-%02d%s", v.month, v.day, formatTimezoneSuffix(v.tz))
}

func formatXSDGDay(v xsdGValue) string {
	return fmt.Sprintf("---%02d%s", v.day, formatTimezoneSuffix(v.tz))
}

func formatXSDGMonth(v xsdGValue) string {
	return fmt.Sprintf("--%02d%s", v.month, formatTimezoneSuffix(v.tz))
}

func formatTimezoneSuffix(tz xsdTimezone) string {
	if !tz.present {
		return ""
	}
	return formatTimezone(tz.minutes)
}

func maxGMonthDayOfMonth(month int) int {
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		// xs:gMonthDay has no year; XSD compares it in an arbitrary leap year.
		return 29
	default:
		return 31
	}
}

// applyGValueBounds reuses parsed g-values when validation already has them.
func applyGValueBounds(kind primitiveKind, f facetSet, norm string, actual actualValue, parse func(string) (xsdGValue, error)) error {
	value := actual.G
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = parse(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(&f, value, parse, compareXSDGValue, actualGValueLiteral(kind))
}

// actualGValueLiteral trusts cached values only for the primitive being checked.
func actualGValueLiteral(kind primitiveKind) func(*compiledLiteral) (xsdGValue, bool) {
	return func(l *compiledLiteral) (xsdGValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.G, true
		}
		return xsdGValue{}, false
	}
}

func validateGValueFacetBounds(kind primitiveKind, f facetSet) error {
	name, parse, ok := gValueFacet(kind)
	if !ok {
		return nil
	}
	lower, err := facetBound(f.MinInclusive, f.MinExclusive, facetCanonical, parse, func(other, out xsdGValue) bool {
		return partialCompareForMinInclusive(compareXSDGValue(other, out))
	})
	if err != nil {
		return err
	}
	upper, err := facetBound(f.MaxInclusive, f.MaxExclusive, facetCanonical, parse, func(other, out xsdGValue) bool {
		return partialCompareForMaxInclusive(compareXSDGValue(other, out))
	})
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDGValue) {
		return fmt.Errorf("%s lower bound cannot exceed upper bound", name)
	}
	return nil
}

func gValueFacet(kind primitiveKind) (string, func(string) (xsdGValue, error), bool) {
	switch kind {
	case primGDay:
		return "gDay", parseXSDGDayValue, true
	case primGMonthDay:
		return "gMonthDay", parseXSDGMonthDayValue, true
	case primGMonth:
		return "gMonth", parseXSDGMonthValue, true
	case primGYearMonth:
		return "gYearMonth", parseXSDGYearMonthValue, true
	case primGYear:
		return "gYear", parseXSDGYearValue, true
	default:
		return "", nil, false
	}
}
