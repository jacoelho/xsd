package loader

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/types"
)

func TestFacetApplicability_LengthOnListType(t *testing.T) {
	// Length facets are applicable to list types, even if itemType is numeric
	// Per XSD spec: for list types, length counts list items, not string length

	// Create a list type with numeric itemType (integer)
	integerType := types.GetBuiltin(types.TypeNameInteger)
	if integerType == nil {
		t.Fatal("integer type not found")
	}

	listType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "IntegerList",
		},
		List: &types.ListType{
			ItemType: integerType.Name(),
		},
	}
	listType.SetVariety(types.ListVariety)
	listType.ItemType = integerType

	// Test length facet on list type
	lengthFacet := &facets.Length{Value: 5}
	facetList := []facets.Facet{lengthFacet}
	baseQName := listType.QName

	err := validateFacetConstraints(facetList, listType, baseQName)
	if err != nil {
		t.Errorf("length facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_MaxLengthOnListType(t *testing.T) {
	// maxLength facets are applicable to list types, even if itemType is numeric

	integerType := types.GetBuiltin(types.TypeNameInteger)
	if integerType == nil {
		t.Fatal("integer type not found")
	}

	listType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "IntegerList",
		},
		List: &types.ListType{
			ItemType: integerType.Name(),
		},
	}
	listType.SetVariety(types.ListVariety)
	listType.ItemType = integerType

	// Test maxLength facet on list type
	maxLengthFacet := &facets.MaxLength{Value: 3}
	facetList := []facets.Facet{maxLengthFacet}
	baseQName := listType.QName

	err := validateFacetConstraints(facetList, listType, baseQName)
	if err != nil {
		t.Errorf("maxLength facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_MinLengthOnListType(t *testing.T) {
	// minLength facets are applicable to list types, even if itemType is numeric

	decimalType := types.GetBuiltin(types.TypeNameDecimal)
	if decimalType == nil {
		t.Fatal("decimal type not found")
	}

	listType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DecimalList",
		},
		List: &types.ListType{
			ItemType: decimalType.Name(),
		},
	}
	listType.SetVariety(types.ListVariety)
	listType.ItemType = decimalType

	// Test minLength facet on list type
	minLengthFacet := &facets.MinLength{Value: 2}
	facetList := []facets.Facet{minLengthFacet}
	baseQName := listType.QName

	err := validateFacetConstraints(facetList, listType, baseQName)
	if err != nil {
		t.Errorf("minLength facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_LengthOnAtomicNumericType(t *testing.T) {
	// Length facets are NOT applicable to atomic numeric types (should fail)

	integerType := types.GetBuiltin(types.TypeNameInteger)
	if integerType == nil {
		t.Fatal("integer type not found")
	}

	atomicType := &types.SimpleType{
		QName: types.QName{
			Namespace: types.XSDNamespace,
			Local:     "integer",
		},
	}
	atomicType.SetVariety(types.AtomicVariety)
	atomicType.MarkBuiltin()

	lengthFacet := &facets.Length{Value: 5}
	facetList := []facets.Facet{lengthFacet}
	baseQName := types.QName{
		Namespace: types.XSDNamespace,
		Local:     "integer",
	}

	err := validateFacetConstraints(facetList, atomicType, baseQName)
	if err == nil {
		t.Error("length facet should NOT be applicable to atomic numeric type, but validation passed")
	} else if !strings.Contains(err.Error(), "not applicable") {
		t.Errorf("expected error about facet not applicable, got: %v", err)
	}
}
