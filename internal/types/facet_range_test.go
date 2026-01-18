package types

import (
	"testing"
	"time"
)

func TestGenericMinInclusive_BigRat(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	minVal, _ := ParseDecimal("100")
	compMin := ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testVal, _ := ParseDecimal("150")
	typedValue := NewDecimalValue(NewParsedValue("150", testVal), decimalType)

	// should pass (150 >= 100)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (50 < 100)
	failVal, _ := ParseDecimal("50")
	failTypedValue := NewDecimalValue(NewParsedValue("50", failVal), decimalType)
	if err := facet.Validate(failTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value < min")
	}
}

func TestGenericMaxInclusive_BigRat(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	maxVal, _ := ParseDecimal("100")
	compMax := ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	testVal, _ := ParseDecimal("50")
	typedValue := NewDecimalValue(NewParsedValue("50", testVal), decimalType)

	// should pass (50 <= 100)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (150 > 100)
	failVal, _ := ParseDecimal("150")
	failTypedValue := NewDecimalValue(NewParsedValue("150", failVal), decimalType)
	if err := facet.Validate(failTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value > max")
	}
}

func TestGenericMinInclusive_Time(t *testing.T) {
	dateTimeType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "dateTime"},
	}
	minTime, _ := ParseDateTime("2001-01-01T00:00:00")
	compMin := ComparableTime{Value: minTime, Typ: dateTimeType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "2001-01-01T00:00:00",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testTime, _ := ParseDateTime("2001-06-01T00:00:00")
	typedValue := NewDateTimeValue(NewParsedValue("2001-06-01T00:00:00", testTime), dateTimeType)

	// should pass (testTime >= minTime)
	if err := facet.Validate(typedValue, dateTimeType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (beforeTime < minTime)
	beforeTime, _ := ParseDateTime("2000-01-01T00:00:00")
	failTypedValue := NewDateTimeValue(NewParsedValue("2000-01-01T00:00:00", beforeTime), dateTimeType)
	if err := facet.Validate(failTypedValue, dateTimeType); err == nil {
		t.Error("Validate() should return error for value before min")
	}
}

func TestGenericMinInclusive_BigInt(t *testing.T) {
	integerType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
	}
	minVal, _ := ParseInteger("100")
	compMin := ComparableBigInt{Value: minVal, Typ: integerType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testVal, _ := ParseInteger("150")
	typedValue := NewIntegerValue(NewParsedValue("150", testVal), integerType)

	// should pass (150 >= 100)
	if err := facet.Validate(typedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestGenericMinExclusive(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	minVal, _ := ParseDecimal("100")
	compMin := ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minExclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}

	// should pass (150 > 100)
	testVal, _ := ParseDecimal("150")
	typedValue := NewDecimalValue(NewParsedValue("150", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (100 is not > 100)
	equalVal, _ := ParseDecimal("100")
	equalTypedValue := NewDecimalValue(NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == min (exclusive)")
	}
}

func TestGenericMaxExclusive(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	maxVal, _ := ParseDecimal("100")
	compMax := ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	// should pass (50 < 100)
	testVal, _ := ParseDecimal("50")
	typedValue := NewDecimalValue(NewParsedValue("50", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (100 is not < 100)
	equalVal, _ := ParseDecimal("100")
	equalTypedValue := NewDecimalValue(NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == max (exclusive)")
	}
}

func TestGenericFacet_TypeMismatch(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	boolType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "boolean"},
	}
	minVal, _ := ParseDecimal("100")
	compMin := ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// try to validate with wrong type (boolean instead of decimal)
	boolVal, _ := ParseBoolean("true")
	boolTypedValue := NewBooleanValue(NewParsedValue("true", boolVal), boolType)

	// should fail with type mismatch error
	if err := facet.Validate(boolTypedValue, boolType); err == nil {
		t.Error("Validate() should return error for type mismatch")
	}
}

// TestGenericFacet_StringTypedValue_Decimal tests facet validation with StringTypedValue
// This simulates the case where parseToTypedValue fails and falls back to string validation
func TestGenericFacet_StringTypedValue_Decimal(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	maxVal, _ := ParseDecimal("100")
	compMax := ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	// create StringTypedValue (simulating fallback when parseToTypedValue fails)
	// this is the scenario that causes the conversion error
	stringTypedValue := &StringTypedValue{
		Value: "50",
		Typ:   decimalType,
	}

	// should pass (50 < 100) - the string should be parsed to *big.Rat
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (string '50' should be parsed and compared)", err)
	}

	// should fail (150 > 100)
	failStringTypedValue := &StringTypedValue{
		Value: "150",
		Typ:   decimalType,
	}
	if err := facet.Validate(failStringTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value > max (string '150' should be parsed and compared)")
	}
}

// TestGenericFacet_StringTypedValue_Decimal_MinInclusive tests minInclusive with StringTypedValue
func TestGenericFacet_StringTypedValue_Decimal_MinInclusive(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	minVal, _ := ParseDecimal("100")
	compMin := ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// should pass (150 >= 100)
	stringTypedValue := &StringTypedValue{
		Value: "150",
		Typ:   decimalType,
	}
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (50 < 100)
	failStringTypedValue := &StringTypedValue{
		Value: "50",
		Typ:   decimalType,
	}
	if err := facet.Validate(failStringTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value < min")
	}
}

// TestGenericFacet_StringTypedValue_Decimal_MinExclusive tests minExclusive with StringTypedValue
func TestGenericFacet_StringTypedValue_Decimal_MinExclusive(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	minVal, _ := ParseDecimal("100")
	compMin := ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minExclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}

	// should pass (150 > 100)
	stringTypedValue := &StringTypedValue{
		Value: "150",
		Typ:   decimalType,
	}
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// should fail (100 is not > 100)
	failStringTypedValue := &StringTypedValue{
		Value: "100",
		Typ:   decimalType,
	}
	if err := facet.Validate(failStringTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == min (exclusive)")
	}
}

// TestGenericFacet_StringTypedValue_Integer tests facet validation with StringTypedValue for integer type
func TestGenericFacet_StringTypedValue_Integer(t *testing.T) {
	// create an integer type (primitive is decimal)
	integerType := mustBuiltinSimpleType(t, TypeNameInteger)

	// create facet with maxInclusive on integer (uses ComparableBigInt)
	maxVal, _ := ParseInteger("100")
	compMax := ComparableBigInt{Value: maxVal, Typ: integerType}
	facet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	stringTypedValue := &StringTypedValue{
		Value: "50",
		Typ:   integerType,
	}

	// should pass (50 <= 100) - the string should be parsed to *big.Int
	if err := facet.Validate(stringTypedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil (string '50' should be parsed and compared)", err)
	}
}

// TestGenericFacet_ValueSpaceComparison_Decimal tests that value space comparison works correctly
// 1.0 == 1.000 for decimal types (same value space, different lexical representations)
func TestGenericFacet_ValueSpaceComparison_Decimal(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}

	// create facet with value "1.0"
	facetVal, _ := ParseDecimal("1.0")
	compFacet := ComparableBigRat{Value: facetVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "1.0",
		value:   compFacet,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	// value "1.000" should pass (same value space as "1.0")
	testVal, _ := ParseDecimal("1.000")
	typedValue := NewDecimalValue(NewParsedValue("1.000", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (1.000 should equal 1.0 in value space)", err)
	}

	// value "1" should also pass (same value space)
	testVal2, _ := ParseDecimal("1")
	typedValue2 := NewDecimalValue(NewParsedValue("1", testVal2), decimalType)
	if err := facet.Validate(typedValue2, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (1 should equal 1.0 in value space)", err)
	}
}

// TestGenericFacet_Duration tests range facets on duration types (OrderedPartial)
func TestGenericFacet_Duration(t *testing.T) {
	durationType := mustBuiltinSimpleType(t, TypeNameDuration)

	// test minInclusive with duration
	minDur, err := ParseDurationToTimeDuration("P1D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	compMin := ComparableDuration{Value: minDur, Typ: durationType}
	minFacet := &RangeFacet{
		name:    "minInclusive",
		lexical: "P1D",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// should pass (P2D >= P1D)
	testDur, err := ParseDurationToTimeDuration("P2D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	testValue := &DurationTypedValue{
		Value: "P2D",
		Typ:   durationType,
		dur:   testDur,
	}
	err = minFacet.Validate(testValue, durationType)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil (P2D should be >= P1D)", err)
	}

	// should fail (PT12H < P1D, since 12 hours < 1 day)
	failDur, err := ParseDurationToTimeDuration("PT12H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	failValue := &DurationTypedValue{
		Value: "PT12H",
		Typ:   durationType,
		dur:   failDur,
	}
	err = minFacet.Validate(failValue, durationType)
	if err == nil {
		t.Error("Validate() should return error for PT12H < P1D")
	}

	// test maxInclusive with duration
	maxDur, err := ParseDurationToTimeDuration("P30D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	compMax := ComparableDuration{Value: maxDur, Typ: durationType}
	maxFacet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "P30D",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	// should pass (P7D <= P30D)
	testDur2, err := ParseDurationToTimeDuration("P7D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	testValue2 := &DurationTypedValue{
		Value: "P7D",
		Typ:   durationType,
		dur:   testDur2,
	}
	if err := maxFacet.Validate(testValue2, durationType); err != nil {
		t.Errorf("Validate() error = %v, want nil (P7D should be <= P30D)", err)
	}
}

// DurationTypedValue is a helper type for testing duration facets
type DurationTypedValue struct {
	Typ   Type
	Value string
	dur   time.Duration
}

func (d *DurationTypedValue) Type() Type {
	return d.Typ
}

func (d *DurationTypedValue) Lexical() string {
	return d.Value
}

func (d *DurationTypedValue) Native() any {
	return ComparableDuration{Value: d.dur}
}

func (d *DurationTypedValue) String() string {
	return d.Value
}

// TestRangeFacet_CrossTypeNumeric checks cross-type numeric comparisons.
// It covers decimal facet values against integer instance values.
func TestRangeFacet_CrossTypeNumeric(t *testing.T) {
	// scenario: maxExclusive facet on a decimal type with value "100", but instance value is integer
	// this simulates cases like Boeing IPO test where quantity field has maxExclusive on decimal
	// but the instance value is parsed as integer
	decimalType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	integerType := &SimpleType{
		QName: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
	}
	integerType.MarkBuiltin()
	integerType.SetVariety(AtomicVariety)
	integerType.SetPrimitiveType(decimalType)

	// create maxExclusive facet with decimal value (ComparableBigRat)
	maxVal, _ := ParseDecimal("100")
	compMax := ComparableBigRat{Value: maxVal, Typ: decimalType}
	maxFacet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	t.Run("MaxExclusive_decimal_100_on_integer_50_passes", func(t *testing.T) {
		intVal50, _ := ParseInteger("50")
		integerValue50 := NewIntegerValue(NewParsedValue("50", intVal50), integerType)
		if err := maxFacet.Validate(integerValue50, integerType); err != nil {
			t.Errorf("MaxExclusive(100) on integer(50) error = %v, want nil", err)
		}
	})

	t.Run("MaxExclusive_decimal_100_on_integer_100_fails", func(t *testing.T) {
		intVal100, _ := ParseInteger("100")
		integerValue100 := NewIntegerValue(NewParsedValue("100", intVal100), integerType)
		if err := maxFacet.Validate(integerValue100, integerType); err == nil {
			t.Error("MaxExclusive(100) on integer(100) should return error")
		}
	})

	t.Run("MaxExclusive_decimal_100_on_integer_150_fails", func(t *testing.T) {
		intVal150, _ := ParseInteger("150")
		integerValue150 := NewIntegerValue(NewParsedValue("150", intVal150), integerType)
		if err := maxFacet.Validate(integerValue150, integerType); err == nil {
			t.Error("MaxExclusive(100) on integer(150) should return error")
		}
	})

	// create minInclusive facet with integer value (ComparableBigInt)
	// this tests the reverse: facet has integer value, instance has decimal value
	minVal, _ := ParseInteger("100")
	compMin := ComparableBigInt{Value: minVal, Typ: integerType}
	minFacet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	t.Run("MinInclusive_integer_100_on_decimal_150.0_passes", func(t *testing.T) {
		decimalVal150, _ := ParseDecimal("150.0")
		decimalValue150 := NewDecimalValue(NewParsedValue("150.0", decimalVal150), decimalType)
		if err := minFacet.Validate(decimalValue150, decimalType); err != nil {
			t.Errorf("MinInclusive(100) on decimal(150.0) error = %v, want nil", err)
		}
	})

	t.Run("MinInclusive_integer_100_on_decimal_100.0_passes", func(t *testing.T) {
		decimalVal100, _ := ParseDecimal("100.0")
		decimalValue100 := NewDecimalValue(NewParsedValue("100.0", decimalVal100), decimalType)
		if err := minFacet.Validate(decimalValue100, decimalType); err != nil {
			t.Errorf("MinInclusive(100) on decimal(100.0) error = %v, want nil", err)
		}
	})

	t.Run("MinInclusive_integer_100_on_decimal_50.0_fails", func(t *testing.T) {
		decimalVal50, _ := ParseDecimal("50.0")
		decimalValue50 := NewDecimalValue(NewParsedValue("50.0", decimalVal50), decimalType)
		if err := minFacet.Validate(decimalValue50, decimalType); err == nil {
			t.Error("MinInclusive(100) on decimal(50.0) should return error")
		}
	})
}
