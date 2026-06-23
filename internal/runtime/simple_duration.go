package runtime

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
)

const (
	durationDaySeconds = 24 * 60 * 60
	maxInt64Value      = int64(^uint64(0) >> 1)
	minInt64Value      = -maxInt64Value - 1
)

// DurationValue is the value-space projection for xs:duration values.
type DurationValue struct {
	frac         string
	months       int64
	seconds      int64
	negativeFrac bool
}

// ValidateDurationLexical validates raw as an XML Schema duration lexical value.
func ValidateDurationLexical[T byteText](raw T) error {
	_, err := parseDurationValue(raw)
	return err
}

// ParseDurationValue parses s as an XML Schema xs:duration value.
func ParseDurationValue(s string) (DurationValue, error) {
	return parseDurationValue(s)
}

func parseDurationValue[T byteText](raw T) (DurationValue, error) {
	if len(raw) == 0 {
		return DurationValue{}, errors.New("invalid duration")
	}
	i := 0
	negative := false
	if raw[i] == '-' {
		negative = true
		i++
		if i == len(raw) {
			return DurationValue{}, errors.New("invalid duration")
		}
	}
	if raw[i] != 'P' {
		return DurationValue{}, errors.New("invalid duration")
	}
	i++
	date, err := parseDurationDateParts(raw, &i)
	if err != nil {
		return DurationValue{}, err
	}
	tm, err := parseDurationTimeParts(raw, &i)
	if err != nil {
		return DurationValue{}, err
	}
	if i != len(raw) || !date.seen && !tm.seen {
		return DurationValue{}, errors.New("invalid duration")
	}
	monthTotal, err := checkedDurationMulInt64(date.years, 12)
	if err != nil {
		return DurationValue{}, err
	}
	monthTotal, err = checkedDurationAddInt64(monthTotal, date.months)
	if err != nil {
		return DurationValue{}, err
	}
	secondTotal, err := checkedDurationWholeSeconds(date, tm)
	if err != nil {
		return DurationValue{}, err
	}
	if negative {
		if monthTotal == minInt64Value || secondTotal == minInt64Value {
			return DurationValue{}, errors.New("invalid duration")
		}
		monthTotal = -monthTotal
		secondTotal = -secondTotal
	}
	return DurationValue{
		frac:         tm.frac,
		months:       monthTotal,
		seconds:      secondTotal,
		negativeFrac: negative && tm.frac != "",
	}, nil
}

// EqualDurationValues reports XML Schema equality for xs:duration values.
func EqualDurationValues(a, b DurationValue) bool {
	return a.months == b.months &&
		a.seconds == b.seconds &&
		a.negativeFrac == b.negativeFrac &&
		compareFraction(a.frac, b.frac) == 0
}

// CompareDurationValues compares xs:duration values using the XML Schema
// partial order.
func CompareDurationValues(a, b DurationValue) OrderedFacetRelation {
	months := cmp.Compare(a.months, b.months)
	seconds := compareDurationSeconds(a, b)
	if months == 0 {
		return orderedFacetRelationFromInt(seconds)
	}
	if seconds == 0 || months == seconds {
		return orderedFacetRelationFromInt(months)
	}
	refs := [...]xsdDateTimePoint{
		{year: xsdYear{digits: "1696"}, month: 9, day: 1},
		{year: xsdYear{digits: "1697"}, month: 2, day: 1},
		{year: xsdYear{digits: "1903"}, month: 3, day: 1},
		{year: xsdYear{digits: "1903"}, month: 7, day: 1},
	}
	relation := 0
	for i, ref := range refs {
		ta, ok := addDurationToPoint(ref, a)
		if !ok {
			return OrderedFacetIncomparable
		}
		tb, ok := addDurationToPoint(ref, b)
		if !ok {
			return OrderedFacetIncomparable
		}
		n := compareXSDDateTimePoint(ta, tb)
		if i == 0 {
			relation = n
			continue
		}
		if relation != n {
			return OrderedFacetIncomparable
		}
	}
	return orderedFacetRelationFromInt(relation)
}

type durationDateParts struct {
	years  int64
	months int64
	days   int64
	seen   bool
}

