package xsd

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

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
	secondTotal, err := checkedDurationWholeSeconds(date, tm)
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

// checkedDurationWholeSeconds keeps every unit conversion overflow-checked.
func checkedDurationWholeSeconds(date xsdDurationDateParts, tm xsdDurationTimeParts) (int64, error) {
	out, err := checkedMulInt64(date.days, daySeconds)
	if err != nil {
		return 0, err
	}
	hourSeconds, err := checkedMulInt64(tm.hours, 60*60)
	if err != nil {
		return 0, err
	}
	out, err = checkedAddInt64(out, hourSeconds)
	if err != nil {
		return 0, err
	}
	minuteSeconds, err := checkedMulInt64(tm.minutes, 60)
	if err != nil {
		return 0, err
	}
	out, err = checkedAddInt64(out, minuteSeconds)
	if err != nil {
		return 0, err
	}
	return checkedAddInt64(out, tm.seconds)
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

// applyDurationBounds reuses parsed duration values when validation already has them.
func applyDurationBounds(f facetSet, norm string, actual actualValue) error {
	value := actual.Duration
	if !actual.Valid || actual.Kind != primDuration {
		var err error
		value, err = parseXSDDurationValue(norm)
		if err != nil {
			return err
		}
	}
	return applyPartialBoundsParsed(&f, value, parseXSDDurationValue, compareXSDDuration, actualDurationLiteral)
}

func actualDurationLiteral(l *compiledLiteral) (xsdDurationValue, bool) {
	if l.Actual.Valid && l.Actual.Kind == primDuration {
		return l.Actual.Duration, true
	}
	return xsdDurationValue{}, false
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

// Duration lower and upper facets use different partial-order acceptance rules.
func durationLowerBound(f facetSet) (orderedFacetBound[xsdDurationValue], error) {
	return facetBound(f.MinInclusive, f.MinExclusive, facetCanonical, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
		return partialCompareForMinInclusive(compareXSDDuration(other, out))
	})
}

// durationUpperBound applies the max-facet rule for partial duration order.
func durationUpperBound(f facetSet) (orderedFacetBound[xsdDurationValue], error) {
	return facetBound(f.MaxInclusive, f.MaxExclusive, facetCanonical, parseXSDDurationValue, func(other, out xsdDurationValue) bool {
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
