package types

import (
	"math"
	"math/big"
	"testing"
	"time"
)

func TestTypedValue_Decimal(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameDecimal)

	lexical := "123.456"
	native, err := ParseDecimal(lexical)
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}

	value := NewDecimalValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[*big.Rat](value)
	if err != nil {
		t.Errorf("ValueAs[*big.Rat]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[*big.Rat]() = %v, want %v", extracted, native)
	}

	// test type mismatch
	_, err = ValueAs[bool](value)
	if err == nil {
		t.Error("ValueAs[bool]() should return error for type mismatch")
	}
}

func TestTypedValue_Boolean(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameBoolean)

	lexical := "true"
	native, err := ParseBoolean(lexical)
	if err != nil {
		t.Fatalf("ParseBoolean() error = %v", err)
	}

	value := NewBooleanValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() != "true" {
		t.Errorf("String() = %v, want 'true'", value.String())
	}

	// test type-safe extraction
	extracted, err := ValueAs[bool](value)
	if err != nil {
		t.Errorf("ValueAs[bool]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[bool]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_DateTime(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameDateTime)

	lexical := "2001-10-26T21:32:52"
	native, err := ParseDateTime(lexical)
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}

	value := NewDateTimeValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[time.Time](value)
	if err != nil {
		t.Errorf("ValueAs[time.Time]() error = %v", err)
	}
	if !extracted.Equal(native) {
		t.Errorf("ValueAs[time.Time]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_Integer(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameInteger)

	lexical := "12345678901234567890"
	native, err := ParseInteger(lexical)
	if err != nil {
		t.Fatalf("ParseInteger() error = %v", err)
	}

	value := NewIntegerValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[*big.Int](value)
	if err != nil {
		t.Errorf("ValueAs[*big.Int]() error = %v", err)
	}
	if extracted.Cmp(native) != 0 {
		t.Errorf("ValueAs[*big.Int]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_Float(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameFloat)

	lexical := "123.456"
	native, err := ParseFloat(lexical)
	if err != nil {
		t.Fatalf("ParseFloat() error = %v", err)
	}

	value := NewFloatValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[float32](value)
	if err != nil {
		t.Errorf("ValueAs[float32]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[float32]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_String(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameString)

	lexical := "hello world"
	native, err := ParseString(lexical)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	value := NewStringValue(NewParsedValue(lexical, native), typ)

	if value.Type() != typ {
		t.Errorf("Type() = %v, want %v", value.Type(), typ)
	}
	if value.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", value.Lexical(), lexical)
	}
	if value.Native() != native {
		t.Errorf("Native() = %v, want %v", value.Native(), native)
	}
	if value.String() != lexical {
		t.Errorf("String() = %v, want %v", value.String(), lexical)
	}

	// test type-safe extraction
	extracted, err := ValueAs[string](value)
	if err != nil {
		t.Errorf("ValueAs[string]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[string]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_CanonicalNumericString(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)
	decNative, err := ParseDecimal("1")
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}
	decValue := NewDecimalValue(NewParsedValue("1", decNative), decimalType)
	if decValue.String() != "1.0" {
		t.Fatalf("decimal String() = %q, want %q", decValue.String(), "1.0")
	}

	decNative, err = ParseDecimal("001.500")
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}
	decValue = NewDecimalValue(NewParsedValue("001.500", decNative), decimalType)
	if decValue.String() != "1.5" {
		t.Fatalf("decimal String() = %q, want %q", decValue.String(), "1.5")
	}

	floatType := mustBuiltinSimpleType(t, TypeNameFloat)
	floatValue := NewFloatValue(NewParsedValue("1.5", float32(1.5)), floatType)
	if floatValue.String() != "1.5E0" {
		t.Fatalf("float String() = %q, want %q", floatValue.String(), "1.5E0")
	}
	floatInf := NewFloatValue(NewParsedValue("INF", float32(math.Inf(1))), floatType)
	if floatInf.String() != "INF" {
		t.Fatalf("float String() = %q, want %q", floatInf.String(), "INF")
	}
	floatNegInf := NewFloatValue(NewParsedValue("-INF", float32(math.Inf(-1))), floatType)
	if floatNegInf.String() != "-INF" {
		t.Fatalf("float String() = %q, want %q", floatNegInf.String(), "-INF")
	}
	floatNaN := NewFloatValue(NewParsedValue("NaN", float32(math.NaN())), floatType)
	if floatNaN.String() != "NaN" {
		t.Fatalf("float String() = %q, want %q", floatNaN.String(), "NaN")
	}

	doubleType := mustBuiltinSimpleType(t, TypeNameDouble)
	doubleValue := NewDoubleValue(NewParsedValue("1.5", 1.5), doubleType)
	if doubleValue.String() != "1.5E0" {
		t.Fatalf("double String() = %q, want %q", doubleValue.String(), "1.5E0")
	}
	doubleInf := NewDoubleValue(NewParsedValue("INF", math.Inf(1)), doubleType)
	if doubleInf.String() != "INF" {
		t.Fatalf("double String() = %q, want %q", doubleInf.String(), "INF")
	}
	doubleNegInf := NewDoubleValue(NewParsedValue("-INF", math.Inf(-1)), doubleType)
	if doubleNegInf.String() != "-INF" {
		t.Fatalf("double String() = %q, want %q", doubleNegInf.String(), "-INF")
	}
	doubleNaN := NewDoubleValue(NewParsedValue("NaN", math.NaN()), doubleType)
	if doubleNaN.String() != "NaN" {
		t.Fatalf("double String() = %q, want %q", doubleNaN.String(), "NaN")
	}
}

func TestValueAs_WithComparableWrappers(t *testing.T) {
	// test ComparableBigRat - unwrap to *big.Rat
	rat := &big.Rat{}
	rat.SetString("1.5")
	typ := mustBuiltinSimpleType(t, TypeNameDecimal)
	val := NewDecimalValue(NewParsedValue("1.5", rat), typ)

	// test direct unwrap to *big.Rat
	result, err := ValueAs[*big.Rat](val)
	if err != nil {
		t.Errorf("ValueAs[*big.Rat]() error = %v", err)
	}
	if result.Cmp(rat) != 0 {
		t.Errorf("ValueAs[*big.Rat]() = %v, want %v", result, rat)
	}

	// test ComparableBigInt - unwrap to *big.Int
	bigInt := big.NewInt(123)
	typInt := mustBuiltinSimpleType(t, TypeNameInteger)
	valInt := NewIntegerValue(NewParsedValue("123", bigInt), typInt)

	resultInt, err := ValueAs[*big.Int](valInt)
	if err != nil {
		t.Errorf("ValueAs[*big.Int]() error = %v", err)
	}
	if resultInt.Cmp(bigInt) != 0 {
		t.Errorf("ValueAs[*big.Int]() = %v, want %v", resultInt, bigInt)
	}

	// test ComparableTime - unwrap to time.Time
	dt, err := ParseDateTime("2001-10-26T21:32:52")
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	typTime := mustBuiltinSimpleType(t, TypeNameDateTime)
	valTime := NewDateTimeValue(NewParsedValue("2001-10-26T21:32:52", dt), typTime)

	resultTime, err := ValueAs[time.Time](valTime)
	if err != nil {
		t.Errorf("ValueAs[time.Time]() error = %v", err)
	}
	if !resultTime.Equal(dt) {
		t.Errorf("ValueAs[time.Time]() = %v, want %v", resultTime, dt)
	}

	// test ComparableFloat64 - unwrap to float64
	typFloat := mustBuiltinSimpleType(t, TypeNameFloat)
	valFloat := NewFloatValue(NewParsedValue("123.456", float32(123.456)), typFloat)

	resultFloat, err := ValueAs[float32](valFloat)
	if err != nil {
		t.Errorf("ValueAs[float32]() error = %v", err)
	}
	if resultFloat != float32(123.456) {
		t.Errorf("ValueAs[float32]() = %v, want %v", resultFloat, float32(123.456))
	}
}

func TestValueAs_UnwrappableInterface(t *testing.T) {
	// test that Unwrappable interface works correctly
	rat := &big.Rat{}
	rat.SetString("1.5")
	cbr := ComparableBigRat{Value: rat}

	// test Unwrap method
	unwrapped := cbr.Unwrap()
	if unwrapped != rat {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, rat)
	}

	// test that all Comparable types implement Unwrappable
	var _ Unwrappable = ComparableBigRat{}
	var _ Unwrappable = ComparableBigInt{}
	var _ Unwrappable = ComparableTime{}
	var _ Unwrappable = ComparableFloat64{}
	var _ Unwrappable = ComparableFloat32{}
}

func TestNormalizerForType_BuiltinDispatch(t *testing.T) {
	dateTimeType := GetBuiltin(TypeNameDateTime)
	if dateTimeType == nil {
		t.Fatal("GetBuiltin(TypeNameDateTime) returned nil")
	}
	normalizer := normalizerForType(dateTimeType)
	if _, ok := normalizer.(dateTimeNormalizer); !ok {
		t.Errorf("normalizerForType(dateTime) = %T, want dateTimeNormalizer", normalizer)
	}

	stringType := GetBuiltin(TypeNameString)
	if stringType == nil {
		t.Fatal("GetBuiltin(TypeNameString) returned nil")
	}
	normalizer = normalizerForType(stringType)
	if _, ok := normalizer.(whiteSpaceNormalizer); !ok {
		t.Errorf("normalizerForType(string) = %T, want whiteSpaceNormalizer", normalizer)
	}
}

func TestParseQNameValue_DefaultNamespace(t *testing.T) {
	context := map[string]string{
		"":  "urn:default",
		"p": "urn:pref",
	}

	qname, err := ParseQNameValue("local", context)
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != "urn:default" || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want {urn:default}local", qname)
	}

	qname, err = ParseQNameValue("p:local", context)
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != "urn:pref" || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want {urn:pref}local", qname)
	}

	qname, err = ParseQNameValue("local", map[string]string{"p": "urn:pref"})
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != NamespaceEmpty || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want local in no namespace", qname)
	}
}
