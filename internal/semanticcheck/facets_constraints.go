package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
)

// ValidateFacetConstraints validates facet consistency and values for a base type.
func ValidateFacetConstraints(schema *parser.Schema, facetList []model.Facet, baseType model.Type, baseQName model.QName) error {
	input := facetengine.SchemaConstraintInput{
		FacetList: facetList,
		BaseType:  baseType,
		BaseQName: baseQName,
	}
	callbacks := facetengine.SchemaConstraintCallbacks{
		ValidateRangeConsistency: facetengine.ValidateRangeConsistency,
		ValidateRangeValues:      facetengine.ValidateRangeValues,
		ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
			return validateValueAgainstTypeWithFacets(schema, value, baseType, context, make(map[model.Type]bool))
		},
	}
	return facetengine.ValidateSchemaConstraints(input, callbacks)
}
