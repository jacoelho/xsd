package semanticcheck

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateFacetConstraints validates facet consistency and values for a base type.
func ValidateFacetConstraints(schema *parser.Schema, facetList []types.Facet, baseType types.Type, baseQName types.QName) error {
	input := facetengine.SchemaConstraintInput{
		FacetList: facetList,
		BaseType:  baseType,
		BaseQName: baseQName,
	}
	callbacks := facetengine.SchemaConstraintCallbacks{
		ValidateRangeConsistency: facetengine.ValidateRangeConsistency,
		ValidateRangeValues:      facetengine.ValidateRangeValues,
		ValidateEnumerationValue: func(value string, baseType types.Type, context map[string]string) error {
			return validateValueAgainstTypeWithFacets(schema, value, baseType, context, make(map[types.Type]bool))
		},
	}
	return facetengine.ValidateSchemaConstraints(input, callbacks)
}
