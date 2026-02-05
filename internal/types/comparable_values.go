package types

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
)

var (
	// durationDatePattern matches date components in XSD duration format: Y, M, D
	// Examples: "1Y", "2M", "3D", "1Y2M3D"
	durationDatePattern = regexp.MustCompile(`(\d+)Y|(\d+)M|(\d+)D`)

	// durationTimePattern matches time components in XSD duration format: H, M, S
	// Examples: "1H", "2M", "3S", "1.5S", "1H2M3.4S"
	durationTimePattern = regexp.MustCompile(`(\d+)H|(\d+)M|(\d+(\.\d+)?)S`)
)

// ComparableValue is a unified interface for comparable values that can be compared across types
// This is used by range facets to store and compare values without generic type parameters
type ComparableValue interface {
	Compare(other ComparableValue) (int, error)
	String() string
	Type() Type // Returns the XSD type this value represents
}

// Unwrappable is an interface for types that can unwrap their inner value
type Unwrappable interface {
	Unwrap() any
}

// ComparableDec wraps num.Dec to implement ComparableValue.
type ComparableDec struct {
	Typ   Type
	Value num.Dec
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Supports cross-type comparison with ComparableInt since integers are a subset of decimals.
func (c ComparableDec) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableDec:
		return c.Value.Compare(otherVal.Value), nil
	case ComparableInt:
		return c.Value.Compare(otherVal.Value.AsDec()), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableDec with %T", other)
	}
}

// String returns the string representation (implements ComparableValue)
func (c ComparableDec) String() string {
	return string(c.Value.RenderCanonical(nil))
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableDec) Type() Type {
	return c.Typ
}

// Unwrap returns the inner num.Dec value.
func (c ComparableDec) Unwrap() any {
	return c.Value
}

// ComparableInt wraps num.Int to implement ComparableValue.
type ComparableInt struct {
	Typ   Type
	Value num.Int
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Supports cross-type comparison with ComparableDec since integers are a subset of decimals.
func (c ComparableInt) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableInt:
		return c.Value.Compare(otherVal.Value), nil
	case ComparableDec:
		return c.Value.CompareDec(otherVal.Value), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableInt with %T", other)
	}
}

// String returns the string representation (implements ComparableValue)
func (c ComparableInt) String() string {
	return string(c.Value.RenderCanonical(nil))
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableInt) Type() Type {
	return c.Typ
}

// Unwrap returns the inner num.Int value.
func (c ComparableInt) Unwrap() any {
	return c.Value
}

// ComparableTime wraps time.Time to implement ComparableValue
type ComparableTime struct {
	Value time.Time
	// XSD type this value represents
	Typ          Type
	TimezoneKind value.TimezoneKind
}

var errIndeterminateTimeComparison = errors.New("time comparison indeterminate")

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableTime) Compare(other ComparableValue) (int, error) {
	otherTime, ok := other.(ComparableTime)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableTime with %T", other)
	}
	if c.TimezoneKind == otherTime.TimezoneKind {
		if c.Value.Before(otherTime.Value) {
			return -1, nil
		}
		if c.Value.After(otherTime.Value) {
			return 1, nil
		}
		return 0, nil
	}
	if c.TimezoneKind == value.TZKnown {
		return compareTimezonedToLocal(c.Value, otherTime.Value)
	}
	cmp, err := compareTimezonedToLocal(otherTime.Value, c.Value)
	if err != nil {
		return 0, err
	}
	return -cmp, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableTime) String() string {
	switch c.TimezoneKind {
	case value.TZKnown:
		return c.Value.UTC().Format(time.RFC3339Nano)
	default:
		return c.Value.Format("2006-01-02T15:04:05.999999999")
	}
}

func compareTimezonedToLocal(timezoned, local time.Time) (int, error) {
	tzUTC := timezoned.UTC()
	localUTC := local.UTC()
	localPlus14 := localUTC.Add(-14 * time.Hour)
	localMinus14 := localUTC.Add(14 * time.Hour)
	if tzUTC.Before(localPlus14) {
		return -1, nil
	}
	if tzUTC.After(localMinus14) {
		return 1, nil
	}
	return 0, errIndeterminateTimeComparison
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableTime) Type() Type {
	return c.Typ
}

