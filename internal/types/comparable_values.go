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
	Value num.Dec
	// XSD type this value represents
	Typ Type
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
	Value num.Int
	// XSD type this value represents
	Typ Type
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
	Typ         Type
	HasTimezone bool
}

var errIndeterminateTimeComparison = errors.New("time comparison indeterminate")

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableTime) Compare(other ComparableValue) (int, error) {
	otherTime, ok := other.(ComparableTime)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableTime with %T", other)
	}
	if c.HasTimezone == otherTime.HasTimezone {
		if c.Value.Before(otherTime.Value) {
			return -1, nil
		}
		if c.Value.After(otherTime.Value) {
			return 1, nil
		}
		return 0, nil
	}
	if c.HasTimezone {
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
	if c.HasTimezone {
		return c.Value.Format(time.RFC3339Nano)
	}
	return c.Value.Format("2006-01-02T15:04:05.999999999")
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
	if xsdDur.Seconds > 9223372036.854775807 {
		return 0, fmt.Errorf("second value too large: %g", xsdDur.Seconds)
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

	secondsDuration := time.Duration(xsdDur.Seconds * float64(time.Second))
	if secondsDuration < 0 || secondsDuration > maxDuration {
		return 0, fmt.Errorf("second value too large: %g", xsdDur.Seconds)
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
		seconds := float64(durVal) / float64(time.Second)
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
	Negative bool
	Years    int
	Months   int
	Days     int
	Hours    int
	Minutes  int
	Seconds  float64
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
	var seconds float64
	hasDateComponent := false
	hasTimeComponent := false

	// parse date part (years, months, days)
	if datePart != "" {
		matches := durationDatePattern.FindAllStringSubmatch(datePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := strconv.Atoi(match[1])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid year value: %w", err)
				}
				years = val
				hasDateComponent = true
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid month value: %w", err)
				}
				months = val
				hasDateComponent = true
			}
			if match[3] != "" {
				val, err := strconv.Atoi(match[3])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid day value: %w", err)
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
				val, err := strconv.Atoi(match[1])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid hour value: %w", err)
				}
				hours = val
				hasTimeComponent = true
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid minute value: %w", err)
				}
				minutes = val
				hasTimeComponent = true
			}
			if match[3] != "" {
				val, err := strconv.ParseFloat(match[3], 64)
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid second value: %w", err)
				}
				if val < 0 {
					return XSDDuration{}, fmt.Errorf("second value cannot be negative")
				}
				seconds = val
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
	years   int
	months  int
	days    int
	hours   int
	minutes int
	seconds float64
}

type dateTimeFields struct {
	year   int
	month  int
	day    int
	hour   int
	minute int
	second float64
}

// durationOrderReferenceTimes are the XSD 1.0 reference dateTimes for duration ordering.
var durationOrderReferenceTimes = []dateTimeFields{
	{year: 1696, month: 9, day: 1, hour: 0, minute: 0, second: 0},
	{year: 1697, month: 2, day: 1, hour: 0, minute: 0, second: 0},
	{year: 1903, month: 3, day: 1, hour: 0, minute: 0, second: 0},
	{year: 1903, month: 7, day: 1, hour: 0, minute: 0, second: 0},
}

func durationFieldsFor(value XSDDuration) durationFields {
	sign := 1
	if value.Negative {
		sign = -1
	}
	return durationFields{
		years:   sign * value.Years,
		months:  sign * value.Months,
		days:    sign * value.Days,
		hours:   sign * value.Hours,
		minutes: sign * value.Minutes,
		seconds: float64(sign) * value.Seconds,
	}
}

func isDayTimeDuration(value XSDDuration) bool {
	return value.Years == 0 && value.Months == 0
}

func durationTotalSeconds(value XSDDuration) float64 {
	total := float64(value.Days)*86400 +
		float64(value.Hours)*3600 +
		float64(value.Minutes)*60 +
		value.Seconds
	if value.Negative {
		return -total
	}
	return total
}

func compareDayTimeDurations(left, right XSDDuration) int {
	leftSeconds := durationTotalSeconds(left)
	rightSeconds := durationTotalSeconds(right)
	switch {
	case leftSeconds < rightSeconds:
		return -1
	case leftSeconds > rightSeconds:
		return 1
	default:
		return 0
	}
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
	case left.second < right.second:
		return -1
	case left.second > right.second:
		return 1
	default:
		return 0
	}
}

func addDurationToDateTime(start dateTimeFields, dur durationFields) dateTimeFields {
	tempMonth := start.month + dur.months
	month := moduloIntRange(tempMonth, 1, 13)
	carry := fQuotientIntRange(tempMonth, 1, 13)

	year := start.year + dur.years + carry

	tempSecond := start.second + dur.seconds
	second := moduloFloat(tempSecond, 60)
	carry = fQuotientFloat(tempSecond, 60)

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

func fQuotientFloat(a, b float64) int {
	if b == 0 {
		return 0
	}
	return int(math.Floor(a / b))
}

func moduloFloat(a, b float64) float64 {
	return a - float64(fQuotientFloat(a, b))*b
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

	hasTime := c.Value.Hours != 0 || c.Value.Minutes != 0 || c.Value.Seconds != 0
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
		if c.Value.Seconds != 0 {
			seconds := strconv.FormatFloat(c.Value.Seconds, 'f', -1, 64)
			buf.WriteString(seconds)
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
