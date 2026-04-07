package model

import (
	"math"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestCompareTypedValues(t *testing.T) {
	t.Run("numeric values compare across decimal and integer", func(t *testing.T) {
		left := mustParseBuiltinValue(t, TypeNameDecimal, "1.0")
		right := mustParseBuiltinValue(t, TypeNameInteger, "1")
		if !CompareTypedValues(left, right) {
			t.Fatal("expected decimal and integer values to compare equal")
		}
	})

	t.Run("temporal values compare in value space", func(t *testing.T) {
		left := mustParseBuiltinValue(t, TypeNameDateTime, "2000-01-01T12:00:00Z")
		right := mustParseBuiltinValue(t, TypeNameDateTime, "2000-01-01T07:00:00-05:00")
		if !CompareTypedValues(left, right) {
			t.Fatal("expected dateTime values with different timezones to compare equal")
		}
	})

	t.Run("temporal values of different kinds do not compare equal", func(t *testing.T) {
		left := mustParseBuiltinValue(t, TypeNameDateTime, "2000-01-01T12:00:00Z")
		right := mustParseBuiltinValue(t, TypeNameDate, "2000-01-01Z")
		if CompareTypedValues(left, right) {
			t.Fatal("expected different temporal kinds to compare unequal")
		}
	})

	t.Run("nan floats compare equal", func(t *testing.T) {
		floatType := GetBuiltin(TypeNameFloat).simpleWrapper
		doubleType := GetBuiltin(TypeNameDouble).simpleWrapper
		left := NewFloatValue(NewParsedValue("NaN", float32(math.NaN())), floatType)
		right := NewDoubleValue(NewParsedValue("NaN", math.NaN()), doubleType)
		if !CompareTypedValues(left, right) {
			t.Fatal("expected NaN float values to compare equal")
		}
	})

	t.Run("duration values compare across raw and comparable forms", func(t *testing.T) {
		durationType := GetBuiltin(TypeNameDuration).simpleWrapper
		raw := NewXSDDurationValue(NewParsedValue("P1D", mustParseDuration(t, "P1D")), durationType)
		comparable := &StringTypedValue{
			Value: "P1D",
			Typ:   durationType,
		}
		if CompareTypedValues(raw, comparable) {
			t.Fatal("expected string-backed duration not to compare equal")
		}
		withComparableNative := &testTypedValue{
			typ:     durationType,
			lexical: "P1D",
			native: ComparableXSDDuration{
				Typ:   durationType,
				Value: mustParseDuration(t, "P1D"),
			},
		}
		if !CompareTypedValues(raw, withComparableNative) {
			t.Fatal("expected raw and comparable duration values to compare equal")
		}
	})

	t.Run("binary values compare by bytes", func(t *testing.T) {
		left := mustParseBuiltinValue(t, TypeNameHexBinary, "0A0B")
		right := mustParseBuiltinValue(t, TypeNameBase64Binary, "Cgs=")
		if !CompareTypedValues(left, right) {
			t.Fatal("expected binary values with identical bytes to compare equal")
		}
	})
}

type testTypedValue struct {
	typ     Type
	lexical string
	native  any
}

func (v *testTypedValue) Type() Type      { return v.typ }
func (v *testTypedValue) Lexical() string { return v.lexical }
func (v *testTypedValue) Native() any     { return v.native }
func (v *testTypedValue) String() string  { return v.lexical }

func mustParseBuiltinValue(t *testing.T, typeName TypeName, lexical string) TypedValue {
	t.Helper()
	typed, err := GetBuiltin(typeName).ParseValue(lexical)
	if err != nil {
		t.Fatalf("ParseValue(%s, %q) error = %v", typeName, lexical, err)
	}
	return typed
}

func mustParseDuration(t *testing.T, lexical string) value.Duration {
	t.Helper()
	parsed, err := value.ParseDuration(lexical)
	if err != nil {
		t.Fatalf("ParseDuration(%q) error = %v", lexical, err)
	}
	return parsed
}
