package durationlex

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
)

// ErrIndeterminateComparison reports that two durations are incomparable in XSD value space.
var ErrIndeterminateComparison = errors.New("duration comparison indeterminate")

type durationFields struct {
	seconds num.Dec
	years   int
	months  int
	days    int
	hours   int
	minutes int
}

type dateTimeFields struct {
	second num.Dec
	year   int
	month  int
	day    int
	hour   int
	minute int
}

// durationOrderReferenceTimes are the XSD 1.0 reference dateTimes for duration ordering.
var durationOrderReferenceTimes = []dateTimeFields{
	{year: 1696, month: 9, day: 1, hour: 0, minute: 0, second: num.Dec{}},
	{year: 1697, month: 2, day: 1, hour: 0, minute: 0, second: num.Dec{}},
	{year: 1903, month: 3, day: 1, hour: 0, minute: 0, second: num.Dec{}},
	{year: 1903, month: 7, day: 1, hour: 0, minute: 0, second: num.Dec{}},
}

// Compare orders durations using the XSD 1.0 order relation for duration.
func Compare(left, right Duration) (int, error) {
	if isDayTimeDuration(left) && isDayTimeDuration(right) {
		return compareDayTimeDurations(left, right), nil
	}

	leftFields := durationFieldsFor(left)
	rightFields := durationFieldsFor(right)
	sign := 0
	sawEqual := false
	for _, ref := range durationOrderReferenceTimes {
		leftEnd := addDurationToDateTime(ref, leftFields)
		rightEnd := addDurationToDateTime(ref, rightFields)
		cmp := compareDateTimeFields(leftEnd, rightEnd)
		if cmp == 0 {
			if sign != 0 {
				return 0, ErrIndeterminateComparison
			}
			sawEqual = true
			continue
		}
		if sawEqual {
			return 0, ErrIndeterminateComparison
		}
		if sign == 0 {
			sign = cmp
			continue
		}
		if sign != cmp {
			return 0, ErrIndeterminateComparison
		}
	}
	return sign, nil
}

// CanonicalString formats a parsed duration in canonical lexical form used by validators.
func CanonicalString(dur Duration) string {
	var buf strings.Builder
	buf.Grow(32)
	if dur.Negative {
		buf.WriteByte('-')
	}
	buf.WriteByte('P')

	hasDate := false
	if dur.Years != 0 {
		buf.WriteString(strconv.Itoa(dur.Years))
		buf.WriteByte('Y')
		hasDate = true
	}
	if dur.Months != 0 {
		buf.WriteString(strconv.Itoa(dur.Months))
		buf.WriteByte('M')
		hasDate = true
	}
	if dur.Days != 0 {
		buf.WriteString(strconv.Itoa(dur.Days))
		buf.WriteByte('D')
		hasDate = true
	}

	hasTime := dur.Hours != 0 || dur.Minutes != 0 || dur.Seconds.Sign != 0
	if !hasDate && !hasTime {
		return "PT0S"
	}
	if !hasTime {
		return buf.String()
	}

	buf.WriteByte('T')
	if dur.Hours != 0 {
		buf.WriteString(strconv.Itoa(dur.Hours))
		buf.WriteByte('H')
	}
	if dur.Minutes != 0 {
		buf.WriteString(strconv.Itoa(dur.Minutes))
		buf.WriteByte('M')
	}
	if dur.Seconds.Sign != 0 {
		buf.WriteString(formatDurationSeconds(dur.Seconds))
		buf.WriteByte('S')
	}
	return buf.String()
}

func durationFieldsFor(dur Duration) durationFields {
	sign := 1
	if dur.Negative {
		sign = -1
	}
	seconds := dur.Seconds
	if sign < 0 {
		seconds = negateDec(seconds)
	}
	return durationFields{
		years:   sign * dur.Years,
		months:  sign * dur.Months,
		days:    sign * dur.Days,
		hours:   sign * dur.Hours,
		minutes: sign * dur.Minutes,
		seconds: seconds,
	}
}

func isDayTimeDuration(dur Duration) bool {
	return dur.Years == 0 && dur.Months == 0
}

func durationTotalSeconds(dur Duration) num.Dec {
	total := dur.Seconds
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Minutes)), num.FromInt64(60)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Hours)), num.FromInt64(3600)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Days)), num.FromInt64(86400)))
	if dur.Negative {
		total = negateDec(total)
	}
	return total
}

func compareDayTimeDurations(left, right Duration) int {
	leftSeconds := durationTotalSeconds(left)
	rightSeconds := durationTotalSeconds(right)
	return leftSeconds.Compare(rightSeconds)
}

func compareDateTimeFields(left, right dateTimeFields) int {
	switch {
	case left.year < right.year:
		return -1
	case left.year > right.year:
		return 1
	case left.month < right.month:
		return -1
	case left.month > right.month:
		return 1
	case left.day < right.day:
		return -1
	case left.day > right.day:
		return 1
	case left.hour < right.hour:
		return -1
	case left.hour > right.hour:
		return 1
	case left.minute < right.minute:
		return -1
	case left.minute > right.minute:
		return 1
	default:
		return left.second.Compare(right.second)
	}
}