// Unwrap returns the inner time.Time value
func (c ComparableTime) Unwrap() any {
	return c.Value
}

// ComparableFloat64 wraps float64 to implement ComparableValue with NaN/INF handling
type ComparableFloat64 struct {
	Typ   Type
	Value float64
}

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableFloat64) Compare(other ComparableValue) (int, error) {
	otherFloat, ok := other.(ComparableFloat64)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableFloat64 with %T", other)
	}
	if math.IsNaN(c.Value) || math.IsNaN(otherFloat.Value) {
		return 0, fmt.Errorf("cannot compare NaN values")
	}

	cIsInf := math.IsInf(c.Value, 0)
	otherIsInf := math.IsInf(otherFloat.Value, 0)

	if cIsInf && otherIsInf {
		// both are infinite
		if math.IsInf(c.Value, 1) && math.IsInf(otherFloat.Value, 1) {
			return 0, nil // both +INF
		}
		if math.IsInf(c.Value, -1) && math.IsInf(otherFloat.Value, -1) {
			return 0, nil // both -INF
		}
		if math.IsInf(c.Value, 1) && math.IsInf(otherFloat.Value, -1) {
			return 1, nil // +INF > -INF
		}
		return -1, nil // -INF < +INF
	}

	if cIsInf {
		if math.IsInf(c.Value, 1) {
			return 1, nil // +INF > any finite value
		}
		return -1, nil // -INF < any finite value
	}

	if otherIsInf {
		if math.IsInf(otherFloat.Value, 1) {
			return -1, nil // any finite value < +INF
		}
		return 1, nil // any finite value > -INF
	}

	// both are finite, normal comparison
	if c.Value < otherFloat.Value {
		return -1, nil
	}
	if c.Value > otherFloat.Value {
		return 1, nil
	}
	return 0, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableFloat64) String() string {
	return fmt.Sprintf("%g", c.Value)
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableFloat64) Type() Type {
	return c.Typ
}

// Unwrap returns the inner float64 value
func (c ComparableFloat64) Unwrap() any {
	return c.Value
}

// ComparableFloat32 wraps float32 to implement ComparableValue with NaN/INF handling
type ComparableFloat32 struct {
	Typ   Type
	Value float32
}

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableFloat32) Compare(other ComparableValue) (int, error) {
	otherFloat, ok := other.(ComparableFloat32)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableFloat32 with %T", other)
	}
	c64 := ComparableFloat64{Value: float64(c.Value), Typ: c.Typ}
	other64 := ComparableFloat64{Value: float64(otherFloat.Value), Typ: otherFloat.Typ}
	return c64.Compare(other64)
}

// String returns the string representation (implements ComparableValue)
func (c ComparableFloat32) String() string {
	return fmt.Sprintf("%g", c.Value)
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableFloat32) Type() Type {
	return c.Typ
}

// Unwrap returns the inner float32 value
func (c ComparableFloat32) Unwrap() any {
	return c.Value
}

// ComparableDuration wraps time.Duration to implement ComparableValue
// Note: Durations are partially ordered, so comparison is limited to pure day/time durations
type ComparableDuration struct {
	Typ   Type
	Value time.Duration
}

