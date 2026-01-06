package loader

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func TestFacetInheritance_SimpleType(t *testing.T) {
	// Test that facets are inherited from base type
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// Base type with maxInclusive=100
	decimalBaseType := types.GetBuiltin(types.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	maxInclusive100, err := facets.NewMaxInclusive("100", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(types.TypeNameDecimal),
			},
			Facets: []any{
				maxInclusive100,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	baseType.SetVariety(types.AtomicVariety)
	schema.TypeDefs[baseType.QName] = baseType

	// Derived type with maxInclusive=50 (stricter - should be valid)
	// Use the primitive type (decimal) for facet creation
	maxInclusive50, err := facets.NewMaxInclusive("50", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				maxInclusive50,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// Validate - should pass (50 < 100, so it's stricter)
	errs := ValidateSchema(schema)
	for _, err := range errs {
		t.Logf("Validation error: %v", err)
	}
	// Should not have errors about facet restriction
	hasFacetError := false
	for _, err := range errs {
		if err.Error() != "" && (strings.Contains(err.Error(), "maxInclusive") || strings.Contains(err.Error(), "facet")) {
			hasFacetError = true
			break
		}
	}
	if hasFacetError {
		t.Error("Should not have facet restriction errors for valid restriction")
	}
}

func TestFacetInheritance_InvalidRelaxation(t *testing.T) {
	// Test that relaxing facets is rejected
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// Base type with maxInclusive=100
	decimalBaseType := types.GetBuiltin(types.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	maxInclusive100, err := facets.NewMaxInclusive("100", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(types.TypeNameDecimal),
			},
			Facets: []any{
				maxInclusive100,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	baseType.SetVariety(types.AtomicVariety)
	schema.TypeDefs[baseType.QName] = baseType

	// Derived type with maxInclusive=200 (relaxed - should be invalid)
	// Use the primitive type (decimal) for facet creation
	maxInclusive200, err := facets.NewMaxInclusive("200", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				maxInclusive200,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// Validate - should fail (200 > 100, so it's relaxed)
	errs := ValidateSchema(schema)
	hasFacetError := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "maxInclusive") || strings.Contains(err.Error(), "facet") || strings.Contains(err.Error(), "stricter") {
			hasFacetError = true
			break
		}
	}
	if !hasFacetError {
		t.Error("Should have facet restriction error for relaxed facet (maxInclusive 200 > 100)")
	}
}

func TestFacetInheritance_MinInclusive(t *testing.T) {
	// Test minInclusive facet inheritance
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// Base type with minInclusive=10
	decimalBaseType := types.GetBuiltin(types.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	minInclusive10, err := facets.NewMinInclusive("10", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(types.TypeNameDecimal),
			},
			Facets: []any{
				minInclusive10,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	baseType.SetVariety(types.AtomicVariety)
	schema.TypeDefs[baseType.QName] = baseType

	// Derived type with minInclusive=20 (stricter - should be valid)
	// Use the primitive type (decimal) for facet creation
	minInclusive20, err := facets.NewMinInclusive("20", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				minInclusive20,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// Validate - should pass (20 > 10, so it's stricter)
	errs := ValidateSchema(schema)
	hasFacetError := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "minInclusive") || strings.Contains(err.Error(), "facet") {
			hasFacetError = true
			break
		}
	}
	if hasFacetError {
		t.Error("Should not have facet restriction errors for valid restriction (minInclusive 20 > 10)")
	}

	// Test invalid relaxation: minInclusive=5 (relaxed - should be invalid)
	// Use the primitive type (decimal) for facet creation
	minInclusive5, err := facets.NewMinInclusive("5", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	invalidDerived := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "InvalidDerived",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				minInclusive5,
			},
		},
	}
	invalidDerived.ResolvedBase = baseType
	schema.TypeDefs[invalidDerived.QName] = invalidDerived

	errs2 := ValidateSchema(schema)
	hasFacetError2 := false
	for _, err := range errs2 {
		if strings.Contains(err.Error(), "minInclusive") || strings.Contains(err.Error(), "facet") || strings.Contains(err.Error(), "stricter") {
			hasFacetError2 = true
			break
		}
	}
	if !hasFacetError2 {
		t.Error("Should have facet restriction error for relaxed facet (minInclusive 5 < 10)")
	}
}

func TestFacetInheritance_DigitsRelaxation(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	decimalBaseType := types.GetBuiltin(types.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}

	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(types.TypeNameDecimal),
			},
			Facets: []any{
				&facets.TotalDigits{Value: 4},
				&facets.FractionDigits{Value: 2},
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	baseType.SetVariety(types.AtomicVariety)
	schema.TypeDefs[baseType.QName] = baseType

	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				&facets.TotalDigits{Value: 5},
				&facets.FractionDigits{Value: 4},
			},
		},
	}
	derivedType.ResolvedBase = baseType
	derivedType.SetVariety(types.AtomicVariety)
	schema.TypeDefs[derivedType.QName] = derivedType

	errs := ValidateSchema(schema)
	hasFacetError := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "totalDigits") || strings.Contains(err.Error(), "fractionDigits") || strings.Contains(err.Error(), "facet") {
			hasFacetError = true
			break
		}
	}
	if !hasFacetError {
		t.Error("Should have facet restriction error for relaxed digit facets")
	}
}

func TestFacetInheritance_MaxLength(t *testing.T) {
	// Test maxLength facet inheritance
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// Base type with maxLength=100
	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(types.TypeNameString),
			},
			Facets: []any{
				&facets.MaxLength{Value: 100},
			},
		},
	}
	schema.TypeDefs[baseType.QName] = baseType

	// Derived type with maxLength=50 (stricter - should be valid)
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Variety set via SetVariety
		Restriction: &types.Restriction{
			Base: baseType.QName,
			Facets: []any{
				&facets.MaxLength{Value: 50},
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// Validate - should pass
	errs := ValidateSchema(schema)
	hasFacetError := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "maxLength") || strings.Contains(err.Error(), "facet") {
			hasFacetError = true
			break
		}
	}
	if hasFacetError {
		t.Error("Should not have facet restriction errors for valid restriction (maxLength 50 < 100)")
	}
}
