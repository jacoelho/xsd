package facets

import (
	"testing"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
	"github.com/jacoelho/xsd/internal/types"
)

func TestNewMinInclusive_Decimal(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameDecimal))

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

	// Test validation
	testVal, _ := lexicalparser.ParseDecimal("150.75")
	testTypedValue := types.NewDecimalValue(types.NewParsedValue("150.75", testVal), decimalType)
	if err := facet.Validate(testTypedValue, decimalType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// Test invalid value
	invalidVal, _ := lexicalparser.ParseDecimal("50")
	invalidTypedValue := types.NewDecimalValue(types.NewParsedValue("50", invalidVal), decimalType)
	if err := facet.Validate(invalidTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value < min")
	}
}

func TestNewMaxInclusive_Integer(t *testing.T) {
	integerType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// Variety set via SetVariety
	}
	integerType.MarkBuiltin()
	integerType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameInteger))
	// Integer's primitive is decimal
	decimalType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	integerType.SetPrimitiveType(decimalType)

	facet, err := NewMaxInclusive("100", integerType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMaxInclusive() returned nil")
	}

	// Test validation
	testVal, _ := lexicalparser.ParseInteger("50")
	testTypedValue := types.NewIntegerValue(types.NewParsedValue("50", testVal), integerType)
	if err := facet.Validate(testTypedValue, integerType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinInclusive_DateTime(t *testing.T) {
	dateTimeType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "dateTime",
		},
		// Variety set via SetVariety
	}
	dateTimeType.MarkBuiltin()
	dateTimeType.SetPrimitiveType(dateTimeType)
	dateTimeType.SetFundamentalFacets(types.ComputeFundamentalFacets("dateTime"))

	facet, err := NewMinInclusive("2001-01-01T00:00:00", dateTimeType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}

	// Test validation
	testVal, _ := lexicalparser.ParseDateTime("2001-06-01T00:00:00")
	testTypedValue := types.NewDateTimeValue(types.NewParsedValue("2001-06-01T00:00:00", testVal), dateTimeType)
	if err := facet.Validate(testTypedValue, dateTimeType); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinExclusive(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameDecimal))

	facet, err := NewMinExclusive("100", decimalType)
	if err != nil {
		t.Fatalf("NewMinExclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMinExclusive() returned nil")
	}

	// Test validation - value equal to min should fail (exclusive)
	equalVal, _ := lexicalparser.ParseDecimal("100")
	equalTypedValue := types.NewDecimalValue(types.NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == min (exclusive)")
	}
}

func TestNewMaxExclusive(t *testing.T) {
	decimalType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(types.AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameDecimal))

	facet, err := NewMaxExclusive("100", decimalType)
	if err != nil {
		t.Fatalf("NewMaxExclusive() error = %v", err)
	}

	if facet == nil {
		t.Fatal("NewMaxExclusive() returned nil")
	}

	// Test validation - value equal to max should fail (exclusive)
	equalVal, _ := lexicalparser.ParseDecimal("100")
	equalTypedValue := types.NewDecimalValue(types.NewParsedValue("100", equalVal), decimalType)
	if err := facet.Validate(equalTypedValue, decimalType); err == nil {
		t.Error("Validate() should return error for value == max (exclusive)")
	}
}

func TestNewMinInclusive_InvalidType(t *testing.T) {
	// Create a string type (not comparable for range facets)
	stringType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "string",
		},
		// Variety set via SetVariety
	}
	stringType.MarkBuiltin()
	stringType.SetPrimitiveType(stringType)
	stringType.SetFundamentalFacets(types.ComputeFundamentalFacets(types.TypeNameString))

	// Should return error for non-comparable type
	_, err := NewMinInclusive("test", stringType)
	if err == nil {
		t.Error("NewMinInclusive() should return error for non-comparable type")
	}
}

func TestNewMinInclusive_WithBuiltinBase(t *testing.T) {
	// Test case from issue 003: SimpleType with BuiltinType as ResolvedBase
	// This tests that PrimitiveType() correctly handles BuiltinType as base
	intBuiltin := types.GetBuiltin(types.TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	// Create a derived type that restricts int (BuiltinType)
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &types.Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin
	derivedType.SetVariety(types.AtomicVariety)

	// Create facet using constructor - should work now that PrimitiveType() handles BuiltinType
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
	// Test nested derivation: moreDerived -> derived -> int (builtin)
	// This matches the example in issue 003
	intBuiltin := types.GetBuiltin(types.TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	// First level: derived restricts int
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &types.Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin
	derivedType.SetVariety(types.AtomicVariety)

	// Second level: moreDerived restricts derived
	moreDerivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "moreDerived",
		},
		Restriction: &types.Restriction{
			Base: derivedType.QName,
		},
	}
	moreDerivedType.ResolvedBase = derivedType
	moreDerivedType.SetVariety(types.AtomicVariety)

	// Create facet using constructor - should work with nested derivation
	facet, err := NewMinInclusive("10", moreDerivedType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v (this was failing before the fix)", err)
	}

	if facet == nil {
		t.Fatal("NewMinInclusive() returned nil")
	}
}

func TestNewMinInclusive_Date(t *testing.T) {
	dateBuiltin := types.GetBuiltin(types.TypeNameDate)
	if dateBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameDate) returned nil")
	}

	// Create facet using constructor - should parse as date, not dateTime
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

	// Test validation
	testVal, _ := types.ParseDate("2000-06-01")
	dateType := types.NewSimpleType(dateBuiltin.Name(), "")
	dateType.SetVariety(types.AtomicVariety)
	dateType.MarkBuiltin()
	testTypedValue := types.NewDateTimeValue(types.NewParsedValue("2000-06-01", testVal), dateType)
	if err := facet.Validate(testTypedValue, dateBuiltin); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestNewMinInclusive_Time(t *testing.T) {
	timeBuiltin := types.GetBuiltin(types.TypeNameTime)
	if timeBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameTime) returned nil")
	}

	// Create facet using constructor - should parse as time, not dateTime
	// Note: ParseTime currently has format bugs, but this test verifies the correct parser is called
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
	gYearBuiltin := types.GetBuiltin(types.TypeNameGYear)
	if gYearBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGYear) returned nil")
	}

	// Create facet using constructor - should parse as gYear, not dateTime
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
	gYearMonthBuiltin := types.GetBuiltin(types.TypeNameGYearMonth)
	if gYearMonthBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGYearMonth) returned nil")
	}

	// Create facet using constructor - should parse as gYearMonth, not dateTime
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
	gMonthDayBuiltin := types.GetBuiltin(types.TypeNameGMonthDay)
	if gMonthDayBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGMonthDay) returned nil")
	}

	// Create facet using constructor - should parse as gMonthDay, not dateTime
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
	gDayBuiltin := types.GetBuiltin(types.TypeNameGDay)
	if gDayBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGDay) returned nil")
	}

	// Create facet using constructor - should parse as gDay, not dateTime
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
	gMonthBuiltin := types.GetBuiltin(types.TypeNameGMonth)
	if gMonthBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameGMonth) returned nil")
	}

	// Create facet using constructor - should parse as gMonth, not dateTime
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
