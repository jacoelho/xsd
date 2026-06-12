package xsd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const (
	daySeconds             = 24 * 60 * 60
	xsdTimezoneUncertainty = 14 * 60
	maxInt64Value          = int64(^uint64(0) >> 1)
	minInt64Value          = -maxInt64Value - 1
)

type xsdYear struct {
	digits string
	neg    bool
}

type xsdDateValue struct {
	point xsdDateTimePoint
	hasTZ bool
}

type xsdDateTimeValue struct {
	instant xsdDateTimePoint
	hasTZ   bool
}

func parseXSDDateTimeValue(s string) (xsdDateTimeValue, error) {
	date, next, err := parseXSDDatePart(s)
	if err != nil {
		return xsdDateTimeValue{}, err
	}
	if next >= len(s) || s[next] != 'T' {
		return xsdDateTimeValue{}, fmt.Errorf("invalid dateTime")
	}
	tm, err := parseXSDTimeParts(s[next+1:])
	if err != nil {
		return xsdDateTimeValue{}, fmt.Errorf("invalid dateTime")
	}
	dayOffset, second := divModDay(tm.rawSecond())
	point := xsdDateTimePoint{
		year:   date.year,
		month:  date.month,
		day:    date.day,
		second: second,
		frac:   tm.frac,
	}
	point = addDays(point, dayOffset)
	return xsdDateTimeValue{instant: point, hasTZ: tm.hasTZ()}, nil
}

func parseXSDDateValue(s string) (xsdDateValue, error) {
	date, next, err := parseXSDDatePart(s)
	if err != nil {
		return xsdDateValue{}, err
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "date")
	if err != nil {
		return xsdDateValue{}, err
	}
	point := xsdDateTimePoint{
		year:  date.year,
		month: date.month,
		day:   date.day,
	}
	if tz.present {
		point = addMinutes(point, -tz.minutes)
	}
	return xsdDateValue{point: point, hasTZ: tz.present}, nil
}

func validateDateNoOutputBytesFast(s []byte) (bool, error) {
	if len(s) != len("2006-01-02") || s[4] != '-' || s[7] != '-' {
		return false, nil
	}
	year, ok := parseFixedDigits(s[0:4])
	if !ok || year == 0 {
		return true, fmt.Errorf("invalid date/time")
	}
	month, ok := parseFixedDigits(s[5:7])
	if !ok {
		return true, fmt.Errorf("invalid date/time")
	}
	day, ok := parseFixedDigits(s[8:10])
	if !ok || month < 1 || month > 12 || day < 1 || day > daysInPositiveYearMonth(year, month) {
		return true, fmt.Errorf("invalid date/time")
	}
	return true, nil
}

