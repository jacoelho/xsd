package types

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/num"
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
	simpleValue[num.Dec]
}

// NewDecimalValue creates a new DecimalValue
func NewDecimalValue(parsed ParsedValue[num.Dec], typ *SimpleType) TypedValue {
	return &DecimalValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *DecimalValue) String() string {
	return canonicalDecimalString(v.lexical)
}

// XSDDurationValue represents a duration value.
type XSDDurationValue struct {
	simpleValue[XSDDuration]
}

// NewXSDDurationValue creates a new XSDDurationValue.
func NewXSDDurationValue(parsed ParsedValue[XSDDuration], typ *SimpleType) TypedValue {
	return &XSDDurationValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *XSDDurationValue) String() string {
	if v == nil {
		return ""
	}
	return ComparableXSDDuration{Value: v.native, Typ: v.typ}.String()
}

func canonicalDecimalString(lexical string) string {
	s := TrimXMLWhitespace(lexical)
	if s == "" {
		return s
	}
	sign := ""
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		sign = "-"
		s = s[1:]
	}

	intPart := s
	fracPart := ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart = s[:dot]
		fracPart = s[dot+1:]
	}

	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}

	fracPart = strings.TrimRight(fracPart, "0")
	if fracPart == "" {
		fracPart = "0"
	}

	if isAllZeros(intPart) && isAllZeros(fracPart) {
		sign = ""
		intPart = "0"
		fracPart = "0"
	}

	return sign + intPart + "." + fracPart
}

func isAllZeros(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// IntegerValue represents an integer value
type IntegerValue struct {
	simpleValue[num.Int]
}

// NewIntegerValue creates a new IntegerValue
func NewIntegerValue(parsed ParsedValue[num.Int], typ *SimpleType) TypedValue {
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

// HexBinaryValue represents a hexBinary value.
type HexBinaryValue struct {
	simpleValue[[]byte]
}

// NewHexBinaryValue creates a new HexBinaryValue.
func NewHexBinaryValue(parsed ParsedValue[[]byte], typ *SimpleType) TypedValue {
	return &HexBinaryValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *HexBinaryValue) String() string {
	if v == nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(v.native))
}

// Base64BinaryValue represents a base64Binary value.
type Base64BinaryValue struct {
	simpleValue[[]byte]
}

// NewBase64BinaryValue creates a new Base64BinaryValue.
func NewBase64BinaryValue(parsed ParsedValue[[]byte], typ *SimpleType) TypedValue {
	return &Base64BinaryValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *Base64BinaryValue) String() string {
	if v == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(v.native)
}

// DateTimeValue represents a dateTime value
type DateTimeValue struct {
	simpleValue[time.Time]
	kind        TypeName
	hasTimezone bool
}

// NewDateTimeValue creates a new DateTimeValue
func NewDateTimeValue(parsed ParsedValue[time.Time], typ *SimpleType) TypedValue {
	kind := TypeNameDateTime
	if typ != nil {
		if primitive := typ.PrimitiveType(); primitive != nil {
			kind = TypeName(primitive.Name().Local)
		} else {
			kind = TypeName(typ.Name().Local)
		}
	}
	return &DateTimeValue{
		simpleValue: newSimpleValue(parsed, typ, nil),
		kind:        kind,
		hasTimezone: HasTimezone(parsed.Lexical),
	}
}

func (v *DateTimeValue) String() string {
	return canonicalDateTimeString(v.native, v.kind, v.hasTimezone)
}

func canonicalDateTimeString(value time.Time, kind TypeName, hasTimezone bool) string {
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	fraction := formatFraction(value.Nanosecond())
	tz := ""
	if hasTimezone {
		tz = formatTimezone(value)
	}

	switch kind {
	case TypeNameDateTime:
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s%s", year, int(month), day, hour, minute, second, fraction, tz)
	case TypeNameDate:
		return fmt.Sprintf("%04d-%02d-%02d%s", year, int(month), day, tz)
	case TypeNameTime:
		return fmt.Sprintf("%02d:%02d:%02d%s%s", hour, minute, second, fraction, tz)
	case TypeNameGYearMonth:
		return fmt.Sprintf("%04d-%02d%s", year, int(month), tz)
	case TypeNameGYear:
		return fmt.Sprintf("%04d%s", year, tz)
	case TypeNameGMonthDay:
		return fmt.Sprintf("--%02d-%02d%s", int(month), day, tz)
	case TypeNameGMonth:
		return fmt.Sprintf("--%02d%s", int(month), tz)
	case TypeNameGDay:
		return fmt.Sprintf("---%02d%s", day, tz)
	default:
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s%s", year, int(month), day, hour, minute, second, fraction, tz)
	}
}

func formatFraction(nanos int) string {
	if nanos == 0 {
		return ""
	}
	frac := fmt.Sprintf("%09d", nanos)
	frac = strings.TrimRight(frac, "0")
	return "." + frac
}

func formatTimezone(value time.Time) string {
	_, offset := value.Zone()
	if offset == 0 {
		return "Z"
	}
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

// FloatValue represents a float value
type FloatValue struct {
	simpleValue[float32]
}

// NewFloatValue creates a new FloatValue
func NewFloatValue(parsed ParsedValue[float32], typ *SimpleType) TypedValue {
	return &FloatValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *FloatValue) String() string {
	return canonicalFloat(float64(v.native), 32)
}

// DoubleValue represents a double value
type DoubleValue struct {
	simpleValue[float64]
}

// NewDoubleValue creates a new DoubleValue
func NewDoubleValue(parsed ParsedValue[float64], typ *SimpleType) TypedValue {
	return &DoubleValue{simpleValue: newSimpleValue(parsed, typ, nil)}
}

func (v *DoubleValue) String() string {
	return canonicalFloat(v.native, 64)
}

func canonicalFloat(value float64, bits int) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "INF"
	}
	if math.IsInf(value, -1) {
		return "-INF"
	}
	if value == 0 {
		return "0.0E0"
	}
	raw := strconv.FormatFloat(value, 'E', -1, bits)
	parts := strings.SplitN(raw, "E", 2)
	mantissa := parts[0]
	exponent := "0"
	if len(parts) == 2 {
		exponent = parts[1]
	}

	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		i := len(mantissa) - 1
		for i > dot+1 && mantissa[i] == '0' {
			i--
		}
		mantissa = mantissa[:i+1]
	}

	expVal, err := strconv.Atoi(exponent)
	if err != nil {
		return mantissa + "E" + exponent
	}
	return mantissa + "E" + strconv.Itoa(expVal)
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
	if value == nil {
		return zero, fmt.Errorf("cannot convert nil value")
	}
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
	xsdTypeName := "unknown"
	if typ := value.Type(); typ != nil {
		xsdTypeName = typ.Name().Local
	}
	return zero, fmt.Errorf("cannot convert value of type %s", xsdTypeName)
}
