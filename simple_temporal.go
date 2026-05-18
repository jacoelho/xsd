package xsd

import (
	"cmp"
	"fmt"
	"strconv"
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

func parseXSDDate(s string) (xsdDateValue, error) {
	return parseXSDDateValue(s)
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
	date, next, err := parseXSDDatePart(s, 0)
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
	date, next, err := parseXSDDatePart(s, 0)
	if err != nil {
		return xsdDateValue{}, err
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdDateValue{}, err
		}
		return xsdDateValue{}, fmt.Errorf("invalid date")
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

type xsdDatePart struct {
	year       xsdYear
	month, day int
}

func parseXSDDatePart(s string, i int) (xsdDatePart, int, error) {
	year, next, err := parseXSDYear(s, i)
	if err != nil {
		return xsdDatePart{}, 0, err
	}
	if next >= len(s) || s[next] != '-' {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	month, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok || next >= len(s) || s[next] != '-' {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	day, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok || month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) {
		return xsdDatePart{}, 0, fmt.Errorf("invalid date/time")
	}
	return xsdDatePart{year: year, month: month, day: day}, next, nil
}

func parseXSDYear(s string, i int) (xsdYear, int, error) {
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

func addYear(y xsdYear, delta int) xsdYear {
	for delta > 0 {
		y = nextYear(y)
		delta--
	}
	for delta < 0 {
		y = prevYear(y)
		delta++
	}
	return y
}

func nextYear(y xsdYear) xsdYear {
	if y.neg {
		if strings.TrimLeft(y.digits, "0") == "1" {
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
	if strings.TrimLeft(y.digits, "0") == "1" {
		return xsdYear{digits: "0001", neg: true}
	}
	y.digits = canonicalYearDigits(subUnsignedDecimalOne(y.digits))
	return y
}

func addUnsignedDecimalOne(s string) string {
	b := []byte(s)
	for i := len(b) - 1; i >= 0; i-- {
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
	for i := len(b) - 1; i >= 0; i-- {
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

func parseFixedDigits(s string, i, n int) (int, int, bool) {
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
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 31
	}
}

func isLeapYear(y xsdYear) bool {
	return yearMod(y, 400) == 0 || yearMod(y, 4) == 0 && yearMod(y, 100) != 0
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
	hour, _, ok1 := parseFixedDigits(s, i+1, 2)
	minute, _, ok2 := parseFixedDigits(s, i+4, 2)
	if !ok1 || !ok2 || hour > 14 || minute > 59 || hour == 14 && minute != 0 {
		return xsdTimezone{}, i, fmt.Errorf("invalid timezone")
	}
	offset := hour*60 + minute
	if s[i] == '-' {
		offset = -offset
	}
	return xsdTimezone{minutes: offset, present: true}, i + 6, nil
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

func addMinutes(p xsdDateTimePoint, minutes int) xsdDateTimePoint {
	return addSeconds(p, minutes*60)
}

func addSeconds(p xsdDateTimePoint, seconds int) xsdDateTimePoint {
	days, second := divModDay(p.second + seconds)
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
	p.year = addYear(p.year, 1)
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
		p.year = addYear(p.year, -1)
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
	hour, next, ok := parseFixedDigits(s, 0, 2)
	if !ok || next >= len(s) || s[next] != ':' {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	minute, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok || next >= len(s) || s[next] != ':' {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	second, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok {
		return xsdTimeParts{}, fmt.Errorf("invalid time")
	}
	frac, next, err := parseFraction(s, next)
	if err != nil {
		return xsdTimeParts{}, err
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdTimeParts{}, err
		}
		return xsdTimeParts{}, fmt.Errorf("invalid time")
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

type xsdDurationValue struct {
	frac         string
	months       int64
	seconds      int64
	negativeFrac bool
}

func parseXSDDurationValue(s string) (xsdDurationValue, error) {
	if s == "" {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	i := 0
	negative := false
	if s[i] == '-' {
		negative = true
		i++
		if i == len(s) {
			return xsdDurationValue{}, fmt.Errorf("invalid duration")
		}
	}
	if s[i] != 'P' {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	i++
	date, err := parseXSDDurationDateParts(s, &i)
	if err != nil {
		return xsdDurationValue{}, err
	}
	tm, err := parseXSDDurationTimeParts(s, &i)
	if err != nil {
		return xsdDurationValue{}, err
	}
	if i != len(s) || !date.seen && !tm.seen {
		return xsdDurationValue{}, fmt.Errorf("invalid duration")
	}
	monthTotal, err := checkedMulInt64(date.years, 12)
	if err != nil {
		return xsdDurationValue{}, err
	}
	monthTotal, err = checkedAddInt64(monthTotal, date.months)
	if err != nil {
		return xsdDurationValue{}, err
	}
	secondTotal, err := checkedDurationWholeSeconds(date.days, tm.hours, tm.minutes, tm.seconds)
	if err != nil {
		return xsdDurationValue{}, err
	}
	if negative {
		if monthTotal == minInt64Value || secondTotal == minInt64Value {
			return xsdDurationValue{}, fmt.Errorf("invalid duration")
		}
		monthTotal = -monthTotal
		secondTotal = -secondTotal
	}
	return xsdDurationValue{
		frac:         tm.frac,
		months:       monthTotal,
		seconds:      secondTotal,
		negativeFrac: negative && tm.frac != "",
	}, nil
}

type xsdDurationDateParts struct {
	years  int64
	months int64
	days   int64
	seen   bool
}

func parseXSDDurationDateParts(s string, i *int) (xsdDurationDateParts, error) {
	var out xsdDurationDateParts
	stage := 0
	for *i < len(s) && s[*i] != 'T' {
		value, err := parseDurationUnsigned(s, i)
		if err != nil || *i >= len(s) {
			return xsdDurationDateParts{}, fmt.Errorf("invalid duration")
		}
		switch s[*i] {
		case 'Y':
			if stage >= 1 {
				return xsdDurationDateParts{}, fmt.Errorf("invalid duration")
			}
			out.years = value
			stage = 1
		case 'M':
			if stage >= 2 {
				return xsdDurationDateParts{}, fmt.Errorf("invalid duration")
			}
			out.months = value
			stage = 2
		case 'D':
			if stage >= 3 {
				return xsdDurationDateParts{}, fmt.Errorf("invalid duration")
			}
			out.days = value
			stage = 3
		default:
			return xsdDurationDateParts{}, fmt.Errorf("invalid duration")
		}
		*i++
		out.seen = true
	}
	return out, nil
}

type xsdDurationTimeParts struct {
	frac    string
	hours   int64
	minutes int64
	seconds int64
	seen    bool
}

func parseXSDDurationTimeParts(s string, i *int) (xsdDurationTimeParts, error) {
	if *i == len(s) {
		return xsdDurationTimeParts{}, nil
	}
	if s[*i] != 'T' {
		return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
	}
	*i++
	var out xsdDurationTimeParts
	stage := 0
	for *i < len(s) {
		value, err := parseDurationUnsigned(s, i)
		if err != nil {
			return xsdDurationTimeParts{}, err
		}
		partFrac := ""
		hadFrac := false
		if *i < len(s) && s[*i] == '.' {
			hadFrac = true
			*i++
			start := *i
			for *i < len(s) && isASCIIDigit(s[*i]) {
				*i++
			}
			if *i == start {
				return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
			}
			partFrac = strings.TrimRight(s[start:*i], "0")
		}
		if *i >= len(s) {
			return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
		}
		switch s[*i] {
		case 'H':
			if stage >= 1 || hadFrac {
				return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
			}
			out.hours = value
			stage = 1
		case 'M':
			if stage >= 2 || hadFrac {
				return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
			}
			out.minutes = value
			stage = 2
		case 'S':
			if stage >= 3 {
				return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
			}
			out.seconds = value
			out.frac = partFrac
			stage = 3
		default:
			return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
		}
		*i++
		out.seen = true
	}
	if !out.seen {
		return xsdDurationTimeParts{}, fmt.Errorf("invalid duration")
	}
	return out, nil
}

func parseDurationUnsigned(s string, i *int) (int64, error) {
	if *i >= len(s) || !isASCIIDigit(s[*i]) {
		return 0, fmt.Errorf("invalid duration")
	}
	var out int64
	for *i < len(s) && isASCIIDigit(s[*i]) {
		digit := int64(s[*i] - '0')
		if out > (maxInt64Value-digit)/10 {
			return 0, fmt.Errorf("invalid duration")
		}
		out = out*10 + digit
		*i++
	}
	return out, nil
}

func checkedDurationWholeSeconds(days, hours, minutes, seconds int64) (int64, error) {
	out, err := checkedMulInt64(days, daySeconds)
	if err != nil {
		return 0, err
	}
	hourSeconds, err := checkedMulInt64(hours, 60*60)
	if err != nil {
		return 0, err
	}
	out, err = checkedAddInt64(out, hourSeconds)
	if err != nil {
		return 0, err
	}
	minuteSeconds, err := checkedMulInt64(minutes, 60)
	if err != nil {
		return 0, err
	}
	out, err = checkedAddInt64(out, minuteSeconds)
	if err != nil {
		return 0, err
	}
	return checkedAddInt64(out, seconds)
}

func checkedMulInt64(a, b int64) (int64, error) {
	if a != 0 && b > maxInt64Value/a {
		return 0, fmt.Errorf("invalid duration")
	}
	return a * b, nil
}

func checkedAddInt64(a, b int64) (int64, error) {
	if a > maxInt64Value-b {
		return 0, fmt.Errorf("invalid duration")
	}
	return a + b, nil
}

//nolint:govet // Field order keeps raw fields and normalized instant grouped.
type xsdGValue struct {
	instant xsdDateTimePoint
	tz      xsdTimezone
	year    xsdYear
	month   int
	day     int
}

func parseXSDGYearMonthValue(s string) (xsdGValue, error) {
	year, next, err := parseXSDYear(s, 0)
	if err != nil || next >= len(s) || s[next] != '-' {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gYearMonth")
	}
	month, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok || month < 1 || month > 12 {
		return xsdGValue{}, fmt.Errorf("invalid gYearMonth")
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gYearMonth")
	}
	return newXSDGValue(year, month, 1, tz), nil
}

func parseXSDGYearValue(s string) (xsdGValue, error) {
	year, next, err := parseXSDYear(s, 0)
	if err != nil {
		return xsdGValue{}, err
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gYear")
	}
	return newXSDGValue(year, 1, 1, tz), nil
}

func parseXSDGMonthDayValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "--") {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	month, next, ok := parseFixedDigits(s, 2, 2)
	if !ok || next >= len(s) || s[next] != '-' {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	day, next, ok := parseFixedDigits(s, next+1, 2)
	if !ok || month < 1 || month > 12 || day < 1 || day > maxGMonthDayOfMonth(month) {
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gMonthDay")
	}
	return newXSDGValue(xsdYear{digits: "2000"}, month, day, tz), nil
}

func parseXSDGDayValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "---") {
		return xsdGValue{}, fmt.Errorf("invalid gDay")
	}
	day, next, ok := parseFixedDigits(s, 3, 2)
	if !ok || day < 1 || day > 31 {
		return xsdGValue{}, fmt.Errorf("invalid gDay")
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gDay")
	}
	return newXSDGValue(xsdYear{digits: "2000"}, 1, day, tz), nil
}

func parseXSDGMonthValue(s string) (xsdGValue, error) {
	if !strings.HasPrefix(s, "--") {
		return xsdGValue{}, fmt.Errorf("invalid gMonth")
	}
	month, next, ok := parseFixedDigits(s, 2, 2)
	if !ok || month < 1 || month > 12 {
		return xsdGValue{}, fmt.Errorf("invalid gMonth")
	}
	tz, next, err := parseXSDTimezone(s, next)
	if err != nil || next != len(s) {
		if err != nil {
			return xsdGValue{}, err
		}
		return xsdGValue{}, fmt.Errorf("invalid gMonth")
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

func applyDurationBounds(f facetSet, norm string, actual actualValue) error {
	value := actual.Duration
	if !actual.Valid || actual.Kind != primDuration {
		var err error
		value, err = parseXSDDurationValue(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, parseXSDDurationValue, compareXSDDuration, func(l *compiledLiteral) (xsdDurationValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == primDuration {
			return l.Actual.Duration, true
		}
		return xsdDurationValue{}, false
	})
}

func equalXSDDuration(a, b xsdDurationValue) bool {
	return a.months == b.months &&
		a.seconds == b.seconds &&
		a.negativeFrac == b.negativeFrac &&
		compareFraction(a.frac, b.frac) == 0
}

func validateDurationFacetBounds(f facetSet) error {
	lower, err := durationLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := durationUpperBound(f)
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDDuration) {
		return fmt.Errorf("duration lower bound cannot exceed upper bound")
	}
	return nil
}

func durationLowerBound(f facetSet) (orderedFacetBound[xsdDurationValue], error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
		return partialCompareForMinInclusive(compareXSDDuration(other, out))
	})
}

func durationUpperBound(f facetSet) (orderedFacetBound[xsdDurationValue], error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
		return partialCompareForMaxInclusive(compareXSDDuration(other, out))
	})
}

func compareXSDDuration(a, b xsdDurationValue) partialCompareResult {
	months := cmp.Compare(a.months, b.months)
	seconds := compareXSDDurationSeconds(a, b)
	if months == 0 {
		return partialCompareFromInt(seconds)
	}
	if seconds == 0 || months == seconds {
		return partialCompareFromInt(months)
	}
	refs := []xsdDateTimePoint{
		{year: xsdYear{digits: "1696"}, month: 9, day: 1},
		{year: xsdYear{digits: "1697"}, month: 2, day: 1},
		{year: xsdYear{digits: "1903"}, month: 3, day: 1},
		{year: xsdYear{digits: "1903"}, month: 7, day: 1},
	}
	relation := 0
	for _, ref := range refs {
		ta, ok := addXSDDurationToPoint(ref, a)
		if !ok {
			return partialCompareIncomparable
		}
		tb, ok := addXSDDurationToPoint(ref, b)
		if !ok {
			return partialCompareIncomparable
		}
		n := compareXSDDateTimePoint(ta, tb)
		if n == 0 {
			continue
		}
		if relation != 0 && relation != n {
			return partialCompareIncomparable
		}
		relation = n
	}
	return partialCompareFromInt(relation)
}

func compareXSDDurationSeconds(a, b xsdDurationValue) int {
	if a.seconds != b.seconds {
		return cmp.Compare(a.seconds, b.seconds)
	}
	aSign := durationFracSign(a)
	bSign := durationFracSign(b)
	if aSign != bSign {
		return cmp.Compare(aSign, bSign)
	}
	switch aSign {
	case -1:
		return -compareFraction(a.frac, b.frac)
	case 1:
		return compareFraction(a.frac, b.frac)
	default:
		return 0
	}
}

func durationFracSign(d xsdDurationValue) int {
	if d.frac == "" {
		return 0
	}
	if d.negativeFrac {
		return -1
	}
	return 1
}

func addXSDDurationToPoint(p xsdDateTimePoint, d xsdDurationValue) (xsdDateTimePoint, bool) {
	p, ok := addXSDMonthsToPoint(p, d.months)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	if d.seconds != 0 {
		p, ok = addSeconds64(p, d.seconds)
		if !ok {
			return xsdDateTimePoint{}, false
		}
	}
	if d.frac == "" {
		return p, true
	}
	if d.negativeFrac {
		p, ok = addSeconds64(p, -1)
		if !ok {
			return xsdDateTimePoint{}, false
		}
		p.frac = complementFraction(d.frac)
	} else {
		p.frac = d.frac
	}
	return p, true
}

func addSeconds64(p xsdDateTimePoint, seconds int64) (xsdDateTimePoint, bool) {
	total, ok := checkedAddSignedInt64(int64(p.second), seconds)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	days, second := divModDay64(total)
	p.second = int(second)
	return addDays64(p, days)
}

func divModDay64(second int64) (int64, int64) {
	days := second / daySeconds
	rest := second % daySeconds
	if rest < 0 {
		rest += daySeconds
		days--
	}
	return days, rest
}

func addDays64(p xsdDateTimePoint, days int64) (xsdDateTimePoint, bool) {
	ordinal, ok := dateOrdinal(p.year, p.month, p.day)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	ordinal, ok = checkedAddSignedInt64(ordinal, days)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	year, month, day, ok := ordinalDate(ordinal)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	p.year = year
	p.month = month
	p.day = day
	return p, true
}

func addXSDMonthsToPoint(p xsdDateTimePoint, months int64) (xsdDateTimePoint, bool) {
	year, ok := xsdYearToAstronomicalInt64(p.year)
	if !ok || year > maxInt64Value/12 || year < minInt64Value/12 {
		return xsdDateTimePoint{}, false
	}
	total := year*12 + int64(p.month-1)
	total, ok = checkedAddSignedInt64(total, months)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	newYear := total / 12
	newMonth := total % 12
	if newMonth < 0 {
		newMonth += 12
		newYear--
	}
	p.year = xsdYearFromAstronomicalInt64(newYear)
	p.month = int(newMonth) + 1
	if maxDay := daysInMonth(p.year, p.month); p.day > maxDay {
		p.day = maxDay
	}
	return p, true
}

func dateOrdinal(year xsdYear, month, day int) (int64, bool) {
	y, ok := xsdYearToAstronomicalInt64(year)
	if !ok {
		return 0, false
	}
	if month <= 2 {
		y--
	}
	if y > maxOrdinalYear() || y < -maxOrdinalYear() {
		return 0, false
	}
	era := floorDivInt64(y, 400)
	yearOfEra := y - era*400
	monthPrime := int64(month)
	if monthPrime > 2 {
		monthPrime -= 3
	} else {
		monthPrime += 9
	}
	dayOfYear := (153*monthPrime+2)/5 + int64(day) - 1
	dayOfEra := yearOfEra*365 + yearOfEra/4 - yearOfEra/100 + dayOfYear
	eraDays, ok := checkedMulPositiveInt64(era, 146097)
	if !ok {
		return 0, false
	}
	return checkedAddSignedInt64(eraDays, dayOfEra)
}

func ordinalDate(ordinal int64) (xsdYear, int, int, bool) {
	era := floorDivInt64(ordinal, 146097)
	dayOfEra := ordinal - era*146097
	yearOfEra := (dayOfEra - dayOfEra/1460 + dayOfEra/36524 - dayOfEra/146096) / 365
	eraYears, ok := checkedMulPositiveInt64(era, 400)
	if !ok {
		return xsdYear{}, 0, 0, false
	}
	year, ok := checkedAddSignedInt64(yearOfEra, eraYears)
	if !ok || year > maxOrdinalYear() || year < -maxOrdinalYear() {
		return xsdYear{}, 0, 0, false
	}
	dayOfYear := dayOfEra - (365*yearOfEra + yearOfEra/4 - yearOfEra/100)
	monthPrime := (5*dayOfYear + 2) / 153
	day := int(dayOfYear - (153*monthPrime+2)/5 + 1)
	month := int(monthPrime + 3)
	if monthPrime >= 10 {
		month = int(monthPrime - 9)
		year, ok = checkedAddSignedInt64(year, 1)
		if !ok {
			return xsdYear{}, 0, 0, false
		}
	}
	if year > maxOrdinalYear() || year < -maxOrdinalYear() {
		return xsdYear{}, 0, 0, false
	}
	return xsdYearFromAstronomicalInt64(year), month, day, true
}

func floorDivInt64(a, b int64) int64 {
	q := a / b
	r := a % b
	if r != 0 && (r < 0) != (b < 0) {
		q--
	}
	return q
}

func maxOrdinalYear() int64 {
	return maxInt64Value/366 - 1
}

func xsdYearToAstronomicalInt64(y xsdYear) (int64, bool) {
	value, ok := parseUnsignedInt64Text(y.digits)
	if !ok || value > maxInt64Value {
		return 0, false
	}
	if y.neg {
		return 1 - value, true
	}
	return value, true
}

func xsdYearFromAstronomicalInt64(year int64) xsdYear {
	if year <= 0 {
		return xsdYear{digits: canonicalYearDigits(strconv.FormatInt(1-year, 10)), neg: true}
	}
	return xsdYear{digits: canonicalYearDigits(strconv.FormatInt(year, 10))}
}

func parseUnsignedInt64Text(s string) (int64, bool) {
	var out int64
	for i := range len(s) {
		digit := int64(s[i] - '0')
		if out > (maxInt64Value-digit)/10 {
			return 0, false
		}
		out = out*10 + digit
	}
	return out, true
}

func checkedAddSignedInt64(a, b int64) (int64, bool) {
	switch {
	case b > 0:
		if a > maxInt64Value-b {
			return 0, false
		}
	case b == minInt64Value:
		if a < 0 {
			return 0, false
		}
	case b < 0:
		if a < minInt64Value-b {
			return 0, false
		}
	}
	return a + b, true
}

func checkedMulPositiveInt64(a, b int64) (int64, bool) {
	if a > 0 && a > maxInt64Value/b {
		return 0, false
	}
	if a < 0 && a < minInt64Value/b {
		return 0, false
	}
	return a * b, true
}

func complementFraction(frac string) string {
	out := make([]byte, len(frac))
	for i := range frac {
		out[i] = '9' - (frac[i] - '0')
	}
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] < '9' {
			out[i]++
			break
		}
		out[i] = '0'
	}
	return strings.TrimRight(string(out), "0")
}

func applyGDayBounds(f facetSet, norm string, actual actualValue) error {
	return applyGValueBounds(primGDay, f, norm, actual, parseXSDGDayValue)
}

func validateGDayFacetBounds(f facetSet) error {
	return validateGValueFacetBounds("gDay", f, parseXSDGDayValue)
}

func applyGMonthDayBounds(f facetSet, norm string, actual actualValue) error {
	return applyGValueBounds(primGMonthDay, f, norm, actual, parseXSDGMonthDayValue)
}

func validateGMonthDayFacetBounds(f facetSet) error {
	return validateGValueFacetBounds("gMonthDay", f, parseXSDGMonthDayValue)
}

func applyGMonthBounds(f facetSet, norm string, actual actualValue) error {
	return applyGValueBounds(primGMonth, f, norm, actual, parseXSDGMonthValue)
}

func validateGMonthFacetBounds(f facetSet) error {
	return validateGValueFacetBounds("gMonth", f, parseXSDGMonthValue)
}

func applyGYearMonthBounds(f facetSet, norm string, actual actualValue) error {
	return applyGValueBounds(primGYearMonth, f, norm, actual, parseXSDGYearMonthValue)
}

func validateGYearMonthFacetBounds(f facetSet) error {
	return validateGValueFacetBounds("gYearMonth", f, parseXSDGYearMonthValue)
}

func applyGYearBounds(f facetSet, norm string, actual actualValue) error {
	return applyGValueBounds(primGYear, f, norm, actual, parseXSDGYearValue)
}

func validateGYearFacetBounds(f facetSet) error {
	return validateGValueFacetBounds("gYear", f, parseXSDGYearValue)
}

func applyGValueBounds(kind primitiveKind, f facetSet, norm string, actual actualValue, parse func(string) (xsdGValue, error)) error {
	value := actual.G
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = parse(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, parse, compareXSDGValue, func(l *compiledLiteral) (xsdGValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.G, true
		}
		return xsdGValue{}, false
	})
}

func validateGValueFacetBounds(name string, f facetSet, parse func(string) (xsdGValue, error)) error {
	lower, err := gValueLowerBound(f, parse)
	if err != nil {
		return err
	}
	upper, err := gValueUpperBound(f, parse)
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDGValue) {
		return fmt.Errorf("%s lower bound cannot exceed upper bound", name)
	}
	return nil
}

func gValueLowerBound(f facetSet, parse func(string) (xsdGValue, error)) (orderedFacetBound[xsdGValue], error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out xsdGValue) bool {
		return partialCompareForMinInclusive(compareXSDGValue(other, out))
	})
}

func gValueUpperBound(f facetSet, parse func(string) (xsdGValue, error)) (orderedFacetBound[xsdGValue], error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out xsdGValue) bool {
		return partialCompareForMaxInclusive(compareXSDGValue(other, out))
	})
}

func applyTemporalBounds(kind primitiveKind, f facetSet, norm string, actual actualValue) error {
	if kind == primTime {
		return applyTimeBounds(f, norm, actual)
	}
	value := actual.Temporal
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = parseXSDTemporalValue(kind, norm)
		if err != nil {
			return err
		}
	}
	parse := func(s string) (xsdTemporalValue, error) {
		return parseXSDTemporalValue(kind, s)
	}
	return applyPartialBoundsParsed(f, value, parse, compareXSDTemporal, func(l *compiledLiteral) (xsdTemporalValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.Temporal, true
		}
		return xsdTemporalValue{}, false
	})
}