func parseDurationDateParts[T byteText](raw T, i *int) (durationDateParts, error) {
	var out durationDateParts
	stage := 0
	for *i < len(raw) && raw[*i] != 'T' {
		value, err := parseDurationUnsigned(raw, i)
		if err != nil || *i >= len(raw) {
			return durationDateParts{}, errors.New("invalid duration")
		}
		switch raw[*i] {
		case 'Y':
			if stage >= 1 {
				return durationDateParts{}, errors.New("invalid duration")
			}
			out.years = value
			stage = 1
		case 'M':
			if stage >= 2 {
				return durationDateParts{}, errors.New("invalid duration")
			}
			out.months = value
			stage = 2
		case 'D':
			if stage >= 3 {
				return durationDateParts{}, errors.New("invalid duration")
			}
			out.days = value
			stage = 3
		default:
			return durationDateParts{}, errors.New("invalid duration")
		}
		*i++
		out.seen = true
	}
	return out, nil
}

type durationTimeParts struct {
	frac    string
	hours   int64
	minutes int64
	seconds int64
	seen    bool
}

func parseDurationTimeParts[T byteText](raw T, i *int) (durationTimeParts, error) {
	if *i == len(raw) {
		return durationTimeParts{}, nil
	}
	if raw[*i] != 'T' {
		return durationTimeParts{}, errors.New("invalid duration")
	}
	*i++
	var out durationTimeParts
	stage := 0
	for *i < len(raw) {
		value, err := parseDurationUnsigned(raw, i)
		if err != nil {
			return durationTimeParts{}, err
		}
		frac, hadFrac, err := parseDurationFraction(raw, i)
		if err != nil {
			return durationTimeParts{}, err
		}
		if *i >= len(raw) {
			return durationTimeParts{}, errors.New("invalid duration")
		}
		switch raw[*i] {
		case 'H':
			if stage >= 1 || hadFrac {
				return durationTimeParts{}, errors.New("invalid duration")
			}
			out.hours = value
			stage = 1
		case 'M':
			if stage >= 2 || hadFrac {
				return durationTimeParts{}, errors.New("invalid duration")
			}
			out.minutes = value
			stage = 2
		case 'S':
			if stage >= 3 {
				return durationTimeParts{}, errors.New("invalid duration")
			}
			out.seconds = value
			out.frac = frac
			stage = 3
		default:
			return durationTimeParts{}, errors.New("invalid duration")
		}
		*i++
		out.seen = true
	}
	if !out.seen {
		return durationTimeParts{}, errors.New("invalid duration")
	}
	return out, nil
}

func parseDurationFraction[T byteText](raw T, i *int) (string, bool, error) {
	if *i >= len(raw) || raw[*i] != '.' {
		return "", false, nil
	}
	*i++
	start := *i
	for *i < len(raw) && isASCIIDigit(raw[*i]) {
		*i++
	}
	if *i == start {
		return "", true, errors.New("invalid duration")
	}
	return strings.TrimRight(string(raw[start:*i]), "0"), true, nil
}

func parseDurationUnsigned[T byteText](raw T, i *int) (int64, error) {
	if *i >= len(raw) || !isASCIIDigit(raw[*i]) {
		return 0, errors.New("invalid duration")
	}
	var out int64
	for *i < len(raw) && isASCIIDigit(raw[*i]) {
		digit := int64(raw[*i] - '0')
		if out > (maxInt64Value-digit)/10 {
			return 0, errors.New("invalid duration")
		}
		out = out*10 + digit
		*i++
	}
	return out, nil
}

func checkedDurationWholeSeconds(date durationDateParts, tm durationTimeParts) (int64, error) {
	out, err := checkedDurationMulInt64(date.days, durationDaySeconds)
	if err != nil {
		return 0, err
	}
	hourSeconds, err := checkedDurationMulInt64(tm.hours, 60*60)
	if err != nil {
		return 0, err
	}
	out, err = checkedDurationAddInt64(out, hourSeconds)
	if err != nil {
		return 0, err
	}
	minuteSeconds, err := checkedDurationMulInt64(tm.minutes, 60)
	if err != nil {
		return 0, err
	}
	out, err = checkedDurationAddInt64(out, minuteSeconds)
	if err != nil {
		return 0, err
	}
	return checkedDurationAddInt64(out, tm.seconds)
}

