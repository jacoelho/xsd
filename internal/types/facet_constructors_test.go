package types

import "testing"

func TestNewMinInclusive_Decimal(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

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
	integerType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// variety set via SetVariety
	}
	integerType.MarkBuiltin()
	integerType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameInteger))
	// integer's primitive is decimal
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	integerType.SetPrimitiveType(decimalType)

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
	dateTimeType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "dateTime",
		},
		// variety set via SetVariety
	}
	dateTimeType.MarkBuiltin()
	dateTimeType.SetPrimitiveType(dateTimeType)
	dateTimeType.SetFundamentalFacets(ComputeFundamentalFacets("dateTime"))

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
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

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
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "decimal",
		},
		// variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetPrimitiveType(decimalType)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

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
	stringType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "string",
		},
		// variety set via SetVariety
	}
	stringType.MarkBuiltin()
	stringType.SetPrimitiveType(stringType)
	stringType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameString))

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
	derivedType.SetVariety(AtomicVariety)

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
	derivedType.SetVariety(AtomicVariety)

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
	moreDerivedType.SetVariety(AtomicVariety)

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
	testVal, _ := ParseDate("2000-06-01")
	dateType := NewSimpleType(dateBuiltin.Name(), "")
	dateType.SetVariety(AtomicVariety)
	dateType.MarkBuiltin()
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