// ParseDurationToTimeDuration parses an XSD duration string into a time.Duration
// Returns an error if the duration contains years or months (which cannot be converted to time.Duration)
// or if the duration string is invalid.
func ParseDurationToTimeDuration(s string) (time.Duration, error) {
	xsdDur, err := ParseXSDDuration(s)
	if err != nil {
		return 0, err
	}
	if xsdDur.Years != 0 || xsdDur.Months != 0 {
		return 0, fmt.Errorf("durations with years or months cannot be converted to time.Duration (indeterminate)")
	}
	const maxDuration = time.Duration(^uint64(0) >> 1)

	componentDuration := func(value int, unit time.Duration) (time.Duration, error) {
		if value == 0 {
			return 0, nil
		}
		if value < 0 {
			return 0, fmt.Errorf("duration component out of range")
		}
		limit := int64(maxDuration / unit)
		if int64(value) > limit {
			return 0, fmt.Errorf("duration too large")
		}
		return time.Duration(value) * unit, nil
	}

	addDuration := func(total, delta time.Duration) (time.Duration, error) {
		if delta < 0 {
			return 0, fmt.Errorf("duration component out of range")
		}
		if total > maxDuration-delta {
			return 0, fmt.Errorf("duration too large")
		}
		return total + delta, nil
	}

	dur := time.Duration(0)
	var delta time.Duration

	delta, err = componentDuration(xsdDur.Days, 24*time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(xsdDur.Hours, time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(xsdDur.Minutes, time.Minute)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	secondsDuration, err := secondsToDuration(xsdDur.Seconds)
	if err != nil {
		return 0, err
	}
	if dur, err = addDuration(dur, secondsDuration); err != nil {
		return 0, err
	}

	if xsdDur.Negative {
		dur = -dur
	}
	return dur, nil
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Both durations must be pure day/time durations (no years/months)
func (c ComparableDuration) Compare(other ComparableValue) (int, error) {
	// try ComparableXSDDuration first (for full XSD duration support)
	if otherXSDDur, ok := other.(ComparableXSDDuration); ok {
		negative := c.Value < 0
		durVal := c.Value
		if negative {
			durVal = -durVal
		}
		hours := int(durVal / time.Hour)
		durVal %= time.Hour
		minutes := int(durVal / time.Minute)
		durVal %= time.Minute
		seconds := decFromDurationSeconds(durVal)
		thisXSDDur := ComparableXSDDuration{
			Value: XSDDuration{
				Negative: negative,
				Years:    0,
				Months:   0,
				Days:     0,
				Hours:    hours,
				Minutes:  minutes,
				Seconds:  seconds,
			},
			Typ: c.Typ,
		}
		return thisXSDDur.Compare(otherXSDDur)
	}
	otherDur, ok := other.(ComparableDuration)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableDuration with %T", other)
	}
	if c.Value < otherDur.Value {
		return -1, nil
	}
	if c.Value > otherDur.Value {
		return 1, nil
	}
	return 0, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableDuration) String() string {
	return c.Value.String()
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableDuration) Type() Type {
	return c.Typ
}

// Unwrap returns the inner time.Duration value
func (c ComparableDuration) Unwrap() any {
	return c.Value
}

// XSDDuration represents a full XSD duration with all components
type XSDDuration struct {
	Seconds  num.Dec
	Years    int
	Months   int
	Days     int
	Hours    int
	Minutes  int
	Negative bool
}

// ComparableXSDDuration wraps XSDDuration to implement ComparableValue
// This supports full XSD durations including years and months
type ComparableXSDDuration struct {
	Typ   Type
	Value XSDDuration
}

var errIndeterminateDurationComparison = errors.New("duration comparison indeterminate")

// ParseXSDDuration parses an XSD duration string into an XSDDuration struct
// Supports all XSD duration components including years and months
func ParseXSDDuration(s string) (XSDDuration, error) {
	if s == "" {
		return XSDDuration{}, fmt.Errorf("empty duration")
	}

	input := s
	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	if s == "" || s[0] != 'P' {
		return XSDDuration{}, fmt.Errorf("duration must start with P")
	}
	s = s[1:]

	datePart := s
	timePart := ""
	sawTimeDesignator := false
	if before, after, ok := strings.Cut(s, "T"); ok {
		sawTimeDesignator = true
		datePart = before
		timePart = after
		if strings.IndexByte(timePart, 'T') != -1 {
			return XSDDuration{}, fmt.Errorf("invalid duration format: multiple T separators")
		}
	}

	if !durationPattern.MatchString(input) {
		return XSDDuration{}, fmt.Errorf("invalid duration format: %s", input)
	}

	var years, months, days, hours, minutes int
	var seconds num.Dec
	hasDateComponent := false
	hasTimeComponent := false
	maxComponent := uint64(^uint(0) >> 1)
	parseComponent := func(value, label string) (int, error) {
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return 0, fmt.Errorf("%s value too large", label)
			}
			return 0, fmt.Errorf("invalid %s value: %w", label, err)
		}
		if u > maxComponent {
			return 0, fmt.Errorf("%s value too large", label)
		}
		return int(u), nil
	}

	// parse date part (years, months, days)
	if datePart != "" {
		matches := durationDatePattern.FindAllStringSubmatch(datePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := parseComponent(match[1], "year")
				if err != nil {
					return XSDDuration{}, err
				}
				years = val
				hasDateComponent = true
			}
			if match[2] != "" {
				val, err := parseComponent(match[2], "month")
				if err != nil {
					return XSDDuration{}, err
				}
				months = val
				hasDateComponent = true
			}
			if match[3] != "" {
				val, err := parseComponent(match[3], "day")
				if err != nil {
					return XSDDuration{}, err
				}
				days = val
				hasDateComponent = true
			}
		}
	}

	// parse time part (hours, minutes, seconds)
	if timePart != "" {
		matches := durationTimePattern.FindAllStringSubmatch(timePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := parseComponent(match[1], "hour")
				if err != nil {
					return XSDDuration{}, err
				}
				hours = val
				hasTimeComponent = true
			}
			if match[2] != "" {
				val, err := parseComponent(match[2], "minute")
				if err != nil {
					return XSDDuration{}, err
				}
				minutes = val
				hasTimeComponent = true
			}
			if match[3] != "" {
				dec, perr := num.ParseDec([]byte(match[3]))
				if perr != nil {
					return XSDDuration{}, fmt.Errorf("invalid second value: %w", perr)
				}
				if dec.Sign < 0 {
					return XSDDuration{}, fmt.Errorf("second value cannot be negative")
				}
				seconds = dec
				hasTimeComponent = true
			}
		}
	}

	// check if we actually parsed any components
	hasAnyComponent := hasDateComponent || hasTimeComponent
	if !hasAnyComponent {
		return XSDDuration{}, fmt.Errorf("duration must have at least one component")
	}
	if sawTimeDesignator && !hasTimeComponent {
		return XSDDuration{}, fmt.Errorf("time designator present but no time components specified")
	}

	if years == 0 && months == 0 && days == 0 && hours == 0 && minutes == 0 && seconds.Sign == 0 {
		negative = false
	}
	return XSDDuration{
		Negative: negative,
		Years:    years,
		Months:   months,
		Days:     days,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  seconds,
	}, nil
}

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

