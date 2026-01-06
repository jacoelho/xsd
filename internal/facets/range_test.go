package facets

import (
	"testing"
	"time"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
	"github.com/jacoelho/xsd/internal/types"
)

func TestGenericMinInclusive_BigRat(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	minVal, _ := lexicalparser.ParseDecimal("100")
	compMin := types.ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testVal, _ := lexicalparser.ParseDecimal("150")
	typedValue := types.NewDecimalValue(types.NewParsedValue("150", testVal), decimalType)

	// Should pass (150 >= 100)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (50 < 100)
	failVal, _ := lexicalparser.ParseDecimal("50")
	failTypedValue := types.NewDecimalValue(types.NewParsedValue("50", failVal), decimalType)
	if err := facet.Validate(failTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value < min")
	}
}

func TestGenericMaxInclusive_BigRat(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	maxVal, _ := lexicalparser.ParseDecimal("100")
	compMax := types.ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	testVal, _ := lexicalparser.ParseDecimal("50")
	typedValue := types.NewDecimalValue(types.NewParsedValue("50", testVal), decimalType)

	// Should pass (50 <= 100)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (150 > 100)
	failVal, _ := lexicalparser.ParseDecimal("150")
	failTypedValue := types.NewDecimalValue(types.NewParsedValue("150", failVal), decimalType)
	if err := facet.Validate(failTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value > max")
	}
}

