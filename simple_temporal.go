package xsd

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func parseXSDDate(s string) (time.Time, error) {
	if err := rejectUnsupportedYear(s); err != nil {
		return time.Time{}, err
	}
	layouts := []string{"2006-01-02", "2006-01-02Z07:00"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date")
}

func parseXSDDateTime(s string) (time.Time, error) {
	if err := rejectUnsupportedYear(s); err != nil {
		return time.Time{}, err
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid dateTime")
}

func parseXSDDateTimeCanonical(s string) (string, error) {
	if err := rejectUnsupportedYear(s); err != nil {
		return "", err
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return formatXSDDateTime(t.UTC(), true), nil
	}
	layouts := []string{"2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return formatXSDDateTime(t, false), nil
		}
	}
	return "", fmt.Errorf("invalid dateTime")
}

func formatXSDDateTime(t time.Time, withZone bool) string {
	out := t.Format("2006-01-02T15:04:05")
	if t.Nanosecond() != 0 {
		frac := fmt.Sprintf(".%09d", t.Nanosecond())
		out += strings.TrimRight(frac, "0")
	}
	if withZone {
		out += "Z"
	}
	return out
}

type xsdTemporalValue struct {
	instant time.Time
	hasTZ   bool
}

const xsdTimezoneUncertainty = 14 * time.Hour

func parseXSDTemporalValue(kind primitiveKind, s string) (xsdTemporalValue, error) {
	switch kind {
	case primDate:
		t, err := parseXSDDate(s)
		return xsdTemporalValue{instant: t, hasTZ: hasXSDTimezone(s)}, err
	case primDateTime:
		t, err := parseXSDDateTime(s)
		return xsdTemporalValue{instant: t, hasTZ: hasXSDTimezone(s)}, err
	default:
		return xsdTemporalValue{}, fmt.Errorf("not a temporal type")
	}
}

func hasXSDTimezone(s string) bool {
	if strings.HasSuffix(s, "Z") {
		return true
	}
	if len(s) < 6 {
		return false
	}
	tz := s[len(s)-6:]
	return (tz[0] == '+' || tz[0] == '-') && tz[3] == ':'
}

func compareXSDTemporal(a, b xsdTemporalValue) (int, bool) {
	if a.hasTZ == b.hasTZ {
		return compareTime(a.instant, b.instant), true
	}
	if !a.hasTZ {
		lo := a.instant.Add(-xsdTimezoneUncertainty)
		hi := a.instant.Add(xsdTimezoneUncertainty)
		if hi.Before(b.instant) {
			return -1, true
		}
		if lo.After(b.instant) {
			return 1, true
		}
		return 0, false
	}
	lo := b.instant.Add(-xsdTimezoneUncertainty)
	hi := b.instant.Add(xsdTimezoneUncertainty)
	if a.instant.Before(lo) {
		return -1, true
	}
	if a.instant.After(hi) {
		return 1, true
	}
	return 0, false
}

func compareTime(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}

const dayNanos int64 = 24 * 60 * 60 * 1e9

var xsdTimeRE = regexp.MustCompile(`^([0-9]{2}):([0-9]{2}):([0-9]{2})(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})?$`)

type xsdTimeValue struct {
	nanos int64
	hasTZ bool
}

func parseXSDTimeCanonical(s string) (string, error) {
	v, err := parseXSDTimeValue(s)
	if err != nil {
		return "", err
	}
	return formatXSDTime(v), nil
}

func parseXSDTimeValue(s string) (xsdTimeValue, error) {
	nanos, hasTZ, err := parseXSDTimeRaw(s)
	if err != nil {
		return xsdTimeValue{}, err
	}
	return xsdTimeValue{nanos: modDay(nanos), hasTZ: hasTZ}, nil
}

func parseXSDTimeNanos(s string) (int64, error) {
	value, err := parseXSDTimeValue(s)
	if err != nil {
		return 0, err
	}
	return value.nanos, nil
}

func parseXSDTimeRaw(s string) (int64, bool, error) {
	m := xsdTimeRE.FindStringSubmatch(s)
	if m == nil {
		return 0, false, fmt.Errorf("invalid time")
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	second, _ := strconv.Atoi(m[3])
	if hour > 23 || minute > 59 || second > 59 && (hour != 23 || minute != 59 || second != 60) {
		return 0, false, fmt.Errorf("invalid time")
	}
	frac, err := parseFractionalNanos(m[4])
	if err != nil {
		return 0, false, err
	}
	nanos := int64(((hour*60+minute)*60+second))*1e9 + frac
	hasTZ := m[5] != ""
	if hasTZ {
		offset, err := parseTimezoneOffsetNanos(m[5])
		if err != nil {
			return 0, false, err
		}
		nanos -= offset
	}
	return nanos, hasTZ, nil
}

func parseXSDTimeRawNanos(s string) (int64, error) {
	nanos, _, err := parseXSDTimeRaw(s)
	return nanos, err
}

func parseFractionalNanos(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	digits := strings.TrimPrefix(s, ".")
	if len(digits) > 9 {
		return 0, fmt.Errorf("invalid time")
	}
	for len(digits) < 9 {
		digits += "0"
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("invalid time")
	}
	return int64(v), nil
}

func parseTimezoneOffsetNanos(s string) (int64, error) {
	if s == "Z" {
		return 0, nil
	}
	sign := int64(1)
	if s[0] == '-' {
		sign = -1
	}
	hour, err1 := strconv.Atoi(s[1:3])
	minute, err2 := strconv.Atoi(s[4:6])
	if err1 != nil || err2 != nil || hour > 14 || minute > 59 || (hour == 14 && minute != 0) {
		return 0, fmt.Errorf("invalid timezone")
	}
	return sign * int64(hour*60+minute) * 60 * 1e9, nil
}

func modDay(n int64) int64 {
	n %= dayNanos
	if n < 0 {
		n += dayNanos
	}
	return n
}

func formatXSDTime(v xsdTimeValue) string {
	n := v.nanos
	hour := n / (60 * 60 * 1e9)
	n %= 60 * 60 * 1e9
	minute := n / (60 * 1e9)
	n %= 60 * 1e9
	second := n / 1e9
	nano := n % 1e9
	s := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	if nano != 0 {
		frac := fmt.Sprintf("%09d", nano)
		s += "." + strings.TrimRight(frac, "0")
	}
	if v.hasTZ {
		s += "Z"
	}
	return s
}

var durationRE = regexp.MustCompile(`^-?P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

func parseXSDDuration(s string) error {
	_, err := parseXSDDurationValue(s)
	return err
}

type xsdDurationValue struct {
	months int64
	nanos  float64
}

func parseXSDDurationValue(s string) (xsdDurationValue, error) {
	m := durationRE.FindStringSubmatch(s)
	if m == nil {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	seen := false
	for i := 1; i < len(m); i++ {
		seen = seen || m[i] != ""
	}
	if !seen {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	if strings.Contains(s, "T") && m[4] == "" && m[5] == "" && m[6] == "" {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	part := func(i int) (int64, error) {
		if m[i] == "" {
			return 0, nil
		}
		v, err := strconv.ParseInt(m[i], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration")
		}
		return v, nil
	}
	years, err := part(1)
	if err != nil {
		return xsdDurationValue{}, err
	}
	months, err := part(2)
	if err != nil {
		return xsdDurationValue{}, err
	}
	days, err := part(3)
	if err != nil {
		return xsdDurationValue{}, err
	}
	hours, err := part(4)
	if err != nil {
		return xsdDurationValue{}, err
	}
	minutes, err := part(5)
	if err != nil {
		return xsdDurationValue{}, err
	}
	seconds, err := durationSecondNanos(m[6])
	if err != nil {
		return xsdDurationValue{}, err
	}
	out := xsdDurationValue{
		months: years*12 + months,
		nanos:  (((float64(days)*24+float64(hours))*60+float64(minutes))*60)*1e9 + seconds,
	}
	if strings.HasPrefix(s, "-") {
		out.months = -out.months
		out.nanos = -out.nanos
	}
	return out, nil
}

func durationSecondNanos(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration")
	}
	return v * 1e9, nil
}

func parseXSDGYearMonth(s string) error {
	_, err := parseXSDGYearMonthValue(s)
	return err
}

func parseXSDGYearMonthValue(s string) (int, error) {
	main := stripTimezone(s)
	parts := strings.Split(main, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid gYearMonth")
	}
	if err := validateYear(parts[0]); err != nil {
		return 0, err
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("invalid gYearMonth")
	}
	if err := validateTimezoneSuffix(s); err != nil {
		return 0, err
	}
	year, _ := strconv.Atoi(parts[0])
	return year*12 + month, nil
}

func parseXSDGYear(s string) error {
	_, err := parseXSDGYearValue(s)
	return err
}

func parseXSDGYearValue(s string) (int, error) {
	main := stripTimezone(s)
	if strings.Contains(main, "-") || strings.Contains(main, ":") {
		return 0, fmt.Errorf("invalid gYear")
	}
	if err := validateYear(main); err != nil {
		return 0, err
	}
	if err := validateTimezoneSuffix(s); err != nil {
		return 0, err
	}
	year, _ := strconv.Atoi(main)
	return year, nil
}

func parseXSDGMonthDay(s string) error {
	_, err := parseXSDGMonthDayValue(s)
	return err
}

func parseXSDGMonthDayValue(s string) (int, error) {
	main := stripTimezone(s)
	if !strings.HasPrefix(main, "--") {
		return 0, fmt.Errorf("invalid gMonthDay")
	}
	parts := strings.Split(strings.TrimPrefix(main, "--"), "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid gMonthDay")
	}
	month, err1 := strconv.Atoi(parts[0])
	day, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || month < 1 || month > 12 || day < 1 || day > maxGMonthDayOfMonth(month) {
		return 0, fmt.Errorf("invalid gMonthDay")
	}
	if err := validateTimezoneSuffix(s); err != nil {
		return 0, err
	}
	return month*32 + day, nil
}

func parseXSDGDay(s string) error {
	_, err := parseXSDGDayValue(s)
	return err
}

func parseXSDGDayValue(s string) (int, error) {
	main := stripTimezone(s)
	if !strings.HasPrefix(main, "---") {
		return 0, fmt.Errorf("invalid gDay")
	}
	day, err := strconv.Atoi(strings.TrimPrefix(main, "---"))
	if err != nil || day < 1 || day > 31 {
		return 0, fmt.Errorf("invalid gDay")
	}
	if err := validateTimezoneSuffix(s); err != nil {
		return 0, err
	}
	return day, nil
}

func parseXSDGMonth(s string) error {
	_, err := parseXSDGMonthValue(s)
	return err
}

func parseXSDGMonthValue(s string) (int, error) {
	main := stripTimezone(s)
	if !strings.HasPrefix(main, "--") || strings.HasPrefix(main, "---") {
		return 0, fmt.Errorf("invalid gMonth")
	}
	month, err := strconv.Atoi(strings.TrimPrefix(main, "--"))
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("invalid gMonth")
	}
	if err := validateTimezoneSuffix(s); err != nil {
		return 0, err
	}
	return month, nil
}

func stripTimezone(s string) string {
	if before, ok := strings.CutSuffix(s, "Z"); ok {
		return before
	}
	if len(s) >= 6 {
		tz := s[len(s)-6:]
		if (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' {
			return s[:len(s)-6]
		}
	}
	return s
}

func validateTimezoneSuffix(s string) error {
	if len(s) < 6 {
		return nil
	}
	tz := s[len(s)-6:]
	if (tz[0] != '+' && tz[0] != '-') || tz[3] != ':' {
		return nil
	}
	hour, err1 := strconv.Atoi(tz[1:3])
	minute, err2 := strconv.Atoi(tz[4:6])
	if err1 != nil || err2 != nil || hour > 14 || minute > 59 || (hour == 14 && minute != 0) {
		return fmt.Errorf("invalid timezone")
	}
	return nil
}

func validateYear(s string) error {
	if strings.HasPrefix(s, "+") {
		return fmt.Errorf("invalid date/time")
	}
	if strings.HasPrefix(s, "-") {
		return unsupported(ErrUnsupportedDateTime, "date/time BCE years are not supported")
	}
	if len(s) < 4 {
		return fmt.Errorf("invalid date/time")
	}
	if len(s) > 4 && s[0] == '0' {
		return fmt.Errorf("invalid date/time")
	}
	if s == "0000" {
		return fmt.Errorf("invalid date/time")
	}
	year, err := strconv.Atoi(s)
	if err != nil || year < 1 {
		return fmt.Errorf("invalid date/time")
	}
	return nil
}

func rejectUnsupportedYear(s string) error {
	if strings.HasPrefix(s, "+") {
		return fmt.Errorf("invalid date/time")
	}
	if strings.HasPrefix(s, "-") {
		return unsupported(ErrUnsupportedDateTime, "date/time BCE years are not supported")
	}
	yearEnd := strings.IndexByte(s, '-')
	if yearEnd < 0 {
		return fmt.Errorf("invalid date/time")
	}
	if yearEnd < 4 {
		return fmt.Errorf("invalid date/time")
	}
	if yearEnd > 4 {
		return unsupported(ErrUnsupportedDateTime, "date/time years outside 0001-9999 are not supported")
	}
	return validateYear(s[:yearEnd])
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

func applyDurationBounds(f facetSet, norm string) error {
	value, err := parseXSDDurationValue(norm)
	if err != nil {
		return err
	}
	cmpLit := func(l *compiledLiteral) (xsdDurationValue, bool, error) {
		if l == nil {
			return xsdDurationValue{}, false, nil
		}
		v, err := parseXSDDurationValue(l.Canonical)
		return v, true, err
	}
	if lit, ok, err := cmpLit(f.MinInclusive); err != nil {
		return err
	} else if ok {
		if cmp, comparable := compareXSDDuration(value, lit); !comparable || cmp < 0 {
			return fmt.Errorf("minInclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MaxInclusive); err != nil {
		return err
	} else if ok {
		if cmp, comparable := compareXSDDuration(value, lit); !comparable || cmp > 0 {
			return fmt.Errorf("maxInclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MinExclusive); err != nil {
		return err
	} else if ok {
		if cmp, comparable := compareXSDDuration(value, lit); !comparable || cmp <= 0 {
			return fmt.Errorf("minExclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MaxExclusive); err != nil {
		return err
	} else if ok {
		if cmp, comparable := compareXSDDuration(value, lit); !comparable || cmp >= 0 {
			return fmt.Errorf("maxExclusive facet failed")
		}
	}
	return nil
}

func validateDurationFacetBounds(f facetSet) error {
	lower, lowerExclusive, hasLower, err := durationLowerBound(f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := durationUpperBound(f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	cmp, comparable := compareXSDDuration(lower, upper)
	if comparable && (cmp > 0 || cmp == 0 && (lowerExclusive || upperExclusive)) {
		return fmt.Errorf("duration lower bound cannot exceed upper bound")
	}
	return nil
}

func durationLowerBound(f facetSet) (xsdDurationValue, bool, bool, error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
		cmp, comparable := compareXSDDuration(other, out)
		return comparable && cmp >= 0
	})
}

func durationUpperBound(f facetSet) (xsdDurationValue, bool, bool, error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
		cmp, comparable := compareXSDDuration(other, out)
		return comparable && cmp <= 0
	})
}

func compareXSDDuration(a, b xsdDurationValue) (int, bool) {
	months := cmpInt64(a.months, b.months)
	nanos := cmpFloat64(a.nanos, b.nanos)
	if months == 0 {
		return nanos, true
	}
	if nanos == 0 || months == nanos {
		return months, true
	}
	refs := []time.Time{
		time.Date(1696, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1697, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1903, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1903, 7, 1, 0, 0, 0, 0, time.UTC),
	}
	relation := 0
	for _, ref := range refs {
		ta, ok := addXSDDurationToTime(ref, a)
		if !ok {
			return 0, false
		}
		tb, ok := addXSDDurationToTime(ref, b)
		if !ok {
			return 0, false
		}
		cmp := ta.Compare(tb)
		if cmp == 0 {
			continue
		}
		if relation != 0 && relation != cmp {
			return 0, false
		}
		relation = cmp
	}
	return relation, true
}

func addXSDDurationToTime(t time.Time, d xsdDurationValue) (time.Time, bool) {
	t, ok := addXSDMonthsToTime(t, d.months)
	if !ok {
		return time.Time{}, false
	}
	if d.nanos == 0 {
		return t, true
	}
	if math.Abs(d.nanos) > float64(math.MaxInt64) {
		return time.Time{}, false
	}
	return t.Add(time.Duration(math.Round(d.nanos))), true
}

func addXSDMonthsToTime(t time.Time, months int64) (time.Time, bool) {
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	total := int64(year)*12 + int64(month) - 1 + months
	newYear := total / 12
	newMonth := total % 12
	if newMonth < 0 {
		newMonth += 12
		newYear--
	}
	if newYear > int64(math.MaxInt) || newYear < int64(math.MinInt) {
		return time.Time{}, false
	}
	monthValue := time.Month(newMonth + 1)
	maxDay := daysInSpecificMonth(int(newYear), monthValue)
	if day > maxDay {
		day = maxDay
	}
	return time.Date(int(newYear), monthValue, day, hour, minute, sec, t.Nanosecond(), time.UTC), true
}

func daysInSpecificMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func applyGDayBounds(f facetSet, norm string) error {
	return applyIntOrderedBounds(f, norm, parseXSDGDayValue)
}

func validateGDayFacetBounds(f facetSet) error {
	lower, lowerExclusive, hasLower, err := gDayLowerBound(f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := gDayUpperBound(f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	if lower > upper || lower == upper && (lowerExclusive || upperExclusive) {
		return fmt.Errorf("gDay lower bound cannot exceed upper bound")
	}
	return nil
}

func gDayLowerBound(f facetSet) (int, bool, bool, error) {
	return intLowerBound(f, parseXSDGDayValue)
}

func gDayUpperBound(f facetSet) (int, bool, bool, error) {
	return intUpperBound(f, parseXSDGDayValue)
}

func applyGMonthDayBounds(f facetSet, norm string) error {
	return applyIntOrderedBounds(f, norm, parseXSDGMonthDayValue)
}

func validateGMonthDayFacetBounds(f facetSet) error {
	lower, lowerExclusive, hasLower, err := gMonthDayLowerBound(f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := gMonthDayUpperBound(f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	if lower > upper || lower == upper && (lowerExclusive || upperExclusive) {
		return fmt.Errorf("gMonthDay lower bound cannot exceed upper bound")
	}
	return nil
}

func gMonthDayLowerBound(f facetSet) (int, bool, bool, error) {
	return intLowerBound(f, parseXSDGMonthDayValue)
}

func gMonthDayUpperBound(f facetSet) (int, bool, bool, error) {
	return intUpperBound(f, parseXSDGMonthDayValue)
}

func applyGMonthBounds(f facetSet, norm string) error {
	return applyIntOrderedBounds(f, norm, parseXSDGMonthValue)
}

func validateGMonthFacetBounds(f facetSet) error {
	return validateIntOrderedFacetBounds("gMonth", f, parseXSDGMonthValue)
}

func applyGYearMonthBounds(f facetSet, norm string) error {
	return applyIntOrderedBounds(f, norm, parseXSDGYearMonthValue)
}

func validateGYearMonthFacetBounds(f facetSet) error {
	return validateIntOrderedFacetBounds("gYearMonth", f, parseXSDGYearMonthValue)
}

func applyGYearBounds(f facetSet, norm string) error {
	return applyIntOrderedBounds(f, norm, parseXSDGYearValue)
}

func validateGYearFacetBounds(f facetSet) error {
	return validateIntOrderedFacetBounds("gYear", f, parseXSDGYearValue)
}

func applyTemporalBounds(kind primitiveKind, f facetSet, norm string) error {
	if kind == primTime {
		return applyTimeBounds(f, norm)
	}
	value, err := parseXSDTemporalValue(kind, norm)
	if err != nil {
		return err
	}
	cmpLit := func(l *compiledLiteral) (xsdTemporalValue, bool, error) {
		if l == nil {
			return xsdTemporalValue{}, false, nil
		}
		t, err := parseXSDTemporalValue(kind, l.Canonical)
		return t, true, err
	}
	if lit, ok, err := cmpLit(f.MinInclusive); err != nil {
		return err
	} else if ok {
		cmp, comparable := compareXSDTemporal(value, lit)
		if !comparable || cmp < 0 {
			return fmt.Errorf("minInclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MaxInclusive); err != nil {
		return err
	} else if ok {
		cmp, comparable := compareXSDTemporal(value, lit)
		if !comparable || cmp > 0 {
			return fmt.Errorf("maxInclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MinExclusive); err != nil {
		return err
	} else if ok {
		cmp, comparable := compareXSDTemporal(value, lit)
		if !comparable || cmp <= 0 {
			return fmt.Errorf("minExclusive facet failed")
		}
	}
	if lit, ok, err := cmpLit(f.MaxExclusive); err != nil {
		return err
	} else if ok {
		cmp, comparable := compareXSDTemporal(value, lit)
		if !comparable || cmp >= 0 {
			return fmt.Errorf("maxExclusive facet failed")
		}
	}
	return nil
}

func validateTemporalFacetBounds(kind primitiveKind, f facetSet) error {
	if kind == primTime {
		return validateTimeFacetBounds(f)
	}
	lower, lowerExclusive, hasLower, err := temporalLowerBound(kind, f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := temporalUpperBound(kind, f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	cmp, comparable := compareXSDTemporal(lower, upper)
	if comparable && (cmp > 0 || cmp == 0 && (lowerExclusive || upperExclusive)) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func temporalLowerBound(kind primitiveKind, f facetSet) (xsdTemporalValue, bool, bool, error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out xsdTemporalValue) bool {
		cmp, comparable := compareXSDTemporal(other, out)
		return comparable && cmp >= 0
	})
}

func temporalUpperBound(kind primitiveKind, f facetSet) (xsdTemporalValue, bool, bool, error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out xsdTemporalValue) bool {
		cmp, comparable := compareXSDTemporal(other, out)
		return comparable && cmp <= 0
	})
}

func validateTimeFacetBounds(f facetSet) error {
	lower, lowerExclusive, hasLower, err := timeLowerBound(f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := timeUpperBound(f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	if lower > upper || lower == upper && (lowerExclusive || upperExclusive) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func validateTimeFacetRestriction(f, base facetSet, step orderedFacetStep) error {
	baseLower, baseLowerExclusive, hasLower, err := timeRawLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, baseUpperExclusive, hasUpper, err := timeRawUpperBound(base)
	if err != nil {
		return err
	}
	if step.minInclusive && hasLower {
		if err := validateTimeLowerRestriction("minInclusive", f.MinInclusive, false, baseLower, baseLowerExclusive); err != nil {
			return err
		}
	}
	if step.minExclusive && hasLower {
		if err := validateTimeLowerRestriction("minExclusive", f.MinExclusive, true, baseLower, baseLowerExclusive); err != nil {
			return err
		}
	}
	if step.maxInclusive && hasUpper {
		if err := validateTimeUpperRestriction("maxInclusive", f.MaxInclusive, false, baseUpper, baseUpperExclusive); err != nil {
			return err
		}
	}
	if step.maxExclusive && hasUpper {
		if err := validateTimeUpperRestriction("maxExclusive", f.MaxExclusive, true, baseUpper, baseUpperExclusive); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeLowerRestriction(name string, lit *compiledLiteral, exclusive bool, base int64, baseExclusive bool) error {
	if lit == nil {
		return nil
	}
	value, _, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	if value < base || value == base && !exclusive && baseExclusive {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateTimeUpperRestriction(name string, lit *compiledLiteral, exclusive bool, base int64, baseExclusive bool) error {
	if lit == nil {
		return nil
	}
	value, _, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	if value > base || value == base && !exclusive && baseExclusive {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

func timeRawLowerBound(f facetSet) (int64, bool, bool, error) {
	return facetBoundLexical(f.MinInclusive, f.MinExclusive, parseXSDTimeRawNanos, func(other, out int64) bool { return other >= out })
}

func timeRawUpperBound(f facetSet) (int64, bool, bool, error) {
	return facetBoundLexical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeRawNanos, func(other, out int64) bool { return other <= out })
}

func timeLowerBound(f facetSet) (int64, bool, bool, error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parseXSDTimeNanos, func(other, out int64) bool { return other >= out })
}

func timeUpperBound(f facetSet) (int64, bool, bool, error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeNanos, func(other, out int64) bool { return other <= out })
}

func applyTimeBounds(f facetSet, norm string) error {
	value, err := parseXSDTimeValue(norm)
	if err != nil {
		return err
	}
	cmpLit := func(l *compiledLiteral) (xsdTimeValue, bool, error) {
		if l == nil {
			return xsdTimeValue{}, false, nil
		}
		v, err := parseXSDTimeValue(l.Canonical)
		return v, true, err
	}
	if lit, ok, err := cmpLit(f.MinInclusive); err != nil {
		return err
	} else if ok && value.nanos < lit.nanos {
		return fmt.Errorf("minInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxInclusive); err != nil {
		return err
	} else if ok && value.nanos > lit.nanos {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MinExclusive); err != nil {
		return err
	} else if ok && value.nanos <= lit.nanos {
		return fmt.Errorf("minExclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxExclusive); err != nil {
		return err
	} else if ok && value.nanos >= lit.nanos {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}