func durationFieldsFor(dur XSDDuration) durationFields {
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

func isDayTimeDuration(dur XSDDuration) bool {
	return dur.Years == 0 && dur.Months == 0
}

func durationTotalSeconds(dur XSDDuration) num.Dec {
	total := dur.Seconds
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Minutes)), num.FromInt64(60)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Hours)), num.FromInt64(3600)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Days)), num.FromInt64(86400)))
	if dur.Negative {
		total = negateDec(total)
	}
	return total
}

func compareDayTimeDurations(left, right XSDDuration) int {
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
	carry := fQuotientIntRange(tempMonth, 1, 13)

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
		year += fQuotientIntRange(tempMonth, 1, 13)
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
	y := year + fQuotientIntRange(month, 1, 13)
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

func fQuotientIntRange(a, low, high int) int {
	return fQuotientInt(a-low, high-low)
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
	remainder := decAdd(dec, decFromInt64(int64(-q*divisor)))
	return q, remainder
}

func decFromInt64(v int64) num.Dec {
	return num.FromInt64(v).AsDec()
}

func decFromDurationSeconds(d time.Duration) num.Dec {
	sec := int64(d / time.Second)
	nanos := int64(d % time.Second)
	if nanos == 0 {
		return decFromInt64(sec)
	}
	if nanos < 0 {
		nanos = -nanos
	}
	text := fmt.Sprintf("%d.%09d", sec, nanos)
	text = strings.TrimRight(text, "0")
	dec, _ := num.ParseDec([]byte(text))
	return dec
}

func formatDurationSeconds(sec num.Dec) string {
	buf := sec.RenderCanonical(nil)
	if len(buf) >= 2 && buf[len(buf)-2] == '.' && buf[len(buf)-1] == '0' {
		buf = buf[:len(buf)-2]
	}
	return string(buf)
}

func secondsToDuration(sec num.Dec) (time.Duration, error) {
	if sec.Sign < 0 {
		return 0, fmt.Errorf("second value cannot be negative")
	}
	scaled, err := num.DecToScaledIntExact(sec, 9)
	if err != nil {
		return 0, err
	}
	const maxDuration = time.Duration(^uint64(0) >> 1)
	maxSeconds := num.FromInt64(int64(maxDuration))
	if scaled.Compare(maxSeconds) > 0 {
		return 0, fmt.Errorf("second value too large")
	}
	val, ok := int64FromDigits(scaled.Digits)
	if !ok {
		return 0, fmt.Errorf("second value too large")
	}
	if scaled.Sign < 0 {
		val = -val
	}
	return time.Duration(val), nil
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

func int64FromDigits(digits []byte) (int64, bool) {
	if len(digits) == 0 {
		return 0, true
	}
	var n int64
	for _, d := range digits {
		if n > (int64(^uint64(0)>>1)-int64(d-'0'))/10 {
			return 0, false
		}
		n = n*10 + int64(d-'0')
	}
	return n, true
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

// Compare orders durations using the XSD 1.0 order relation for duration.
func (c ComparableXSDDuration) Compare(other ComparableValue) (int, error) {
	otherDur, ok := other.(ComparableXSDDuration)
	if !ok {
		if otherCompDur, ok := other.(ComparableDuration); ok {
			otherDur = ComparableXSDDuration{Value: durationToXSD(otherCompDur.Value), Typ: otherCompDur.Typ}
		} else {
			return 0, fmt.Errorf("cannot compare ComparableXSDDuration with %T", other)
		}
	}

	left := c.Value
	right := otherDur.Value

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
				return 0, errIndeterminateDurationComparison
			}
			sawEqual = true
			continue
		}
		if sawEqual {
			return 0, errIndeterminateDurationComparison
		}
		if sign == 0 {
			sign = cmp
			continue
		}
		if sign != cmp {
			return 0, errIndeterminateDurationComparison
		}
	}
	return sign, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableXSDDuration) String() string {
	var buf strings.Builder
	if c.Value.Negative {
		buf.WriteString("-")
	}
	buf.WriteString("P")
	hasDate := false
	if c.Value.Years != 0 {
		buf.WriteString(fmt.Sprintf("%dY", c.Value.Years))
		hasDate = true
	}
	if c.Value.Months != 0 {
		buf.WriteString(fmt.Sprintf("%dM", c.Value.Months))
		hasDate = true
	}
	if c.Value.Days != 0 {
		buf.WriteString(fmt.Sprintf("%dD", c.Value.Days))
		hasDate = true
	}

	hasTime := c.Value.Hours != 0 || c.Value.Minutes != 0 || c.Value.Seconds.Sign != 0
	if !hasDate && !hasTime {
		return buf.String() + "T0S"
	}

	if hasTime {
		buf.WriteString("T")
		if c.Value.Hours != 0 {
			buf.WriteString(fmt.Sprintf("%dH", c.Value.Hours))
		}
		if c.Value.Minutes != 0 {
			buf.WriteString(fmt.Sprintf("%dM", c.Value.Minutes))
		}
		if c.Value.Seconds.Sign != 0 {
			buf.WriteString(formatDurationSeconds(c.Value.Seconds))
			buf.WriteString("S")
		}
	}
	return buf.String()
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableXSDDuration) Type() Type {
	return c.Typ
}

// Unwrap returns the inner XSDDuration value
func (c ComparableXSDDuration) Unwrap() any {
	return c.Value
}
