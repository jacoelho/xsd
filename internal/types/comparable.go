package types

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// ComparableBigRat wraps *big.Rat to implement ComparableValue
type ComparableBigRat struct {
	Value *big.Rat
	// XSD type this value represents
	Typ Type
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Supports cross-type comparison with ComparableBigInt since integers are a subset of decimals.
func (c ComparableBigRat) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableBigRat:
		return c.Value.Cmp(otherVal.Value), nil
	case ComparableBigInt:
		otherRat := new(big.Rat).SetInt(otherVal.Value)
		return c.Value.Cmp(otherRat), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableBigRat with %T", other)
	}
}

// String returns the string representation (implements ComparableValue)
func (c ComparableBigRat) String() string {
	return c.Value.String()
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableBigRat) Type() Type {
	return c.Typ
}

// Unwrap returns the inner *big.Rat value
func (c ComparableBigRat) Unwrap() any {
	return c.Value
}

// ComparableBigInt wraps *big.Int to implement ComparableValue
type ComparableBigInt struct {
	Value *big.Int
	// XSD type this value represents
	Typ Type
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Supports cross-type comparison with ComparableBigRat since integers are a subset of decimals.
func (c ComparableBigInt) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableBigInt:
		return c.Value.Cmp(otherVal.Value), nil
	case ComparableBigRat:
		thisRat := new(big.Rat).SetInt(c.Value)
		return thisRat.Cmp(otherVal.Value), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableBigInt with %T", other)
	}
}

// String returns the string representation (implements ComparableValue)
func (c ComparableBigInt) String() string {
	return c.Value.String()
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableBigInt) Type() Type {
	return c.Typ
}

// Unwrap returns the inner *big.Int value
func (c ComparableBigInt) Unwrap() any {
	return c.Value
}

// ComparableTime wraps time.Time to implement ComparableValue
type ComparableTime struct {
	Value time.Time
	// XSD type this value represents
	Typ Type
}

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableTime) Compare(other ComparableValue) (int, error) {
	otherTime, ok := other.(ComparableTime)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableTime with %T", other)
	}
	if c.Value.Before(otherTime.Value) {
		return -1, nil
	}
	if c.Value.After(otherTime.Value) {
		return 1, nil
	}
	return 0, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableTime) String() string {
	return c.Value.Format(time.RFC3339Nano)
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
	Value float64
	// XSD type this value represents
	Typ Type
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
	Value float32
	// XSD type this value represents
	Typ Type
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
	Value time.Duration
	// XSD type this value represents
	Typ Type
}

// ParseDurationToTimeDuration parses an XSD duration string into a time.Duration
// Returns an error if the duration contains years or months (which cannot be converted to time.Duration)
// or if the duration string is invalid.
func ParseDurationToTimeDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	if len(s) == 0 || s[0] != 'P' {
		return 0, fmt.Errorf("duration must start with P")
	}
	s = s[1:]

	datePart := s
	timePart := ""
	if before, after, ok := strings.Cut(s, "T"); ok {
		datePart = before
		timePart = after
		if strings.IndexByte(timePart, 'T') != -1 {
			return 0, fmt.Errorf("invalid duration format: multiple T separators")
		}
	}

	var years, months, days, hours, minutes int
	var seconds float64

	// parse date part (years, months, days)
	if datePart != "" {
		matches := durationDatePattern.FindAllStringSubmatch(datePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := strconv.Atoi(match[1])
				if err != nil {
					return 0, fmt.Errorf("invalid year value: %w", err)
				}
				years = val
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return 0, fmt.Errorf("invalid month value: %w", err)
				}
				months = val
			}
			if match[3] != "" {
				val, err := strconv.Atoi(match[3])
				if err != nil {
					return 0, fmt.Errorf("invalid day value: %w", err)
				}
				days = val
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
					return 0, fmt.Errorf("invalid hour value: %w", err)
				}
				hours = val
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return 0, fmt.Errorf("invalid minute value: %w", err)
				}
				minutes = val
			}
			if match[3] != "" {
				val, err := strconv.ParseFloat(match[3], 64)
				if err != nil {
					return 0, fmt.Errorf("invalid second value: %w", err)
				}
				if val < 0 {
					return 0, fmt.Errorf("second value cannot be negative")
				}
				// max seconds that fit: ~292 years
				if val > 9223372036.854775807 {
					return 0, fmt.Errorf("second value too large: %v", val)
				}
				seconds = val
			}
		}
	}

	// check if duration has years or months (cannot convert to time.Duration)
	if years != 0 || months != 0 {
		return 0, fmt.Errorf("durations with years or months cannot be converted to time.Duration (indeterminate)")
	}

	// check if we actually parsed any components
	// "P" and "PT" without any components are invalid
	// but "PT0S" or "P0D" are valid (explicit zero)
	hasAnyComponent := false
	if datePart != "" {
		// check if datePart contains any component markers
		if strings.Contains(datePart, "Y") || strings.Contains(datePart, "M") || strings.Contains(datePart, "D") {
			hasAnyComponent = true
		}
	}
	if timePart != "" {
		// check if timePart contains any component markers
		if strings.Contains(timePart, "H") || strings.Contains(timePart, "M") || strings.Contains(timePart, "S") {
			hasAnyComponent = true
		}
	}
	if !hasAnyComponent {
		return 0, fmt.Errorf("duration must have at least one component")
	}

	// note: PT0S is a valid XSD duration representing zero, so we allow all zeros
	dur := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))

	if negative {
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
		durVal = durVal % time.Hour
		minutes := int(durVal / time.Minute)
		durVal = durVal % time.Minute
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
	Value XSDDuration
	// XSD type this value represents
	Typ Type
}