func checkedDurationMulInt64(a, b int64) (int64, error) {
	if a != 0 && b > maxInt64Value/a {
		return 0, errors.New("invalid duration")
	}
	return a * b, nil
}

func checkedDurationAddInt64(a, b int64) (int64, error) {
	if a > maxInt64Value-b {
		return 0, errors.New("invalid duration")
	}
	return a + b, nil
}

func compareDurationSeconds(a, b DurationValue) int {
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

func durationFracSign(d DurationValue) int {
	if d.frac == "" {
		return 0
	}
	if d.negativeFrac {
		return -1
	}
	return 1
}

func addDurationToPoint(p xsdDateTimePoint, d DurationValue) (xsdDateTimePoint, bool) {
	p, ok := addDurationMonthsToPoint(p, d.months)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	if d.seconds != 0 {
		p, ok = addDurationSeconds64(p, d.seconds)
		if !ok {
			return xsdDateTimePoint{}, false
		}
	}
	if d.frac == "" {
		return p, true
	}
	if d.negativeFrac {
		p, ok = addDurationSeconds64(p, -1)
		if !ok {
			return xsdDateTimePoint{}, false
		}
		p.frac = complementDurationFraction(d.frac)
	} else {
		p.frac = d.frac
	}
	return p, true
}

func addDurationSeconds64(p xsdDateTimePoint, seconds int64) (xsdDateTimePoint, bool) {
	total, ok := checkedAddSignedInt64(int64(p.second), seconds)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	days, second := durationDivModDay64(total)
	p.second = int(second)
	return addDurationDays64(p, days)
}

func durationDivModDay64(second int64) (int64, int64) {
	days := second / durationDaySeconds
	rest := second % durationDaySeconds
	if rest < 0 {
		rest += durationDaySeconds
		days--
	}
	return days, rest
}

func addDurationDays64(p xsdDateTimePoint, days int64) (xsdDateTimePoint, bool) {
	ordinal, ok := durationDateOrdinal(p.year, p.month, p.day)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	ordinal, ok = checkedAddSignedInt64(ordinal, days)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	year, month, day, ok := durationOrdinalDate(ordinal)
	if !ok {
		return xsdDateTimePoint{}, false
	}
	p.year = year
	p.month = month
	p.day = day
	return p, true
}

func addDurationMonthsToPoint(p xsdDateTimePoint, months int64) (xsdDateTimePoint, bool) {
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

func durationDateOrdinal(year xsdYear, month, day int) (int64, bool) {
	y, ok := xsdYearToAstronomicalInt64(year)
	if !ok {
		return 0, false
	}
	if month <= 2 {
		y--
	}
	if y > maxDurationOrdinalYear() || y < -maxDurationOrdinalYear() {
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

func durationOrdinalDate(ordinal int64) (xsdYear, int, int, bool) {
	era := floorDivInt64(ordinal, 146097)
	dayOfEra := ordinal - era*146097
	yearOfEra := (dayOfEra - dayOfEra/1460 + dayOfEra/36524 - dayOfEra/146096) / 365
	eraYears, ok := checkedMulPositiveInt64(era, 400)
	if !ok {
		return xsdYear{}, 0, 0, false
	}
	year, ok := checkedAddSignedInt64(yearOfEra, eraYears)
	if !ok || year > maxDurationOrdinalYear() || year < -maxDurationOrdinalYear() {
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
	if year > maxDurationOrdinalYear() || year < -maxDurationOrdinalYear() {
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

func maxDurationOrdinalYear() int64 {
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

func complementDurationFraction(frac string) string {
	out := make([]byte, len(frac))
	for i := range frac {
		out[i] = '9' - (frac[i] - '0')
	}
	for i := range slices.Backward(out) {
		if out[i] < '9' {
			out[i]++
			break
		}
		out[i] = '0'
	}
	return strings.TrimRight(string(out), "0")
}
