package types

import (
	"math/big"
	"testing"
	"time"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
)

func TestTypedValue_Decimal(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "123.456"
	native, err := lexicalparser.ParseDecimal(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[*big.Rat](value)
	if err != nil {
		t.Errorf("ValueAs[*big.Rat]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[*big.Rat]() = %v, want %v", extracted, native)
	}

	// Test type mismatch
	_, err = ValueAs[bool](value)
	if err == nil {
		t.Error("ValueAs[bool]() should return error for type mismatch")
	}
}

func TestTypedValue_Boolean(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "boolean",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "true"
	native, err := lexicalparser.ParseBoolean(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[bool](value)
	if err != nil {
		t.Errorf("ValueAs[bool]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[bool]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_DateTime(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "dateTime",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "2001-10-26T21:32:52"
	native, err := lexicalparser.ParseDateTime(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[time.Time](value)
	if err != nil {
		t.Errorf("ValueAs[time.Time]() error = %v", err)
	}
	if !extracted.Equal(native) {
		t.Errorf("ValueAs[time.Time]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_Integer(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "12345678901234567890"
	native, err := lexicalparser.ParseInteger(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[*big.Int](value)
	if err != nil {
		t.Errorf("ValueAs[*big.Int]() error = %v", err)
	}
	if extracted.Cmp(native) != 0 {
		t.Errorf("ValueAs[*big.Int]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_Float(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "float",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "123.456"
	native, err := lexicalparser.ParseFloat(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[float32](value)
	if err != nil {
		t.Errorf("ValueAs[float32]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[float32]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_String(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "string",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)

	lexical := "hello world"
	native, err := lexicalparser.ParseString(lexical)
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

	// Test type-safe extraction
	extracted, err := ValueAs[string](value)
	if err != nil {
		t.Errorf("ValueAs[string]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[string]() = %v, want %v", extracted, native)
	}
}

func TestValueAs_WithComparableWrappers(t *testing.T) {
	// Test ComparableBigRat - unwrap to *big.Rat
	rat := &big.Rat{}
	rat.SetString("1.5")
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	typ.MarkBuiltin()
	typ.SetVariety(AtomicVariety)
	val := NewDecimalValue(NewParsedValue("1.5", rat), typ)

	// Test direct unwrap to *big.Rat
	result, err := ValueAs[*big.Rat](val)
	if err != nil {
		t.Errorf("ValueAs[*big.Rat]() error = %v", err)
	}
	if result.Cmp(rat) != 0 {
		t.Errorf("ValueAs[*big.Rat]() = %v, want %v", result, rat)
	}

	// Test ComparableBigInt - unwrap to *big.Int
	bigInt := big.NewInt(123)
	typInt := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// Variety set via SetVariety
	}
	typInt.MarkBuiltin()
	typInt.SetVariety(AtomicVariety)
	valInt := NewIntegerValue(NewParsedValue("123", bigInt), typInt)

	resultInt, err := ValueAs[*big.Int](valInt)
	if err != nil {
		t.Errorf("ValueAs[*big.Int]() error = %v", err)
	}
	if resultInt.Cmp(bigInt) != 0 {
		t.Errorf("ValueAs[*big.Int]() = %v, want %v", resultInt, bigInt)
	}

	// Test ComparableTime - unwrap to time.Time
	dt, err := lexicalparser.ParseDateTime("2001-10-26T21:32:52")
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	typTime := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "dateTime",
		},
		// Variety set via SetVariety
	}
	typTime.MarkBuiltin()
	typTime.SetVariety(AtomicVariety)
	valTime := NewDateTimeValue(NewParsedValue("2001-10-26T21:32:52", dt), typTime)

	resultTime, err := ValueAs[time.Time](valTime)
	if err != nil {
		t.Errorf("ValueAs[time.Time]() error = %v", err)
	}
	if !resultTime.Equal(dt) {
		t.Errorf("ValueAs[time.Time]() = %v, want %v", resultTime, dt)
	}

	// Test ComparableFloat64 - unwrap to float64
	typFloat := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "float",
		},
		// Variety set via SetVariety
	}
	typFloat.MarkBuiltin()
	typFloat.SetVariety(AtomicVariety)
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
	// Test that Unwrappable interface works correctly
	rat := &big.Rat{}
	rat.SetString("1.5")
	cbr := ComparableBigRat{Value: rat}

	// Test Unwrap method
	unwrapped := cbr.Unwrap()
	if unwrapped != rat {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, rat)
	}

	// Test that all Comparable types implement Unwrappable
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