func addDurationToDateTime(start dateTimeFields, dur durationFields) dateTimeFields {
	tempMonth := start.month + dur.months
	month := moduloIntRange(tempMonth, 1, 13)
	carry := fQuotientInt(tempMonth-1, 12)

	year := start.year + dur.years + carry

	tempSecond := decAdd(start.second, dur.seconds)
	carry, second := decDivModInt(tempSecond, 60)

	tempMinute := start.minute + dur.minutes + carry
	minute := moduloInt(tempMinute, 60)
	carry = fQuotientInt(tempMinute, 60)

	tempHour := start.hour + dur.hours + carry
	hour := moduloInt(tempHour, 24)
	carry = fQuotientInt(tempHour, 24)

	maxDay := maximumDayInMonthFor(year, month)
	tempDay := start.day
	switch {
	case tempDay > maxDay:
		tempDay = maxDay
	case tempDay < 1:
		tempDay = 1
	}
	day := tempDay + dur.days + carry

loop:
	for {
		maxDay = maximumDayInMonthFor(year, month)
		switch {
		case day < 1:
			day += maximumDayInMonthFor(year, month-1)
			carry = -1
		case day > maxDay:
			day -= maxDay
			carry = 1
		default:
			break loop
		}
		tempMonth = month + carry
		month = moduloIntRange(tempMonth, 1, 13)
		year += fQuotientInt(tempMonth-1, 12)
	}

	return dateTimeFields{
		year:   year,
		month:  month,
		day:    day,
		hour:   hour,
		minute: minute,
		second: second,
	}
}

func maximumDayInMonthFor(year, month int) int {
	m := moduloIntRange(month, 1, 13)
	y := year + fQuotientInt(month-1, 12)
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(y) {
			return 29
		}
		return 28
	default:
		return 28
	}
}

func isLeapYear(year int) bool {
	if moduloInt(year, 400) == 0 {
		return true
	}
	if moduloInt(year, 100) == 0 {
		return false
	}
	return moduloInt(year, 4) == 0
}

func fQuotientInt(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a >= 0 {
		return a / b
	}
	return -(((-a) + b - 1) / b)
}

func moduloInt(a, b int) int {
	return a - fQuotientInt(a, b)*b
}

func moduloIntRange(a, low, high int) int {
	return moduloInt(a-low, high-low) + low
}

func decAdd(a, b num.Dec) num.Dec {
	if a.Sign == 0 {
		return b
	}
	if b.Sign == 0 {
		return a
	}
	scale := max(b.Scale, a.Scale)
	ai := num.DecToScaledInt(a, scale)
	bi := num.DecToScaledInt(b, scale)
	sum := num.Add(ai, bi)
	return num.DecFromScaledInt(sum, scale)
}

func negateDec(dec num.Dec) num.Dec {
	if dec.Sign == 0 {
		return dec
	}
	dec.Sign = -dec.Sign
	return dec
}

func decDivModInt(dec num.Dec, divisor int) (int, num.Dec) {
	if divisor == 0 {
		return 0, dec
	}
	if dec.Sign == 0 {
		return 0, dec
	}
	if divisor < 0 {
		divisor = -divisor
		dec = negateDec(dec)
	}
	intDigits := decIntegerDigits(dec)
	qDigits, rem := divModDigitsByInt(intDigits, divisor)
	q := digitsToInt(qDigits)
	hasFraction := dec.Scale > 0
	if dec.Sign < 0 {
		if rem != 0 || hasFraction {
			q = -q - 1
		} else {
			q = -q
		}
	}
	remainder := decAdd(dec, num.FromInt64(int64(-q*divisor)).AsDec())
	return q, remainder
}

func formatDurationSeconds(sec num.Dec) string {
	buf := sec.RenderCanonical(nil)
	if len(buf) >= 2 && buf[len(buf)-2] == '.' && buf[len(buf)-1] == '0' {
		buf = buf[:len(buf)-2]
	}
	return string(buf)
}

func decIntegerDigits(dec num.Dec) []byte {
	if dec.Sign == 0 || len(dec.Coef) == 0 {
		return nil
	}
	if dec.Scale == 0 {
		return dec.Coef
	}
	if int(dec.Scale) >= len(dec.Coef) {
		return nil
	}
	return dec.Coef[:len(dec.Coef)-int(dec.Scale)]
}

func divModDigitsByInt(digits []byte, divisor int) ([]byte, int) {
	if divisor <= 0 || len(digits) == 0 {
		return nil, 0
	}
	quot := make([]byte, len(digits))
	rem := 0
	for i, d := range digits {
		rem = rem*10 + int(d-'0')
		q := rem / divisor
		rem %= divisor
		quot[i] = byte(q) + '0'
	}
	quot = trimLeadingZerosDigits(quot)
	if len(quot) == 0 || allZerosDigits(quot) {
		return nil, rem
	}
	return quot, rem
}

func digitsToInt(digits []byte) int {
	if len(digits) == 0 {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	n := 0
	for _, d := range digits {
		if n > (maxInt-int(d-'0'))/10 {
			return maxInt
		}
		n = n*10 + int(d-'0')
	}
	return n
}

func trimLeadingZerosDigits(b []byte) []byte {
	i := 0
	for i < len(b) && b[i] == '0' {
		i++
	}
	return b[i:]
}

func allZerosDigits(b []byte) bool {
	for _, c := range b {
		if c != '0' {
			return false
		}
	}
	return true
}
