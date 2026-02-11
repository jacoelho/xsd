package model

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestNewMinInclusive_Decimal(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	facet, err := NewMinInclusive("100.5", decimalType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}

	// test validation
	testVal, _ := ParseDecimal("150.75")
	testTypedValue := NewDecimalValue(NewParsedValue("150.75", testVal), decimalType)
	if err := facet.Validate(testTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// test invalid value
	invalidVal, _ := ParseDecimal("50")
	invalidTypedValue := NewDecimalValue(NewParsedValue("50", invalidVal), decimalType)
	if err := facet.Validate(invalidTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value < min")
	}
}

func TestNewMaxInclusive_Integer(t *testing.T) {
	integerType := mustBuiltinSimpleType(t, TypeNameInteger)

	facet, err := NewMaxInclusive("100", integerType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMaxInclusive() returned nil")
	}

	// test validation
	testVal, _ := ParseInteger("50")
	testTypedValue := NewIntegerValue(NewParsedValue("50", testVal), integerType)
	if err := facet.Validate(testTypedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinInclusive_DateTime(t *testing.T) {
	dateTimeType := mustBuiltinSimpleType(t, TypeNameDateTime)

	facet, err := NewMinInclusive("2001-01-01T00:00:00", dateTimeType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	// test validation
	testVal, _ := ParseDateTime("2001-06-01T00:00:00")
	testTypedValue := NewDateTimeValue(NewParsedValue("2001-06-01T00:00:00", testVal), dateTimeType)
	if err := facet.Validate(testTypedValue, dateTimeType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinExclusive(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	facet, err := NewMinExclusive("100", decimalType)
	if err != nil {
		t.Fatalf("NewMinExclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMinExclusive() returned nil")
	}

	// test validation - value equal to min should fail (exclusive)
	equalVal, _ := ParseDecimal("100")
	equalTypedValue := NewDecimalValue(NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == min (exclusive)")
	}
}

func TestNewMaxExclusive(t *testing.T) {
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)

	facet, err := NewMaxExclusive("100", decimalType)
	if err != nil {
		t.Fatalf("NewMaxExclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMaxExclusive() returned nil")
	}

	// test validation - value equal to max should fail (exclusive)
	equalVal, _ := ParseDecimal("100")
	equalTypedValue := NewDecimalValue(NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == max (exclusive)")
	}
}

func TestNewMinInclusive_InvalidType(t *testing.T) {
	// create a string type (not comparable for range facets)
	stringType := mustBuiltinSimpleType(t, TypeNameString)

	// should return error for non-comparable type
	_, err := NewMinInclusive("test", stringType)
	if err == nil {
		t.Error("NewMinInclusive() should return error for non-comparable type")
	}
}

func TestNewMinInclusive_WithBuiltinBase(t *testing.T) {
	// test case from issue 003: SimpleType with BuiltinType as ResolvedBase
	// this tests that PrimitiveType() correctly handles BuiltinType as base
	intBuiltin := GetBuiltin(TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	// create a derived type that restricts int (BuiltinType)
	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin

	// create facet using constructor - should work now that PrimitiveType() handles BuiltinType
	facet, err := NewMinInclusive("5", derivedType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (this was failing before the fix)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_NestedDerivation(t *testing.T) {
	// test nested derivation: moreDerived -> derived -> int (builtin)
	// this matches the example in issue 003
	intBuiltin := GetBuiltin(TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	// first level: derived restricts int
	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin

	// second level: moreDerived restricts derived
	moreDerivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "moreDerived",
		},
		Restriction: &Restriction{
			Base: derivedType.QName,
		},
	}
	moreDerivedType.ResolvedBase = derivedType

	// create facet using constructor - should work with nested derivation
	facet, err := NewMinInclusive("10", moreDerivedType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (this was failing before the fix)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}
}

func TestNewMinInclusive_Date(t *testing.T) {
	dateBuiltin := GetBuiltin(TypeNameDate)
	if dateBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameDate) returned nil")
	}

	// create facet using constructor - should parse as date, not dateTime
	facet, err := NewMinInclusive("1999-05-31", dateBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse date value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}

	// test validation
	testVal, _ := value.ParseDate([]byte("2000-06-01"))
	dateType := mustBuiltinSimpleType(t, TypeNameDate)
	testTypedValue := NewDateTimeValue(NewParsedValue("2000-06-01", testVal), dateType)
	if err := facet.Validate(testTypedValue, dateBuiltin); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinInclusive_Time(t *testing.T) {
	timeBuiltin := GetBuiltin(TypeNameTime)
	if timeBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameTime) returned nil")
	}

	// create facet using constructor - should parse as time, not dateTime
	// note: ParseTime currently has format bugs, but this test verifies the correct parser is called
	facet, err := NewMinInclusive("13:20:00", timeBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse time value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_GYear(t *testing.T) {
	gYearBuiltin := GetBuiltin(TypeNameGYear)
	if gYearBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGYear) returned nil")
	}

	// create facet using constructor - should parse as gYear, not dateTime
	facet, err := NewMinInclusive("2000", gYearBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse gYear value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_GYearMonth(t *testing.T) {
	gYearMonthBuiltin := GetBuiltin(TypeNameGYearMonth)
	if gYearMonthBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGYearMonth) returned nil")
	}

	// create facet using constructor - should parse as gYearMonth, not dateTime
	facet, err := NewMinInclusive("2001-03", gYearMonthBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse gYearMonth value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_GMonthDay(t *testing.T) {
	gMonthDayBuiltin := GetBuiltin(TypeNameGMonthDay)
	if gMonthDayBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGMonthDay) returned nil")
	}

	// create facet using constructor - should parse as gMonthDay, not dateTime
	facet, err := NewMinInclusive("--03-15", gMonthDayBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse gMonthDay value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_GDay(t *testing.T) {
	gDayBuiltin := GetBuiltin(TypeNameGDay)
	if gDayBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGDay) returned nil")
	}

	// create facet using constructor - should parse as gDay, not dateTime
	facet, err := NewMinInclusive("---15", gDayBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse gDay value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_GMonth(t *testing.T) {
	gMonthBuiltin := GetBuiltin(TypeNameGMonth)
	if gMonthBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGMonth) returned nil")
	}

	// create facet using constructor - should parse as gMonth, not dateTime
	facet, err := NewMinInclusive("--03", gMonthBuiltin)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (should parse gMonth value correctly)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	if facet.Name() != "minInclusive" {
		t.Errorf("Name() = %v, want 'minInclusive'", facet.Name())
	}
}

func TestNewMinInclusive_NilBaseType(t *testing.T) {
	t.Parallel()

	_, err := NewMinInclusive("1", nil)
	if err == nil {
		t.Fatal("NewMinInclusive() expected error, got nil")
	}
	if !errors.Is(err, ErrCannotDeterminePrimitiveType) {
		t.Fatalf("error = %v, want errors.Is(..., ErrCannotDeterminePrimitiveType)", err)
	}
}

func TestNewMinInclusive_IntegerDerivedBuiltins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseType    TypeName
		facetValue  string
		testLexical string
	}{
		{
			name:        "nonNegativeInteger",
			baseType:    TypeNameNonNegativeInteger,
			facetValue:  "0",
			testLexical: "0",
		},
		{
			name:        "positiveInteger",
			baseType:    TypeNamePositiveInteger,
			facetValue:  "1",
			testLexical: "1",
		},
		{
			name:        "negativeInteger",
			baseType:    TypeNameNegativeInteger,
			facetValue:  "-100",
			testLexical: "-1",
		},
		{
			name:        "nonPositiveInteger",
			baseType:    TypeNameNonPositiveInteger,
			facetValue:  "-100",
			testLexical: "0",
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseType := mustBuiltinSimpleType(t, tt.baseType)
			facet, err := NewMinInclusive(tt.facetValue, baseType)
			if err != nil {
				t.Fatalf("NewMinInclusive(%q, %s) error = %v", tt.facetValue, tt.baseType, err)
			}

			typed, err := baseType.ParseValue(tt.testLexical)
			if err != nil {
				t.Fatalf("ParseValue(%q) error = %v", tt.testLexical, err)
			}

			if err := facet.Validate(typed, baseType); err != nil {
				t.Fatalf("Validate(%q) error = %v", tt.testLexical, err)
			}
		})
	}
}