func TestGenericMinInclusive_Time(t *testing.T) {
	dateTimeType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "dateTime"},
	}
	minTime, _ := lexicalparser.ParseDateTime("2001-01-01T00:00:00")
	compMin := types.ComparableTime{Value: minTime, Typ: dateTimeType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "2001-01-01T00:00:00",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testTime, _ := lexicalparser.ParseDateTime("2001-06-01T00:00:00")
	typedValue := types.NewDateTimeValue(types.NewParsedValue("2001-06-01T00:00:00", testTime), dateTimeType)

	// Should pass (testTime >= minTime)
	if err := facet.Validate(typedValue, dateTimeType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (beforeTime < minTime)
	beforeTime, _ := lexicalparser.ParseDateTime("2000-01-01T00:00:00")
	failTypedValue := types.NewDateTimeValue(types.NewParsedValue("2000-01-01T00:00:00", beforeTime), dateTimeType)
	if err := facet.Validate(failTypedValue, dateTimeType); err == nil {
		t.Error("Validate() should return error for value before min")
	}
}

func TestGenericMinInclusive_BigInt(t *testing.T) {
	integerType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
	}
	minVal, _ := lexicalparser.ParseInteger("100")
	compMin := types.ComparableBigInt{Value: minVal, Typ: integerType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	testVal, _ := lexicalparser.ParseInteger("150")
	typedValue := types.NewIntegerValue(types.NewParsedValue("150", testVal), integerType)

	// Should pass (150 >= 100)
	if err := facet.Validate(typedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestGenericMinExclusive(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	minVal, _ := lexicalparser.ParseDecimal("100")
	compMin := types.ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minExclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}

	// Should pass (150 > 100)
	testVal, _ := lexicalparser.ParseDecimal("150")
	typedValue := types.NewDecimalValue(types.NewParsedValue("150", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (100 is not > 100)
	equalVal, _ := lexicalparser.ParseDecimal("100")
	equalTypedValue := types.NewDecimalValue(types.NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == min (exclusive)")
	}
}

func TestGenericMaxExclusive(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	maxVal, _ := lexicalparser.ParseDecimal("100")
	compMax := types.ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	// Should pass (50 < 100)
	testVal, _ := lexicalparser.ParseDecimal("50")
	typedValue := types.NewDecimalValue(types.NewParsedValue("50", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (100 is not < 100)
	equalVal, _ := lexicalparser.ParseDecimal("100")
	equalTypedValue := types.NewDecimalValue(types.NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == max (exclusive)")
	}
}

func TestGenericFacet_TypeMismatch(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	boolType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "boolean"},
	}
	minVal, _ := lexicalparser.ParseDecimal("100")
	compMin := types.ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// Try to validate with wrong type (boolean instead of decimal)
	boolVal, _ := lexicalparser.ParseBoolean("true")
	boolTypedValue := types.NewBooleanValue(types.NewParsedValue("true", boolVal), boolType)

	// Should fail with type mismatch error
	if err := facet.Validate(boolTypedValue, boolType); err == nil {
		t.Error("Validate() should return error for type mismatch")
	}
}

// TestGenericFacet_StringTypedValue_Decimal tests facet validation with StringTypedValue
// This simulates the case where parseToTypedValue fails and falls back to string validation
func TestGenericFacet_StringTypedValue_Decimal(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	maxVal, _ := lexicalparser.ParseDecimal("100")
	compMax := types.ComparableBigRat{Value: maxVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	// Create StringTypedValue (simulating fallback when parseToTypedValue fails)
	// This is the scenario that causes the conversion error
	stringTypedValue := &StringTypedValue{
		Value: "50",
		Typ:   decimalType,
	}

	// Should pass (50 < 100) - the string should be parsed to *big.Rat
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (string '50' should be parsed and compared)", err)
	}

	// Should fail (150 > 100)
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
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	minVal, _ := lexicalparser.ParseDecimal("100")
	compMin := types.ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// Should pass (150 >= 100)
	stringTypedValue := &StringTypedValue{
		Value: "150",
		Typ:   decimalType,
	}
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (50 < 100)
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
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	minVal, _ := lexicalparser.ParseDecimal("100")
	compMin := types.ComparableBigRat{Value: minVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "minExclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}

	// Should pass (150 > 100)
	stringTypedValue := &StringTypedValue{
		Value: "150",
		Typ:   decimalType,
	}
	if err := facet.Validate(stringTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Should fail (100 is not > 100)
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
	// Create an integer type (primitive is decimal)
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	integerType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
	}
	integerType.MarkBuiltin()
	integerType.SetVariety(types.AtomicVariety)
	integerType.SetPrimitiveType(decimalType)

	// Create facet with maxInclusive on integer (uses ComparableBigInt)
	maxVal, _ := lexicalparser.ParseInteger("100")
	compMax := types.ComparableBigInt{Value: maxVal, Typ: integerType}
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

	// Should pass (50 <= 100) - the string should be parsed to *big.Int
	if err := facet.Validate(stringTypedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil (string '50' should be parsed and compared)", err)
	}
}

// TestGenericFacet_ValueSpaceComparison_Decimal tests that value space comparison works correctly
// 1.0 == 1.000 for decimal types (same value space, different lexical representations)
func TestGenericFacet_ValueSpaceComparison_Decimal(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}

	// Create facet with value "1.0"
	facetVal, _ := lexicalparser.ParseDecimal("1.0")
	compFacet := types.ComparableBigRat{Value: facetVal, Typ: decimalType}
	facet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "1.0",
		value:   compFacet,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	// Value "1.000" should pass (same value space as "1.0")
	testVal, _ := lexicalparser.ParseDecimal("1.000")
	typedValue := types.NewDecimalValue(types.NewParsedValue("1.000", testVal), decimalType)
	if err := facet.Validate(typedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (1.000 should equal 1.0 in value space)", err)
	}

	// Value "1" should also pass (same value space)
	testVal2, _ := lexicalparser.ParseDecimal("1")
	typedValue2 := types.NewDecimalValue(types.NewParsedValue("1", testVal2), decimalType)
	if err := facet.Validate(typedValue2, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil (1 should equal 1.0 in value space)", err)
	}
}

// TestGenericFacet_Duration tests range facets on duration types (OrderedPartial)
func TestGenericFacet_Duration(t *testing.T) {
	durationType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "duration"},
	}
	durationType.MarkBuiltin()
	durationType.SetVariety(types.AtomicVariety)
	durationType.SetPrimitiveType(durationType)
	durationType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameDuration))

	// Test minInclusive with duration
	minDur, err := types.ParseDurationToTimeDuration("P1D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	compMin := types.ComparableDuration{Value: minDur, Typ: durationType}
	minFacet := &RangeFacet{
		name:    "minInclusive",
		lexical: "P1D",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	// Should pass (P2D >= P1D)
	testDur, err := types.ParseDurationToTimeDuration("P2D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	testValue := &DurationTypedValue{
		Value: "P2D",
		Typ:   durationType,
		dur:   testDur,
	}
	if err := minFacet.Validate(testValue, durationType); err != nil {
		t.Errorf("Validate() error = %v, want nil (P2D should be >= P1D)", err)
	}

	// Should fail (PT12H < P1D, since 12 hours < 1 day)
	failDur, err := types.ParseDurationToTimeDuration("PT12H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	failValue := &DurationTypedValue{
		Value: "PT12H",
		Typ:   durationType,
		dur:   failDur,
	}
	if err := minFacet.Validate(failValue, durationType); err == nil {
		t.Error("Validate() should return error for PT12H < P1D")
	}

	// Test maxInclusive with duration
	maxDur, err := types.ParseDurationToTimeDuration("P30D")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	compMax := types.ComparableDuration{Value: maxDur, Typ: durationType}
	maxFacet := &RangeFacet{
		name:    "maxInclusive",
		lexical: "P30D",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}

	// Should pass (P7D <= P30D)
	testDur2, err := types.ParseDurationToTimeDuration("P7D")
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
	Value string
	Typ   types.Type
	dur   time.Duration
}

func (d *DurationTypedValue) Type() types.Type {
	return d.Typ
}

func (d *DurationTypedValue) Lexical() string {
	return d.Value
}

func (d *DurationTypedValue) Native() any {
	return types.ComparableDuration{Value: d.dur}
}

func (d *DurationTypedValue) String() string {
	return d.Value
}

// TestRangeFacet_CrossTypeNumeric tests range facet validation with cross-type numeric comparison
// This tests the scenario where a facet has a decimal (ComparableBigRat) value but the instance
// value is an integer (ComparableBigInt), or vice versa. Since integers are a subset of decimals
// in XSD's value space, these should be comparable.
func TestRangeFacet_CrossTypeNumeric(t *testing.T) {
	// Scenario: maxExclusive facet on a decimal type with value "100", but instance value is integer
	// This simulates cases like Boeing IPO test where quantity field has maxExclusive on decimal
	// but the instance value is parsed as integer
	decimalType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "decimal"},
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)

	integerType := &types.SimpleType{
		QName: types.QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "integer"},
	}
	integerType.MarkBuiltin()
	integerType.SetVariety(types.AtomicVariety)
	integerType.SetPrimitiveType(decimalType)

	// Create maxExclusive facet with decimal value (ComparableBigRat)
	maxVal, _ := lexicalparser.ParseDecimal("100")
	compMax := types.ComparableBigRat{Value: maxVal, Typ: decimalType}
	maxFacet := &RangeFacet{
		name:    "maxExclusive",
		lexical: "100",
		value:   compMax,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}

	t.Run("MaxExclusive_decimal_100_on_integer_50_passes", func(t *testing.T) {
		intVal50, _ := lexicalparser.ParseInteger("50")
		integerValue50 := types.NewIntegerValue(types.NewParsedValue("50", intVal50), integerType)
		if err := maxFacet.Validate(integerValue50, integerType); err != nil {
			t.Errorf("MaxExclusive(100) on integer(50) error = %v, want nil", err)
		}
	})

	t.Run("MaxExclusive_decimal_100_on_integer_100_fails", func(t *testing.T) {
		intVal100, _ := lexicalparser.ParseInteger("100")
		integerValue100 := types.NewIntegerValue(types.NewParsedValue("100", intVal100), integerType)
		if err := maxFacet.Validate(integerValue100, integerType); err == nil {
			t.Error("MaxExclusive(100) on integer(100) should return error")
		}
	})

	t.Run("MaxExclusive_decimal_100_on_integer_150_fails", func(t *testing.T) {
		intVal150, _ := lexicalparser.ParseInteger("150")
		integerValue150 := types.NewIntegerValue(types.NewParsedValue("150", intVal150), integerType)
		if err := maxFacet.Validate(integerValue150, integerType); err == nil {
			t.Error("MaxExclusive(100) on integer(150) should return error")
		}
	})

	// Create minInclusive facet with integer value (ComparableBigInt)
	// This tests the reverse: facet has integer value, instance has decimal value
	minVal, _ := lexicalparser.ParseInteger("100")
	compMin := types.ComparableBigInt{Value: minVal, Typ: integerType}
	minFacet := &RangeFacet{
		name:    "minInclusive",
		lexical: "100",
		value:   compMin,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}

	t.Run("MinInclusive_integer_100_on_decimal_150.0_passes", func(t *testing.T) {
		decimalVal150, _ := lexicalparser.ParseDecimal("150.0")
		decimalValue150 := types.NewDecimalValue(types.NewParsedValue("150.0", decimalVal150), decimalType)
		if err := minFacet.Validate(decimalValue150, decimalType); err != nil {
			t.Errorf("MinInclusive(100) on decimal(150.0) error = %v, want nil", err)
		}
	})

	t.Run("MinInclusive_integer_100_on_decimal_100.0_passes", func(t *testing.T) {
		decimalVal100, _ := lexicalparser.ParseDecimal("100.0")
		decimalValue100 := types.NewDecimalValue(types.NewParsedValue("100.0", decimalVal100), decimalType)
		if err := minFacet.Validate(decimalValue100, decimalType); err != nil {
			t.Errorf("MinInclusive(100) on decimal(100.0) error = %v, want nil", err)
		}
	})

	t.Run("MinInclusive_integer_100_on_decimal_50.0_fails", func(t *testing.T) {
		decimalVal50, _ := lexicalparser.ParseDecimal("50.0")
		decimalValue50 := types.NewDecimalValue(types.NewParsedValue("50.0", decimalVal50), decimalType)
		if err := minFacet.Validate(decimalValue50, decimalType); err == nil {
			t.Error("MinInclusive(100) on decimal(50.0) should return error")
		}
	})
}
