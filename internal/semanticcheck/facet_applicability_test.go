package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

func TestFacetApplicability_LengthOnListType(t *testing.T) {
	// length facets are applicable to list types, even if itemType is numeric
	// per XSD spec: for list types, length counts list items, not string length

	// create a list type with numeric itemType (integer)
	integerType := builtins.Get(model.TypeNameInteger)
	if integerType == nil {
		t.Fatal("integer type not found")
	}

	listType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "IntegerList",
		},
		List: &model.ListType{
			ItemType: integerType.Name(),
		},
	}
	listType.ItemType = integerType

	// test length facet on list type
	lengthFacet := &model.Length{Value: 5}
	facetList := []model.Facet{lengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
	if err != nil {
		t.Errorf("length facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_MaxLengthOnListType(t *testing.T) {
	// maxLength facets are applicable to list types, even if itemType is numeric

	integerType := builtins.Get(model.TypeNameInteger)
	if integerType == nil {
		t.Fatal("integer type not found")
	}

	listType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "IntegerList",
		},
		List: &model.ListType{
			ItemType: integerType.Name(),
		},
	}
	listType.ItemType = integerType

	// test maxLength facet on list type
	maxLengthFacet := &model.MaxLength{Value: 3}
	facetList := []model.Facet{maxLengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
	if err != nil {
		t.Errorf("maxLength facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_MinLengthOnListType(t *testing.T) {
	// minLength facets are applicable to list types, even if itemType is numeric

	decimalType := builtins.Get(model.TypeNameDecimal)
	if decimalType == nil {
		t.Fatal("decimal type not found")
	}

	listType := &model.SimpleType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "DecimalList",
		},
		List: &model.ListType{
			ItemType: decimalType.Name(),
		},
	}
	listType.ItemType = decimalType

	// test minLength facet on list type
	minLengthFacet := &model.MinLength{Value: 2}
	facetList := []model.Facet{minLengthFacet}
	baseQName := listType.QName

	err := ValidateFacetConstraints(nil, facetList, listType, baseQName)
	if err != nil {
		t.Errorf("minLength facet should be applicable to list type, but got error: %v", err)
	}
}

func TestFacetApplicability_LengthOnAtomicNumericType(t *testing.T) {
	// length facets are NOT applicable to atomic numeric types (should fail)

	atomicType, err := builtins.NewSimpleType(model.TypeNameInteger)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(integer) failed: %v", err)
	}

	lengthFacet := &model.Length{Value: 5}
	facetList := []model.Facet{lengthFacet}
	baseQName := atomicType.Name()

	err = ValidateFacetConstraints(nil, facetList, atomicType, baseQName)
	if err == nil {
		t.Error("length facet should NOT be applicable to atomic numeric type, but validation passed")
	} else if !strings.Contains(err.Error(), "not applicable") {
		t.Errorf("expected error about facet not applicable, got: %v", err)
	}
}

func TestFacetApplicability_LengthOnDurationType(t *testing.T) {
	durationType, err := builtins.NewSimpleType(model.TypeNameDuration)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(duration) failed: %v", err)
	}

	lengthFacet := &model.Length{Value: 3}
	facetList := []model.Facet{lengthFacet}
	baseQName := durationType.Name()

	err = ValidateFacetConstraints(nil, facetList, durationType, baseQName)
	if err == nil {
		t.Error("length facet should NOT be applicable to duration type, but validation passed")
	} else if !strings.Contains(err.Error(), "not applicable") {
		t.Errorf("expected error about facet not applicable, got: %v", err)
	}
}