func validateTemporalFacetBounds(kind primitiveKind, f facetSet) error {
	if kind == primTime {
		return validateTimeFacetBounds(f)
	}
	lower, err := temporalLowerBound(kind, f)
	if err != nil {
		return err
	}
	upper, err := temporalUpperBound(kind, f)
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDTemporal) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func temporalLowerBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out xsdTemporalValue) bool {
		return partialCompareForMinInclusive(compareXSDTemporal(other, out))
	})
}

func temporalUpperBound(kind primitiveKind, f facetSet) (orderedFacetBound[xsdTemporalValue], error) {
	parse := func(s string) (xsdTemporalValue, error) { return parseXSDTemporalValue(kind, s) }
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out xsdTemporalValue) bool {
		return partialCompareForMaxInclusive(compareXSDTemporal(other, out))
	})
}

func validateTimeFacetBounds(f facetSet) error {
	lower, err := timeLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := timeUpperBound(f)
	if err != nil {
		return err
	}
	if partialFacetBoundsInvalid(lower, upper, compareXSDTimePartial) {
		return fmt.Errorf("temporal lower bound cannot exceed upper bound")
	}
	return nil
}

func validateTimeFacetRestriction(f, base facetSet, step orderedFacetStep) error {
	baseLower, err := timeRawLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := timeRawUpperBound(base)
	if err != nil {
		return err
	}
	if step.minInclusive && baseLower.present() {
		if err := validateTimeLowerRestriction("minInclusive", f.MinInclusive, false, baseLower); err != nil {
			return err
		}
	}
	if step.minExclusive && baseLower.present() {
		if err := validateTimeLowerRestriction("minExclusive", f.MinExclusive, true, baseLower); err != nil {
			return err
		}
	}
	if step.maxInclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction("maxInclusive", f.MaxInclusive, false, baseUpper); err != nil {
			return err
		}
	}
	if step.maxExclusive && baseUpper.present() {
		if err := validateTimeUpperRestriction("maxExclusive", f.MaxExclusive, true, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeLowerRestriction(name string, lit *compiledLiteral, exclusive bool, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareLess || cmp == partialCompareEqual && !exclusive && base.exclusive() {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateTimeUpperRestriction(name string, lit *compiledLiteral, exclusive bool, base orderedFacetBound[xsdTimeValue]) error {
	if lit == nil {
		return nil
	}
	value, err := parseXSDTimeRaw(lit.Lexical)
	if err != nil {
		return err
	}
	cmp := compareXSDTimePartial(value, base.value)
	if cmp == partialCompareIncomparable || cmp == partialCompareGreater || cmp == partialCompareEqual && !exclusive && base.exclusive() {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

func timeLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

func timeUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeValue, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

func timeRawLowerBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundLexical(f.MinInclusive, f.MinExclusive, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMinInclusive(compareXSDTimePartial(other, out))
	})
}

func timeRawUpperBound(f facetSet) (orderedFacetBound[xsdTimeValue], error) {
	return facetBoundLexical(f.MaxInclusive, f.MaxExclusive, parseXSDTimeRaw, func(other, out xsdTimeValue) bool {
		return partialCompareForMaxInclusive(compareXSDTimePartial(other, out))
	})
}

func applyTimeBounds(f facetSet, norm string, actual actualValue) error {
	value := actual.Time
	if !actual.Valid || actual.Kind != primTime {
		var err error
		value, err = parseXSDTimeValue(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(f, value, parseXSDTimeValue, compareXSDTimePartial, func(l *compiledLiteral) (xsdTimeValue, bool) {
		if l.Actual.Valid && l.Actual.Kind == primTime {
			return l.Actual.Time, true
		}
		return xsdTimeValue{}, false
	})
}

func applyPartialBoundsParsed[T any](f facetSet, value T, parse func(string) (T, error), compare func(T, T) partialCompareResult, actual func(*compiledLiteral) (T, bool)) error {
	if err := applyPartialBound(f.MinInclusive, "minInclusive", value, parse, compare, actual, partialCompareForMinInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MaxInclusive, "maxInclusive", value, parse, compare, actual, partialCompareForMaxInclusive); err != nil {
		return err
	}
	if err := applyPartialBound(f.MinExclusive, "minExclusive", value, parse, compare, actual, partialCompareForMinExclusive); err != nil {
		return err
	}
	return applyPartialBound(f.MaxExclusive, "maxExclusive", value, parse, compare, actual, partialCompareForMaxExclusive)
}

func applyPartialBound[T any](
	lit *compiledLiteral,
	name string,
	value T,
	parse func(string) (T, error),
	compare func(T, T) partialCompareResult,
	actual func(*compiledLiteral) (T, bool),
	accept func(partialCompareResult) bool,
) error {
	if lit == nil {
		return nil
	}
	limit, err := partialBoundLiteral(lit, parse, actual)
	if err != nil {
		return err
	}
	if !accept(compare(value, limit)) {
		return fmt.Errorf("%s facet failed", name)
	}
	return nil
}

func partialBoundLiteral[T any](lit *compiledLiteral, parse func(string) (T, error), actual func(*compiledLiteral) (T, bool)) (T, error) {
	if actual != nil {
		if v, ok := actual(lit); ok {
			return v, nil
		}
	}
	return parse(lit.Canonical)
}