func parseFixedDigits(s []byte) (int, bool) {
	n := 0
	for _, c := range s {
		if !isASCIIDigit(c) {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func daysInPositiveYearMonth(year, month int) int {
	leap := month == 2 && leapYearRule(year%400, year%100, year%4)
	return daysInMonthForLeap(month, leap)
}

type xsdDatePart struct {
	year       xsdYear
	month, day int
}

func parseXSDDatePart(s string) (xsdDatePart, int, error) {
	year, next, err := parseXSDYear(s)
	if err != nil {
		return xsdDatePart{}, 0, err
	}
	if next >= len(s) || s[next] != '-' {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	month, next, ok := parseTwoDigits(s, next+1)
	if !ok || next >= len(s) || s[next] != '-' {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	day, next, ok := parseTwoDigits(s, next+1)
	if !ok || month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	return xsdDatePart{year: year, month: month, day: day}, next, nil
}

func parseXSDYear(s string) (xsdYear, int, error) {
	i := 0
	if i >= len(s) {
		return xsdYear{}, 0, fmt.Errorf("invalid date/time")
	}
	neg := false
	if s[i] == '+' {
		return xsdYear{}, 0, fmt.Errorf("invalid date/time")
	}
	if s[i] == '-' {
		neg = true
		i++
	}
	start := i
	for i < len(s) && isASCIIDigit(s[i]) {
		i++
	}
	digits := s[start:i]
	if len(digits) < 4 {
		return xsdYear{}, 0, fmt.Errorf("invalid date/time")
	}
	if len(digits) > 4 && digits[0] == '0' {
		return xsdYear{}, 0, fmt.Errorf("invalid date/time")
	}
	if allZeroes(digits) {
		return xsdYear{}, 0, fmt.Errorf("invalid date/time")
	}
	return xsdYear{digits: canonicalYearDigits(digits), neg: neg}, i, nil
}

func allZeroes(s string) bool {
	for i := range len(s) {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

func canonicalYearDigits(s string) string {
	s = strings.TrimLeft(s, "0")
	if len(s) < 4 {
		return strings.Repeat("0", 4-len(s)) + s
	}
	return s
}

func formatYear(y xsdYear) string {
	if y.neg {
		return "-" + y.digits
	}
	return y.digits
}

func compareYear(a, b xsdYear) int {
	if a.neg != b.neg {
		if a.neg {
			return -1
		}
		return 1
	}
	amag := strings.TrimLeft(a.digits, "0")
	bmag := strings.TrimLeft(b.digits, "0")
	out := compareUnsignedDecimalText(amag, bmag)
	if a.neg {
		return -out
	}
	return out
}

func compareUnsignedDecimalText(a, b string) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}
	return cmp.Compare(a, b)
}

func nextYear(y xsdYear) xsdYear {
	if y.neg {
		if y.digits == "0001" {
			return xsdYear{digits: "0001"}
		}
		y.digits = canonicalYearDigits(subUnsignedDecimalOne(y.digits))
		return y
	}
	y.digits = canonicalYearDigits(addUnsignedDecimalOne(y.digits))
	return y
}

func prevYear(y xsdYear) xsdYear {
	if y.neg {
		y.digits = canonicalYearDigits(addUnsignedDecimalOne(y.digits))
		return y
	}
	if y.digits == "0001" {
		return xsdYear{digits: "0001", neg: true}
	}
	y.digits = canonicalYearDigits(subUnsignedDecimalOne(y.digits))
	return y
}

func addUnsignedDecimalOne(s string) string {
	b := []byte(s)
	for i := range slices.Backward(b) {
		if b[i] != '9' {
			b[i]++
			return string(b)
		}
		b[i] = '0'
	}
	return "1" + string(b)
}

func subUnsignedDecimalOne(s string) string {
	b := []byte(s)
	for i := range slices.Backward(b) {
		if b[i] != '0' {
			b[i]--
			break
		}
		b[i] = '9'
	}
	out := strings.TrimLeft(string(b), "0")
	if out == "" {
		return "0"
	}
	return out
}

func parseTwoDigits(s string, i int) (int, int, bool) {
	const n = 2
	if i+n > len(s) {
		return 0, 0, false
	}
	out := 0
	for j := range n {
		b := s[i+j]
		if !isASCIIDigit(b) {
			return 0, 0, false
		}
		out = out*10 + int(b-'0')
	}
	return out, i + n, true
}

func daysInMonth(year xsdYear, month int) int {
	leap := month == 2 && isLeapYear(year)
	return daysInMonthForLeap(month, leap)
}

func daysInMonthForLeap(month int, leap bool) int {
	switch month {
	case 2:
		if leap {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// isLeapYear names the XML Schema leap-year rule.
func isLeapYear(y xsdYear) bool {
	return leapYearRule(yearMod(y, 400), yearMod(y, 100), yearMod(y, 4))
}

// leapYearRule is the Gregorian leap-year rule over the year's residues,
// shared by plain int years and arbitrary-length xsdYears.
func leapYearRule(mod400, mod100, mod4 int) bool {
	return mod400 == 0 || mod4 == 0 && mod100 != 0
}

func yearMod(y xsdYear, m int) int {
	out := 0
	for i := range len(y.digits) {
		out = (out*10 + int(y.digits[i]-'0')) % m
	}
	if y.neg {
		out = (1 - out) % m
		if out < 0 {
			out += m
		}
	}
	return out
}

type xsdTimezone struct {
	minutes int
	present bool
}

func parseXSDTimezone(s string, i int) (xsdTimezone, int, error) {
	if i == len(s) {
		return xsdTimezone{}, i, nil
	}
	if s[i] == 'Z' {
		return xsdTimezone{present: true}, i + 1, nil
	}
	if s[i] != '+' && s[i] != '-' {
		return xsdTimezone{}, i, fmt.Errorf("invalid timezone")
	}
	if i+6 > len(s) || s[i+3] != ':' {
		return xsdTimezone{}, i, fmt.Errorf("invalid timezone")
	}
	hour, _, ok1 := parseTwoDigits(s, i+1)
	minute, _, ok2 := parseTwoDigits(s, i+4)
	if !ok1 || !ok2 || hour > 14 || minute > 59 || hour == 14 && minute != 0 {
		return xsdTimezone{}, i, fmt.Errorf("invalid timezone")
	}
	offset := hour*60 + minute
	if s[i] == '-' {
		offset = -offset
	}
	return xsdTimezone{minutes: offset, present: true}, i + 6, nil
}

func parseXSDTimezoneToEnd(s string, i int, label string) (xsdTimezone, error) {
	tz, next, err := parseXSDTimezone(s, i)
	if err != nil {
		return xsdTimezone{}, err
	}
	if next != len(s) {
		return xsdTimezone{}, fmt.Errorf("invalid %s", label)
	}
	return tz, nil
}

//nolint:govet // Field order keeps date/time components grouped for parser code.
type xsdDateTimePoint struct {
	year   xsdYear
	frac   string
	month  int
	day    int
	second int
}

func compareXSDDateTimePoint(a, b xsdDateTimePoint) int {
	if n := compareYear(a.year, b.year); n != 0 {
		return n
	}
	if n := cmp.Compare(a.month, b.month); n != 0 {
		return n
	}
	if n := cmp.Compare(a.day, b.day); n != 0 {
		return n
	}
	if n := cmp.Compare(a.second, b.second); n != 0 {
		return n
	}
	return compareFraction(a.frac, b.frac)
}

// addMinutes keeps timezone math in minutes at call sites.
func addMinutes(p xsdDateTimePoint, minutes int) xsdDateTimePoint {
	days, second := divModDay(p.second + minutes*60)
	p.second = second
	return addDays(p, days)
}

func divModDay(second int) (int, int) {
	days := second / daySeconds
	rest := second % daySeconds
	if rest < 0 {
		rest += daySeconds
		days--
	}
	return days, rest
}

func addDays(p xsdDateTimePoint, days int) xsdDateTimePoint {
	for days > 0 {
		p = nextDay(p)
		days--
	}
	for days < 0 {
		p = prevDay(p)
		days++
	}
	return p
}

func nextDay(p xsdDateTimePoint) xsdDateTimePoint {
	p.day++
	if p.day <= daysInMonth(p.year, p.month) {
		return p
	}
	p.day = 1
	p.month++
	if p.month <= 12 {
		return p
	}
	p.month = 1
	p.year = nextYear(p.year)
	return p
}

func prevDay(p xsdDateTimePoint) xsdDateTimePoint {
	p.day--
	if p.day >= 1 {
		return p
	}
	p.month--
	if p.month < 1 {
		p.month = 12
		p.year = prevYear(p.year)
	}
	p.day = daysInMonth(p.year, p.month)
	return p
}

func formatXSDDateTimePoint(p xsdDateTimePoint) string {
	hour := p.second / 3600
	minute := p.second / 60 % 60
	second := p.second % 60
	return fmt.Sprintf("%s-%02d-%02dT%02d:%02d:%02d%s",
		formatYear(p.year), p.month, p.day, hour, minute, second, formatFraction(p.frac))
}

func formatXSDDateTime(p xsdDateTimePoint, withZone bool) string {
	out := formatXSDDateTimePoint(p)
	if withZone {
		out += "Z"
	}
	return out
}

func formatXSDDate(v xsdDateValue) string {
	if !v.hasTZ {
		return fmt.Sprintf("%s-%02d-%02d", formatYear(v.point.year), v.point.month, v.point.day)
	}
	midpoint := addMinutes(v.point, 12*60)
	canonicalDate := xsdDateTimePoint{year: midpoint.year, month: midpoint.month, day: midpoint.day}
	offset := minutesBetween(v.point, canonicalDate)
	return fmt.Sprintf("%s-%02d-%02d%s", formatYear(canonicalDate.year), canonicalDate.month, canonicalDate.day, formatTimezone(offset))
}

func minutesBetween(start, end xsdDateTimePoint) int {
	days := 0
	date := xsdDateTimePoint{year: start.year, month: start.month, day: start.day}
	target := xsdDateTimePoint{year: end.year, month: end.month, day: end.day}
	for compareXSDDateTimePoint(date, target) < 0 {
		date = nextDay(date)
		days++
	}
	for compareXSDDateTimePoint(date, target) > 0 {
		date = prevDay(date)
		days--
	}
	return days*24*60 + end.second/60 - start.second/60
}

func formatTimezone(minutes int) string {
	if minutes == 0 {
		return "Z"
	}
	sign := '+'
	if minutes < 0 {
		sign = '-'
		minutes = -minutes
	}
	return fmt.Sprintf("%c%02d:%02d", sign, minutes/60, minutes%60)
}

type xsdTemporalValue struct {
	instant xsdDateTimePoint
	hasTZ   bool
}

type partialCompareResult uint8

const (
	partialCompareLess partialCompareResult = iota
	partialCompareEqual
	partialCompareGreater
	partialCompareIncomparable
)

func partialCompareFromInt(n int) partialCompareResult {
	switch {
	case n < 0:
		return partialCompareLess
	case n > 0:
		return partialCompareGreater
	default:
		return partialCompareEqual
	}
}

func partialCompareForMinInclusive(c partialCompareResult) bool {
	return c == partialCompareEqual || c == partialCompareGreater
}

func partialCompareForMaxInclusive(c partialCompareResult) bool {
	return c == partialCompareEqual || c == partialCompareLess
}

func partialCompareForMinExclusive(c partialCompareResult) bool {
	return c == partialCompareGreater
}

func partialCompareForMaxExclusive(c partialCompareResult) bool {
	return c == partialCompareLess
}

func partialFacetBoundsInvalid[T any](lower, upper orderedFacetBound[T], compare func(T, T) partialCompareResult) bool {
	if !lower.present() || !upper.present() {
		return false
	}
	c := compare(lower.value, upper.value)
	return c != partialCompareIncomparable &&
		(c == partialCompareGreater || c == partialCompareEqual && (lower.exclusive() || upper.exclusive()))
}

func parseXSDTemporalValue(kind primitiveKind, s string) (xsdTemporalValue, error) {
	switch kind {
	case primDate:
		v, err := parseXSDDateValue(s)
		return xsdTemporalValue{instant: v.point, hasTZ: v.hasTZ}, err
	case primDateTime:
		v, err := parseXSDDateTimeValue(s)
		return xsdTemporalValue(v), err
	default:
		return xsdTemporalValue{}, fmt.Errorf("not a temporal type")
	}
}

func compareXSDTemporal(a, b xsdTemporalValue) partialCompareResult {
	if a.hasTZ == b.hasTZ {
		return partialCompareFromInt(compareXSDDateTimePoint(a.instant, b.instant))
	}
	if !a.hasTZ {
		lo := addMinutes(a.instant, -xsdTimezoneUncertainty)
		hi := addMinutes(a.instant, xsdTimezoneUncertainty)
		if compareXSDDateTimePoint(hi, b.instant) < 0 {
			return partialCompareLess
		}
		if compareXSDDateTimePoint(lo, b.instant) > 0 {
			return partialCompareGreater
		}
		return partialCompareIncomparable
	}
	lo := addMinutes(b.instant, -xsdTimezoneUncertainty)
	hi := addMinutes(b.instant, xsdTimezoneUncertainty)
	if compareXSDDateTimePoint(a.instant, lo) < 0 {
		return partialCompareLess
	}
	if compareXSDDateTimePoint(a.instant, hi) > 0 {
		return partialCompareGreater
	}
	return partialCompareIncomparable
}

type xsdTimeValue struct {
	frac   string
	second int
	hasTZ  bool
}

type xsdTimeParts struct {
	frac   string
	tz     xsdTimezone
	hour   int
	minute int
	second int
}

func parseXSDTimeValue(s string) (xsdTimeValue, error) {
	tm, err := parseXSDTimeParts(s)
	if err != nil {
		return xsdTimeValue{}, err
	}
	_, second := divModDay(tm.rawSecond())
	return xsdTimeValue{second: second, frac: tm.frac, hasTZ: tm.hasTZ()}, nil
}

func parseXSDTimeRaw(s string) (xsdTimeValue, error) {
	tm, err := parseXSDTimeParts(s)
	if err != nil {
		return xsdTimeValue{}, err
	}
	return xsdTimeValue{second: tm.rawSecond(), frac: tm.frac, hasTZ: tm.hasTZ()}, nil
}

func parseXSDTimeParts(s string) (xsdTimeParts, error) {
	hour, next, ok := parseTwoDigits(s, 0)
	if !ok || next >= len(s) || s[next] != ':' {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	minute, next, ok := parseTwoDigits(s, next+1)
	if !ok || next >= len(s) || s[next] != ':' {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	second, next, ok := parseTwoDigits(s, next+1)
	if !ok {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	frac, next, err := parseFraction(s, next)
	if err != nil {
		return xsdTimeParts{}, err
	}
	tz, err := parseXSDTimezoneToEnd(s, next, "time")
	if err != nil {
		return xsdTimeParts{}, err
	}
	if hour > 24 || minute > 59 {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	if hour == 24 {
		if minute != 0 || second != 0 || frac != "" {
			return xsdTimeParts{}, fmt.Errorf("invalid time")
		}
	} else if second > 59 && (hour != 23 || minute != 59 || second != 60) {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	return xsdTimeParts{tz: tz, frac: frac, hour: hour, minute: minute, second: second}, nil
}

func (t xsdTimeParts) hasTZ() bool {
	return t.tz.present
}

func (t xsdTimeParts) rawSecond() int {
	return ((t.hour*60+t.minute)*60 + t.second) - t.tz.minutes*60
}

func parseFraction(s string, i int) (string, int, error) {
	if i == len(s) || s[i] != '.' {
		return "", i, nil
	}
	i++
	start := i
	for i < len(s) && isASCIIDigit(s[i]) {
		i++
	}
	if i == start {
		return "", 0, fmt.Errorf("invalid time")
	}
	return strings.TrimRight(s[start:i], "0"), i, nil
}

func compareXSDTime(a, b xsdTimeValue) int {
	if n := cmp.Compare(a.second, b.second); n != 0 {
		return n
	}
	return compareFraction(a.frac, b.frac)
}

func compareXSDTimePartial(a, b xsdTimeValue) partialCompareResult {
	if a.hasTZ == b.hasTZ {
		return partialCompareFromInt(compareXSDTime(a, b))
	}
	return compareXSDTemporal(xsdTimeTemporalValue(a), xsdTimeTemporalValue(b))
}

func xsdTimeTemporalValue(t xsdTimeValue) xsdTemporalValue {
	days, second := divModDay(t.second)
	point := xsdDateTimePoint{
		year:   xsdYear{digits: "2000"},
		month:  1,
		day:    1,
		second: second,
		frac:   t.frac,
	}
	return xsdTemporalValue{instant: addDays(point, days), hasTZ: t.hasTZ}
}

func compareFraction(a, b string) int {
	n := max(len(a), len(b))
	for i := range n {
		ad := byte('0')
		if i < len(a) {
			ad = a[i]
		}
		bd := byte('0')
		if i < len(b) {
			bd = b[i]
		}
		if ad < bd {
			return -1
		}
		if ad > bd {
			return 1
		}
	}
	return 0
}

func formatFraction(frac string) string {
	if frac == "" {
		return ""
	}
	return "." + frac
}

func formatXSDTime(v xsdTimeValue) string {
	hour := v.second / 3600
	minute := v.second / 60 % 60
	second := v.second % 60
	out := fmt.Sprintf("%02d:%02d:%02d%s", hour, minute, second, formatFraction(v.frac))
	if v.hasTZ {
		out += "Z"
	}
	return out
}
