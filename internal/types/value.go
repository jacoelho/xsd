package types

import (
	"fmt"
	"math/big"
	"time"
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
	Lexical string
	Native  T
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
	lexical  string
	native   T
	typ      *SimpleType
	toString func(T) string
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
