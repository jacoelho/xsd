package types

import (
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// TypedValue represents a value with its XSD type
// It stores both the original lexical representation (for error messages, PSVI)
// and the parsed native Go type (for efficient validation)
type TypedValue interface {
	// Type returns the XSD type this value belongs to
	Type() Type

	// Lexical returns the original lexical representation
	Lexical() string

	// Native returns the native Go type representation
	Native() any

	// String returns a canonical string representation
	String() string
}

// ParsedValue captures a normalized lexical value and its parsed native form.
type ParsedValue[T any] struct {
	Native  T
	Lexical string
}

// NewParsedValue constructs a ParsedValue from lexical and native values.
func NewParsedValue[T any](lexical string, native T) ParsedValue[T] {
	return ParsedValue[T]{
		Lexical: lexical,
		Native:  native,
	}
}

// ValueNormalizer normalizes lexical values based on type rules.
type ValueNormalizer interface {
	Normalize(lexical string, typ Type) (string, error)
}

var defaultValueNormalizer ValueNormalizer = whiteSpaceNormalizer{}

var builtinValueNormalizers = map[TypeName]ValueNormalizer{
	TypeNameDateTime:   dateTimeNormalizer{},
	TypeNameDate:       dateTimeNormalizer{},
	TypeNameTime:       dateTimeNormalizer{},
	TypeNameGYearMonth: dateTimeNormalizer{},
	TypeNameGYear:      dateTimeNormalizer{},
	TypeNameGMonthDay:  dateTimeNormalizer{},
	TypeNameGDay:       dateTimeNormalizer{},
	TypeNameGMonth:     dateTimeNormalizer{},
}

// NormalizeValue normalizes lexical values based on their type rules.
func NormalizeValue(lexical string, typ Type) (string, error) {
	if typ == nil {
		return lexical, fmt.Errorf("cannot normalize value for nil type")
	}
	return normalizerForType(typ).Normalize(lexical, typ)
}

func normalizerForType(typ Type) ValueNormalizer {
	if typ == nil {
		return defaultValueNormalizer
	}
	if typ.IsBuiltin() {
		if normalizer, ok := builtinValueNormalizers[TypeName(typ.Name().Local)]; ok {
			return normalizer
		}
	}
	if bt, ok := as[*BuiltinType](typ); ok {
		if normalizer, ok := builtinValueNormalizers[TypeName(bt.Name().Local)]; ok {
			return normalizer
		}
		return defaultValueNormalizer
	}
	if primitive := typ.PrimitiveType(); primitive != nil {
		if normalizer, ok := builtinValueNormalizers[TypeName(primitive.Name().Local)]; ok {
			return normalizer
		}
	}
	return defaultValueNormalizer
}

type simpleValue[T any] struct {
	native   T
	typ      *SimpleType
	toString func(T) string
	lexical  string
}

func newSimpleValue[T any](parsed ParsedValue[T], typ *SimpleType, toString func(T) string) simpleValue[T] {
	return simpleValue[T]{
		lexical:  parsed.Lexical,
		native:   parsed.Native,
		typ:      typ,
		toString: toString,
	}
}

func (v *simpleValue[T]) Type() Type { return v.typ }

func (v *simpleValue[T]) Lexical() string { return v.lexical }

func (v *simpleValue[T]) Native() any { return v.native }

func (v *simpleValue[T]) String() string {
	if v.toString != nil {
		return v.toString(v.native)
	}
	return fmt.Sprint(v.native)
}

// DecimalValue represents a decimal value
type DecimalValue struct {
	simpleValue[*big.Rat]
}

// NewDecimalValue creates a new DecimalValue
func NewDecimalValue(parsed ParsedValue[*big.Rat], typ *SimpleType) TypedValue {
	return &DecimalValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// IntegerValue represents an integer value
type IntegerValue struct {
	simpleValue[*big.Int]
}

// NewIntegerValue creates a new IntegerValue
func NewIntegerValue(parsed ParsedValue[*big.Int], typ *SimpleType) TypedValue {
	return &IntegerValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// BooleanValue represents a boolean value
type BooleanValue struct {
	simpleValue[bool]
}

// NewBooleanValue creates a new BooleanValue
func NewBooleanValue(parsed ParsedValue[bool], typ *SimpleType) TypedValue {
	return &BooleanValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// DateTimeValue represents a dateTime value
type DateTimeValue struct {
	simpleValue[time.Time]
}

// NewDateTimeValue creates a new DateTimeValue
func NewDateTimeValue(parsed ParsedValue[time.Time], typ *SimpleType) TypedValue {
	return &DateTimeValue{
		simpleValue: newSimpleValue(parsed, typ, func(value time.Time) string {
			return value.Format(time.RFC3339Nano)
		}),
	}
}

// FloatValue represents a float value
type FloatValue struct {
	simpleValue[float32]
}

// NewFloatValue creates a new FloatValue
func NewFloatValue(parsed ParsedValue[float32], typ *SimpleType) TypedValue {
	return &FloatValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// DoubleValue represents a double value
type DoubleValue struct {
	simpleValue[float64]
}

// NewDoubleValue creates a new DoubleValue
func NewDoubleValue(parsed ParsedValue[float64], typ *SimpleType) TypedValue {
	return &DoubleValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// StringValue represents a string value
type StringValue struct {
	simpleValue[string]
}

// NewStringValue creates a new StringValue
func NewStringValue(parsed ParsedValue[string], typ *SimpleType) TypedValue {
	return &StringValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// LongValue represents a long value
type LongValue struct {
	simpleValue[int64]
}

// NewLongValue creates a new LongValue
func NewLongValue(parsed ParsedValue[int64], typ *SimpleType) TypedValue {
	return &LongValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// IntValue represents an int value
type IntValue struct {
	simpleValue[int32]
}

// NewIntValue creates a new IntValue
func NewIntValue(parsed ParsedValue[int32], typ *SimpleType) TypedValue {
	return &IntValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// ShortValue represents a short value
type ShortValue struct {
	simpleValue[int16]
}

// NewShortValue creates a new ShortValue
func NewShortValue(parsed ParsedValue[int16], typ *SimpleType) TypedValue {
	return &ShortValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// ByteValue represents a byte value
type ByteValue struct {
	simpleValue[int8]
}

// NewByteValue creates a new ByteValue
func NewByteValue(parsed ParsedValue[int8], typ *SimpleType) TypedValue {
	return &ByteValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// UnsignedLongValue represents an unsignedLong value
type UnsignedLongValue struct {
	simpleValue[uint64]
}

// NewUnsignedLongValue creates a new UnsignedLongValue
func NewUnsignedLongValue(parsed ParsedValue[uint64], typ *SimpleType) TypedValue {
	return &UnsignedLongValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// UnsignedIntValue represents an unsignedInt value
type UnsignedIntValue struct {
	simpleValue[uint32]
}

// NewUnsignedIntValue creates a new UnsignedIntValue
func NewUnsignedIntValue(parsed ParsedValue[uint32], typ *SimpleType) TypedValue {
	return &UnsignedIntValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// UnsignedShortValue represents an unsignedShort value
type UnsignedShortValue struct {
	simpleValue[uint16]
}

// NewUnsignedShortValue creates a new UnsignedShortValue
func NewUnsignedShortValue(parsed ParsedValue[uint16], typ *SimpleType) TypedValue {
	return &UnsignedShortValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// UnsignedByteValue represents an unsignedByte value
type UnsignedByteValue struct {
	simpleValue[uint8]
}

// NewUnsignedByteValue creates a new UnsignedByteValue
func NewUnsignedByteValue(parsed ParsedValue[uint8], typ *SimpleType) TypedValue {
	return &UnsignedByteValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

// ValueAs extracts the native value from a TypedValue with type safety.
// Returns an error if the value type doesn't match the requested type.
func ValueAs[T any](value TypedValue) (T, error) {
	var zero T
	native := value.Native()

	// for Comparable wrapper types, extract the inner value
	if nativeVal, ok := as[T](native); ok {
		return nativeVal, nil
	}
	if unwrappable, ok := as[Unwrappable](native); ok {
		unwrapped := unwrappable.Unwrap()
		if nativeVal, ok := as[T](unwrapped); ok {
			return nativeVal, nil
		}
	}

	// get XSD type name for user-friendly error message
	var xsdTypeName string
	if value != nil && value.Type() != nil {
		xsdTypeName = value.Type().Name().Local
	} else {
		xsdTypeName = "unknown"
	}
	return zero, fmt.Errorf("cannot convert value of type %s", xsdTypeName)
}

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
	Typ   Type
	Value XSDDuration
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

// ParseDecimal parses a decimal string into *big.Rat
// Handles leading/trailing whitespace and validates decimal format
func ParseDecimal(lexical string) (*big.Rat, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}

	rat := new(big.Rat)
	if _, ok := rat.SetString(lexical); !ok {
		return nil, fmt.Errorf("invalid decimal: %s", lexical)
	}
	return rat, nil
}

// ParseInteger parses an integer string into *big.Int
// Handles leading/trailing whitespace and validates integer format
func ParseInteger(lexical string) (*big.Int, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return nil, fmt.Errorf("invalid integer: empty string")
	}

	intVal := new(big.Int)
	if _, ok := intVal.SetString(lexical, 10); !ok {
		return nil, fmt.Errorf("invalid integer: %s", lexical)
	}
	return intVal, nil
}

// ParseBoolean parses a boolean string into bool
// Accepts "true", "false", "1", "0" (XSD boolean lexical representation)
func ParseBoolean(lexical string) (bool, error) {
	lexical = strings.TrimSpace(lexical)
	switch lexical {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s (must be 'true', 'false', '1', or '0')", lexical)
	}
}

// ParseFloat parses a float string into float32 with special value handling
// Handles INF, -INF, and NaN special values
func ParseFloat(lexical string) (float32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid float: empty string")
	}

	switch lexical {
	case "INF", "+INF":
		return float32(math.Inf(1)), nil
	case "-INF":
		return float32(math.Inf(-1)), nil
	case "NaN":
		return float32(math.NaN()), nil
	default:
		f, err := strconv.ParseFloat(lexical, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid float: %s", lexical)
		}
		return float32(f), nil
	}
}

// ParseDouble parses a double string into float64 with special value handling
// Handles INF, -INF, and NaN special values
func ParseDouble(lexical string) (float64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid double: empty string")
	}

	switch lexical {
	case "INF", "+INF":
		return math.Inf(1), nil
	case "-INF":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	default:
		f, err := strconv.ParseFloat(lexical, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid double: %s", lexical)
		}
		return f, nil
	}
}

// ParseDateTime parses a dateTime string into time.Time
// Supports various ISO 8601 formats with and without timezone
func ParseDateTime(lexical string) (time.Time, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid dateTime: empty string")
	}

	// try various ISO 8601 formats
	formats := []string{
		time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05.999999999", // with nanoseconds, no timezone
		"2006-01-02T15:04:05.999999",    // with microseconds, no timezone
		"2006-01-02T15:04:05.999",       // with milliseconds, no timezone
		"2006-01-02T15:04:05",           // no fractional seconds, no timezone
		"2006-01-02T15:04:05Z",          // no fractional seconds, UTC
		"2006-01-02T15:04:05-07:00",     // no fractional seconds, with timezone
		"2006-01-02T15:04:05+07:00",     // no fractional seconds, with timezone
		"2006-01-02T15:04:05.999Z",      // with milliseconds, UTC
		"2006-01-02T15:04:05.999-07:00", // with milliseconds, with timezone
		"2006-01-02T15:04:05.999+07:00", // with milliseconds, with timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, lexical); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid dateTime: %s", lexical)
}

// ParseLong parses a long string into int64
func ParseLong(lexical string) (int64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid long: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid long: %s", lexical)
	}
	return val, nil
}

// ParseInt parses an int string into int32
func ParseInt(lexical string) (int32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid int: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid int: %s", lexical)
	}
	return int32(val), nil
}

// ParseShort parses a short string into int16
func ParseShort(lexical string) (int16, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid short: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid short: %s", lexical)
	}
	return int16(val), nil
}

// ParseByte parses a byte string into int8
func ParseByte(lexical string) (int8, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid byte: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid byte: %s", lexical)
	}
	return int8(val), nil
}

// ParseUnsignedLong parses an unsignedLong string into uint64
func ParseUnsignedLong(lexical string) (uint64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedLong: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", lexical)
	}
	return val, nil
}

// ParseUnsignedInt parses an unsignedInt string into uint32
func ParseUnsignedInt(lexical string) (uint32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedInt: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", lexical)
	}
	return uint32(val), nil
}

// ParseUnsignedShort parses an unsignedShort string into uint16
func ParseUnsignedShort(lexical string) (uint16, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedShort: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", lexical)
	}
	return uint16(val), nil
}

// ParseUnsignedByte parses an unsignedByte string into uint8
func ParseUnsignedByte(lexical string) (uint8, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedByte: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", lexical)
	}
	return uint8(val), nil
}

// ParseString parses a string (no-op, returns as-is)
func ParseString(lexical string) (string, error) {
	return lexical, nil
}

// ParseHexBinary parses a hexBinary string into []byte
func ParseHexBinary(lexical string) ([]byte, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return nil, nil
	}
	if len(lexical)%2 != 0 {
		return nil, fmt.Errorf("invalid hexBinary: odd length")
	}
	data := make([]byte, len(lexical)/2)
	for i := 0; i < len(lexical); i += 2 {
		b, err := strconv.ParseUint(lexical[i:i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hexBinary: %s", lexical)
		}
		data[i/2] = byte(b)
	}
	return data, nil
}

// ParseBase64Binary parses a base64Binary string into []byte
func ParseBase64Binary(lexical string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, lexical)

	if cleaned == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("invalid base64Binary: %s", lexical)
		}
	}
	return decoded, nil
}

