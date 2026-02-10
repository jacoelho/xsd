package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestFacetInheritance_SimpleType(t *testing.T) {
	// test that facets are inherited from base type
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	// base type with maxInclusive=100
	decimalBaseType := builtins.Get(model.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	maxInclusive100, err := facetvalue.NewMaxInclusive("100", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	baseType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(model.TypeNameDecimal),
			},
			Facets: []any{
				maxInclusive100,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	schema.TypeDefs[baseType.QName] = baseType

	// derived type with maxInclusive=50 (stricter - should be valid)
	// use the primitive type (decimal) for facet creation
	maxInclusive50, err := facetvalue.NewMaxInclusive("50", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	derivedType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				maxInclusive50,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// validate - should pass (50 < 100, so it's stricter)
	errs := ValidateStructure(schema)
	for _, err := range errs {
		t.Logf("Validation error: %v", err)
	}
	// should not have errors about facet restriction
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
	// test that relaxing facets is rejected
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	// base type with maxInclusive=100
	decimalBaseType := builtins.Get(model.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	maxInclusive100, err := facetvalue.NewMaxInclusive("100", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	baseType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(model.TypeNameDecimal),
			},
			Facets: []any{
				maxInclusive100,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	schema.TypeDefs[baseType.QName] = baseType

	// derived type with maxInclusive=200 (relaxed - should be invalid)
	// use the primitive type (decimal) for facet creation
	maxInclusive200, err := facetvalue.NewMaxInclusive("200", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMaxInclusive() error = %v", err)
	}
	derivedType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				maxInclusive200,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// validate - should fail (200 > 100, so it's relaxed)
	errs := ValidateStructure(schema)
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
	// test minInclusive facet inheritance
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	// base type with minInclusive=10
	decimalBaseType := builtins.Get(model.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}
	minInclusive10, err := facetvalue.NewMinInclusive("10", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	baseType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(model.TypeNameDecimal),
			},
			Facets: []any{
				minInclusive10,
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	schema.TypeDefs[baseType.QName] = baseType

	// derived type with minInclusive=20 (stricter - should be valid)
	// use the primitive type (decimal) for facet creation
	minInclusive20, err := facetvalue.NewMinInclusive("20", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	derivedType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				minInclusive20,
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// validate - should pass (20 > 10, so it's stricter)
	errs := ValidateStructure(schema)
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

	// test invalid relaxation: minInclusive=5 (relaxed - should be invalid)
	// use the primitive type (decimal) for facet creation
	minInclusive5, err := facetvalue.NewMinInclusive("5", decimalBaseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	invalidDerived := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "InvalidDerived",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				minInclusive5,
			},
		},
	}
	invalidDerived.ResolvedBase = baseType
	schema.TypeDefs[invalidDerived.QName] = invalidDerived

	errs2 := ValidateStructure(schema)
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

func TestFacetInheritance_DeferredFacetConversionError(t *testing.T) {
	baseType := &model.SimpleType{
		QName: model.QName{Namespace: "http://example.com", Local: "BaseType"},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: model.XSDNamespace,
				Local:     string(model.TypeNameInt),
			},
			Facets: []any{
				&model.DeferredFacet{FacetName: "minInclusive", FacetValue: "not-an-int"},
			},
		},
	}
	baseType.ResolvedBase = builtins.Get(model.TypeNameInt)

	if err := validateFacetInheritance(nil, baseType); err == nil {
		t.Fatalf("expected deferred facet conversion error")
	}
}

func TestFacetInheritance_DigitsRelaxation(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	decimalBaseType := builtins.Get(model.TypeNameDecimal)
	if decimalBaseType == nil {
		t.Fatal("decimal built-in type not found")
	}

	baseType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(model.TypeNameDecimal),
			},
			Facets: []any{
				&model.TotalDigits{Value: 4},
				&model.FractionDigits{Value: 2},
			},
		},
	}
	baseType.ResolvedBase = decimalBaseType
	schema.TypeDefs[baseType.QName] = baseType

	derivedType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				&model.TotalDigits{Value: 5},
				&model.FractionDigits{Value: 4},
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	errs := ValidateStructure(schema)
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
	// test maxLength facet inheritance
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	// base type with maxLength=100
	baseType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		Restriction: &model.Restriction{
			Base: model.QName{
				Namespace: "http://www.w3.org/2001/XMLSchema",
				Local:     string(model.TypeNameString),
			},
			Facets: []any{
				&model.MaxLength{Value: 100},
			},
		},
	}
	schema.TypeDefs[baseType.QName] = baseType

	// derived type with maxLength=50 (stricter - should be valid)
	derivedType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &model.Restriction{
			Base: baseType.QName,
			Facets: []any{
				&model.MaxLength{Value: 50},
			},
		},
	}
	derivedType.ResolvedBase = baseType
	schema.TypeDefs[derivedType.QName] = derivedType

	// validate - should pass
	errs := ValidateStructure(schema)
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