// ParseXSDDuration parses an XSD duration string into an XSDDuration struct
// Supports all XSD duration components including years and months
func ParseXSDDuration(s string) (XSDDuration, error) {
	if len(s) == 0 {
		return XSDDuration{}, fmt.Errorf("empty duration")
	}

	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	if len(s) == 0 || s[0] != 'P' {
		return XSDDuration{}, fmt.Errorf("duration must start with P")
	}
	s = s[1:]

	datePart := s
	timePart := ""
	if before, after, ok := strings.Cut(s, "T"); ok {
		datePart = before
		timePart = after
		if strings.IndexByte(timePart, 'T') != -1 {
			return XSDDuration{}, fmt.Errorf("invalid duration format: multiple T separators")
		}
	}

	var years, months, days, hours, minutes int
	var seconds float64

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
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid month value: %w", err)
				}
				months = val
			}
			if match[3] != "" {
				val, err := strconv.Atoi(match[3])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid day value: %w", err)
				}
				days = val
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
			}
			if match[2] != "" {
				val, err := strconv.Atoi(match[2])
				if err != nil {
					return XSDDuration{}, fmt.Errorf("invalid minute value: %w", err)
				}
				minutes = val
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
			}
		}
	}

	// check if we actually parsed any components
	hasAnyComponent := false
	if datePart != "" {
		if strings.Contains(datePart, "Y") || strings.Contains(datePart, "M") || strings.Contains(datePart, "D") {
			hasAnyComponent = true
		}
	}
	if timePart != "" {
		if strings.Contains(timePart, "H") || strings.Contains(timePart, "M") || strings.Contains(timePart, "S") {
			hasAnyComponent = true
		}
	}
	if !hasAnyComponent {
		return XSDDuration{}, fmt.Errorf("duration must have at least one component")
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

// Compare orders durations component-wise per XSD's partial order.
// It compares years, months, days, hours, minutes, then seconds.
func (c ComparableXSDDuration) Compare(other ComparableValue) (int, error) {
	otherDur, ok := other.(ComparableXSDDuration)
	if !ok {
		// try to compare with ComparableDuration (pure day/time durations)
		if otherCompDur, ok := other.(ComparableDuration); ok {
			// convert this XSD duration to time.Duration if possible (no years/months)
			if c.Value.Years != 0 || c.Value.Months != 0 {
				return 0, fmt.Errorf("cannot compare XSD duration with years/months to pure time.Duration")
			}
			thisDur := time.Duration(c.Value.Days)*24*time.Hour +
				time.Duration(c.Value.Hours)*time.Hour +
				time.Duration(c.Value.Minutes)*time.Minute +
				time.Duration(c.Value.Seconds*float64(time.Second))
			if c.Value.Negative {
				thisDur = -thisDur
			}
			if thisDur < otherCompDur.Value {
				return -1, nil
			}
			if thisDur > otherCompDur.Value {
				return 1, nil
			}
			return 0, nil
		}
		return 0, fmt.Errorf("cannot compare ComparableXSDDuration with %T", other)
	}

	cVal := c.Value
	oVal := otherDur.Value

	if cVal.Negative && !oVal.Negative {
		return -1, nil
	}
	if !cVal.Negative && oVal.Negative {
		return 1, nil
	}

	// both have same sign, compare component-wise
	// for negative durations, reverse the comparison
	multiplier := 1
	if cVal.Negative {
		multiplier = -1
	}

	// compare years
	if cVal.Years != oVal.Years {
		if cVal.Years < oVal.Years {
			return -1 * multiplier, nil
		}
		return 1 * multiplier, nil
	}

	// compare months
	if cVal.Months != oVal.Months {
		if cVal.Months < oVal.Months {
			return -1 * multiplier, nil
		}
		return 1 * multiplier, nil
	}

	// compare days
	if cVal.Days != oVal.Days {
		if cVal.Days < oVal.Days {
			return -1 * multiplier, nil
		}
		return 1 * multiplier, nil
	}

	// compare hours
	if cVal.Hours != oVal.Hours {
		if cVal.Hours < oVal.Hours {
			return -1 * multiplier, nil
		}
		return 1 * multiplier, nil
	}

	// compare minutes
	if cVal.Minutes != oVal.Minutes {
		if cVal.Minutes < oVal.Minutes {
			return -1 * multiplier, nil
		}
		return 1 * multiplier, nil
	}

	// compare seconds
	if cVal.Seconds < oVal.Seconds {
		return -1 * multiplier, nil
	}
	if cVal.Seconds > oVal.Seconds {
		return 1 * multiplier, nil
	}

	return 0, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableXSDDuration) String() string {
	var buf strings.Builder
	if c.Value.Negative {
		buf.WriteString("-")
	}
	buf.WriteString("P")
	if c.Value.Years != 0 {
		buf.WriteString(fmt.Sprintf("%dY", c.Value.Years))
	}
	if c.Value.Months != 0 {
		buf.WriteString(fmt.Sprintf("%dM", c.Value.Months))
	}
	if c.Value.Days != 0 {
		buf.WriteString(fmt.Sprintf("%dD", c.Value.Days))
	}
	hasTime := c.Value.Hours != 0 || c.Value.Minutes != 0 || c.Value.Seconds != 0
	if hasTime {
		buf.WriteString("T")
		if c.Value.Hours != 0 {
			buf.WriteString(fmt.Sprintf("%dH", c.Value.Hours))
		}
		if c.Value.Minutes != 0 {
			buf.WriteString(fmt.Sprintf("%dM", c.Value.Minutes))
		}
		if c.Value.Seconds != 0 {
			buf.WriteString(fmt.Sprintf("%gS", c.Value.Seconds))
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