// ParseAnyURI parses an anyURI string (no validation beyond trimming)
func ParseAnyURI(lexical string) (string, error) {
	return strings.TrimSpace(lexical), nil
}

// ParseQNameValue parses a QName value (lexical string) into a QName with namespace resolution.
func ParseQNameValue(lexical string, nsContext map[string]string) (QName, error) {
	trimmed := strings.TrimSpace(lexical)
	if trimmed == "" {
		return QName{}, fmt.Errorf("invalid QName: empty string")
	}

	prefix, local, hasPrefix, err := ParseQName(trimmed)
	if err != nil {
		return QName{}, err
	}

	var ns NamespaceURI
	if hasPrefix {
		var ok bool
		ns, ok = ResolveNamespace(prefix, nsContext)
		if !ok {
			return QName{}, fmt.Errorf("prefix %s not found in namespace context", prefix)
		}
	}

	return QName{Namespace: ns, Local: local}, nil
}

// ParseNOTATION parses a NOTATION value (lexical string) into a QName with namespace resolution.
func ParseNOTATION(lexical string, nsContext map[string]string) (QName, error) {
	return ParseQNameValue(lexical, nsContext)
}

// measureLengthForPrimitive measures length for primitive types.
func measureLengthForPrimitive(value string, primitiveName TypeName) int {
	switch primitiveName {
	case TypeNameHexBinary:
		// hexBinary: each pair of hex characters = 1 octet
		if len(value) == 0 {
			return 0
		}
		if len(value)%2 != 0 {
			// invalid hexBinary - return character count as fallback
			return utf8.RuneCountInString(value)
		}
		return len(value) / 2

	case TypeNameBase64Binary:
		// base64Binary: length is the number of octets it contains
		if len(value) == 0 {
			return 0
		}
		cleaned := strings.Map(func(r rune) rune {
			switch r {
			case ' ', '\t', '\n', '\r':
				return -1
			default:
				return r
			}
		}, value)

		// decode to get actual byte length
		decoded, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			// try URL encoding variant
			decoded, err = base64.URLEncoding.DecodeString(cleaned)
			if err != nil {
				// invalid base64 - return character count as fallback
				return utf8.RuneCountInString(value)
			}
		}
		return len(decoded)
	}

	// for all other types, length is in characters (Unicode code points)
	return utf8.RuneCountInString(value)
}

// isBuiltinListType checks if a type name is a built-in list type.
func isBuiltinListType(name string) bool {
	return name == string(TypeNameNMTOKENS) ||
		name == string(TypeNameIDREFS) ||
		name == string(TypeNameENTITIES)
}
