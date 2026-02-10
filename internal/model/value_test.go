package model

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	qnamelex "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/value"
)

func TestTypedValue_Decimal(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameDecimal)

	lexical := "123.456"
	native, err := ParseDecimal(lexical)
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}

	typedValue := NewDecimalValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	decNative, ok := typedValue.Native().(num.Dec)
	if !ok {
		t.Fatalf("Native() type = %T, want num.Dec", typedValue.Native())
	}
	if decNative.Compare(native) != 0 {
		t.Errorf("Native() = %v, want %v", decNative, native)
	}
	if typedValue.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[num.Dec](typedValue)
	if err != nil {
		t.Errorf("ValueAs[num.Dec]() error = %v", err)
	}
	if extracted.Compare(native) != 0 {
		t.Errorf("ValueAs[num.Dec]() = %v, want %v", extracted, native)
	}

	// test type mismatch
	_, err = ValueAs[bool](typedValue)
	if err == nil {
		t.Error("ValueAs[bool]() should return error for type mismatch")
	}
}

func TestValueAs_NilValue(t *testing.T) {
	_, err := ValueAs[string](TypedValue(nil))
	if err == nil {
		t.Fatal("ValueAs should return error for nil value")
	}
}

func TestTypedValue_Boolean(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameBoolean)

	lexical := "true"
	native, err := ParseBoolean(lexical)
	if err != nil {
		t.Fatalf("ParseBoolean() error = %v", err)
	}

	typedValue := NewBooleanValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	if typedValue.Native() != native {
		t.Errorf("Native() = %v, want %v", typedValue.Native(), native)
	}
	if typedValue.String() != "true" {
		t.Errorf("String() = %v, want 'true'", typedValue.String())
	}

	// test type-safe extraction
	extracted, err := ValueAs[bool](typedValue)
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

	typedValue := NewDateTimeValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	if typedValue.Native() != native {
		t.Errorf("Native() = %v, want %v", typedValue.Native(), native)
	}
	if typedValue.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[time.Time](typedValue)
	if err != nil {
		t.Errorf("ValueAs[time.Time]() error = %v", err)
	}
	if !extracted.Equal(native) {
		t.Errorf("ValueAs[time.Time]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_DateTimeCanonicalString(t *testing.T) {
	tests := []struct {
		name     string
		typeName TypeName
		lexical  string
		want     string
	}{
		{"dateTime no tz", TypeNameDateTime, "2001-10-26T21:32:52", "2001-10-26T21:32:52"},
		{"dateTime with tz", TypeNameDateTime, "2001-10-26T21:32:52Z", "2001-10-26T21:32:52Z"},
		{"date no tz", TypeNameDate, "2001-10-26", "2001-10-26"},
		{"date with tz", TypeNameDate, "2001-10-26Z", "2001-10-26Z"},
		{"time no tz", TypeNameTime, "21:32:52", "21:32:52"},
		{"time with tz", TypeNameTime, "21:32:52+05:30", "16:02:52Z"},
		{"gYear no tz", TypeNameGYear, "2001", "2001"},
		{"gYear with tz", TypeNameGYear, "2001Z", "2001Z"},
		{"gYearMonth no tz", TypeNameGYearMonth, "2001-10", "2001-10"},
		{"gYearMonth with tz", TypeNameGYearMonth, "2001-10Z", "2001-10Z"},
		{"gMonth no tz", TypeNameGMonth, "--10", "--10"},
		{"gMonth with tz", TypeNameGMonth, "--10Z", "--10Z"},
		{"gMonthDay no tz", TypeNameGMonthDay, "--10-26", "--10-26"},
		{"gMonthDay with tz", TypeNameGMonthDay, "--10-26Z", "--10-26Z"},
		{"gDay no tz", TypeNameGDay, "---26", "---26"},
		{"gDay with tz", TypeNameGDay, "---26Z", "---26Z"},
		{"dateTime fractional", TypeNameDateTime, "2001-10-26T21:32:52.1200Z", "2001-10-26T21:32:52.12Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := mustBuiltinSimpleType(t, tt.typeName)
			native, err := parseTemporalForType(tt.typeName, tt.lexical)
			if err != nil {
				t.Fatalf("parseTemporalForType() error = %v", err)
			}
			typedValue := NewDateTimeValue(NewParsedValue(tt.lexical, native), typ)
			if got := typedValue.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypedValue_Integer(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameInteger)

	lexical := "00012345678901234567890"
	native, err := ParseInteger(lexical)
	if err != nil {
		t.Fatalf("ParseInteger() error = %v", err)
	}

	typedValue := NewIntegerValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	intNative, ok := typedValue.Native().(num.Int)
	if !ok {
		t.Fatalf("Native() type = %T, want num.Int", typedValue.Native())
	}
	if intNative.Compare(native) != 0 {
		t.Errorf("Native() = %v, want %v", intNative, native)
	}
	var buf []byte
	buf = native.RenderCanonical(buf)
	want := string(buf)
	if got := typedValue.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	// test type-safe extraction
	extracted, err := ValueAs[num.Int](typedValue)
	if err != nil {
		t.Errorf("ValueAs[num.Int]() error = %v", err)
	}
	if extracted.Compare(native) != 0 {
		t.Errorf("ValueAs[num.Int]() = %v, want %v", extracted, native)
	}
}

func TestTypedValue_Float(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameFloat)

	lexical := "123.456"
	native, err := ParseFloat(lexical)
	if err != nil {
		t.Fatalf("ParseFloat() error = %v", err)
	}

	typedValue := NewFloatValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	if typedValue.Native() != native {
		t.Errorf("Native() = %v, want %v", typedValue.Native(), native)
	}
	if typedValue.String() == "" {
		t.Error("String() should not be empty")
	}

	// test type-safe extraction
	extracted, err := ValueAs[float32](typedValue)
	if err != nil {
		t.Errorf("ValueAs[float32]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[float32]() = %v, want %v", extracted, native)
	}
}

func TestFloatCanonicalizationConsistency(t *testing.T) {
	floatType := mustBuiltinSimpleType(t, TypeNameFloat)
	doubleType := mustBuiltinSimpleType(t, TypeNameDouble)

	floatCases := []float32{
		0,
		float32(math.Copysign(0, -1)),
		1.5,
		1e-5,
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
		float32(math.NaN()),
	}
	for _, v := range floatCases {
		tv := NewFloatValue(NewParsedValue("x", v), floatType)
		got := tv.String()
		want := value.CanonicalFloat(float64(v), 32)
		if got != want {
			t.Fatalf("float canonical = %q, want %q", got, want)
		}
	}

	doubleCases := []float64{
		0,
		math.Copysign(0, -1),
		1.5,
		1e-5,
		math.Inf(1),
		math.Inf(-1),
		math.NaN(),
	}
	for _, v := range doubleCases {
		tv := NewDoubleValue(NewParsedValue("x", v), doubleType)
		got := tv.String()
		want := value.CanonicalFloat(v, 64)
		if got != want {
			t.Fatalf("double canonical = %q, want %q", got, want)
		}
	}
}

func TestTypedValue_String(t *testing.T) {
	typ := mustBuiltinSimpleType(t, TypeNameString)

	lexical := "hello world"
	native, err := ParseString(lexical)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	typedValue := NewStringValue(NewParsedValue(lexical, native), typ)

	if typedValue.Type() != typ {
		t.Errorf("Type() = %v, want %v", typedValue.Type(), typ)
	}
	if typedValue.Lexical() != lexical {
		t.Errorf("Lexical() = %v, want %v", typedValue.Lexical(), lexical)
	}
	if typedValue.Native() != native {
		t.Errorf("Native() = %v, want %v", typedValue.Native(), native)
	}
	if typedValue.String() != lexical {
		t.Errorf("String() = %v, want %v", typedValue.String(), lexical)
	}

	// test type-safe extraction
	extracted, err := ValueAs[string](typedValue)
	if err != nil {
		t.Errorf("ValueAs[string]() error = %v", err)
	}
	if extracted != native {
		t.Errorf("ValueAs[string]() = %v, want %v", extracted, native)
	}
}

func TestParseDecimalRejectsFraction(t *testing.T) {
	for _, lexical := range []string{"1/2", "3/7"} {
		if _, err := ParseDecimal(lexical); err == nil {
			t.Fatalf("expected decimal parse error for %q", lexical)
		}
	}
}

func TestParseBase64BinaryRejectsURLSafe(t *testing.T) {
	if _, err := ParseBase64Binary("AA-_"); err == nil {
		t.Fatalf("expected base64Binary parse error for URL-safe alphabet")
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

func TestDecimalCanonicalizationTable(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)
	cases := []struct {
		lexical string
		want    string
	}{
		{lexical: "0", want: "0.0"},
		{lexical: "-0.0", want: "0.0"},
		{lexical: "+001.500", want: "1.5"},
		{lexical: "000.0100", want: "0.01"},
		{lexical: "-001.2300", want: "-1.23"},
	}

	for _, tc := range cases {
		dec, err := ParseDecimal(tc.lexical)
		if err != nil {
			t.Fatalf("ParseDecimal(%q) error = %v", tc.lexical, err)
		}
		typedValue := NewDecimalValue(NewParsedValue(tc.lexical, dec), decimalType)
		if got := typedValue.String(); got != tc.want {
			t.Fatalf("String(%q) = %q, want %q", tc.lexical, got, tc.want)
		}
	}
}

func TestValueAs_WithComparableWrappers(t *testing.T) {
	// test ComparableDec - unwrap to num.Dec
	dec, _ := ParseDecimal("1.5")
	typ := mustBuiltinSimpleType(t, TypeNameDecimal)
	val := NewDecimalValue(NewParsedValue("1.5", dec), typ)

	// test direct unwrap to num.Dec
	result, err := ValueAs[num.Dec](val)
	if err != nil {
		t.Errorf("ValueAs[num.Dec]() error = %v", err)
	}
	if result.Compare(dec) != 0 {
		t.Errorf("ValueAs[num.Dec]() = %v, want %v", result, dec)
	}

	// test ComparableInt - unwrap to num.Int
	intVal, _ := ParseInteger("123")
	typInt := mustBuiltinSimpleType(t, TypeNameInteger)
	valInt := NewIntegerValue(NewParsedValue("123", intVal), typInt)

	resultInt, err := ValueAs[num.Int](valInt)
	if err != nil {
		t.Errorf("ValueAs[num.Int]() error = %v", err)
	}
	if resultInt.Compare(intVal) != 0 {
		t.Errorf("ValueAs[num.Int]() = %v, want %v", resultInt, intVal)
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

func parseTemporalForType(typeName TypeName, lexical string) (time.Time, error) {
	switch typeName {
	case TypeNameDateTime:
		return ParseDateTime(lexical)
	case TypeNameDate:
		return value.ParseDate([]byte(lexical))
	case TypeNameTime:
		return value.ParseTime([]byte(lexical))
	case TypeNameGYear:
		return value.ParseGYear([]byte(lexical))
	case TypeNameGYearMonth:
		return value.ParseGYearMonth([]byte(lexical))
	case TypeNameGMonthDay:
		return value.ParseGMonthDay([]byte(lexical))
	case TypeNameGMonth:
		return value.ParseGMonth([]byte(lexical))
	case TypeNameGDay:
		return value.ParseGDay([]byte(lexical))
	default:
		return time.Time{}, fmt.Errorf("unsupported temporal type %s", typeName)
	}
}

func TestValueAs_UnwrappableInterface(t *testing.T) {
	// test that Unwrappable interface works correctly
	dec, _ := ParseDecimal("1.5")
	cbr := ComparableDec{Value: dec}

	// test Unwrap method
	unwrapped, ok := cbr.Unwrap().(num.Dec)
	if !ok {
		t.Fatalf("Unwrap() should return num.Dec")
	}
	if unwrapped.Compare(dec) != 0 {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, dec)
	}

	// test that all Comparable types implement Unwrappable
	var _ Unwrappable = ComparableDec{}
	var _ Unwrappable = ComparableInt{}
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

	qname, err := qnamelex.ParseQNameValue("local", context)
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != "urn:default" || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want {urn:default}local", qname)
	}

	qname, err = qnamelex.ParseQNameValue("p:local", context)
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != "urn:pref" || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want {urn:pref}local", qname)
	}

	qname, err = qnamelex.ParseQNameValue("local", map[string]string{"p": "urn:pref"})
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if qname.Namespace != NamespaceEmpty || qname.Local != "local" {
		t.Fatalf("ParseQNameValue() = %v, want local in no namespace", qname)
	}
}

func TestParseQNameValue_XMLPrefixBinding(t *testing.T) {
	qname, err := qnamelex.ParseQNameValue("xml:lang", nil)
	if err != nil {
		t.Fatalf("ParseQNameValue(xml:lang) error = %v", err)
	}
	if qname.Namespace != XMLNamespace || qname.Local != "lang" {
		t.Fatalf("ParseQNameValue(xml:lang) = %v, want {%s}lang", qname, XMLNamespace)
	}

	qname, err = qnamelex.ParseQNameValue("xml:lang", map[string]string{"xml": XMLNamespace})
	if err != nil {
		t.Fatalf("ParseQNameValue(xml:lang) error = %v", err)
	}
	if qname.Namespace != XMLNamespace || qname.Local != "lang" {
		t.Fatalf("ParseQNameValue(xml:lang) = %v, want {%s}lang", qname, XMLNamespace)
	}

	if _, err := qnamelex.ParseQNameValue("xml:lang", map[string]string{"xml": "urn:wrong"}); err == nil {
		t.Fatalf("expected ParseQNameValue to reject wrong xml prefix binding")
	}
}
