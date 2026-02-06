package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestFacetApplicability_LengthOnListType(t *testing.T) {
	// length facets are applicable to list types, even if itemType is numeric
	// per XSD spec: for list types, length counts list items, not string length

	// create a list type with numeric itemType (integer)
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
	listType.ItemType = integerType

	// test length facet on list type
	lengthFacet := &types.Length{Value: 5}
	facetList := []types.Facet{lengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
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
	listType.ItemType = integerType

	// test maxLength facet on list type
	maxLengthFacet := &types.MaxLength{Value: 3}
	facetList := []types.Facet{maxLengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
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
	listType.ItemType = decimalType

	// test minLength facet on list type
	minLengthFacet := &types.MinLength{Value: 2}
	facetList := []types.Facet{minLengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
	if err != nil {
		t.Errorf("minLength facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_LengthOnAtomicNumericType(t *testing.T) {
	// length facets are NOT applicable to atomic numeric types (should fail)

	atomicType, err := types.NewBuiltinSimpleType(types.TypeNameInteger)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(integer) failed: %v", err)
	}

	lengthFacet := &types.Length{Value: 5}
	facetList := []types.Facet{lengthFacet}
	baseQName := atomicType.Name()

	err = ValidateFacetConstraints(nil, facetList, atomicType, baseQName)
	if err == nil {
		t.Error("length facet should NOT be applicable to atomic numeric type, but validation passed")
	} else if !strings.Contains(err.Error(), "not applicable") {
		t.Errorf("expected error about facet not applicable, got: %v", err)
	}
}

func TestFacetApplicability_LengthOnDurationType(t *testing.T) {
	durationType, err := types.NewBuiltinSimpleType(types.TypeNameDuration)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(duration) failed: %v", err)
	}

	lengthFacet := &types.Length{Value: 3}
	facetList := []types.Facet{lengthFacet}
	baseQName := durationType.Name()

	err = ValidateFacetConstraints(nil, facetList, durationType, baseQName)
	if err == nil {
		t.Error("length facet should NOT be applicable to duration type, but validation passed")
	} else if !strings.Contains(err.Error(), "not applicable") {
		t.Errorf("expected error about facet not applicable, got: %v", err)
	}
}
