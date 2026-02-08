package semanticcheck

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateFacetConstraints validates facet consistency and values for a base type.
func ValidateFacetConstraints(schema *parser.Schema, facetList []types.Facet, baseType types.Type, baseQName types.QName) error {
	return facetengine.ValidateSchemaConstraints(
		facetengine.SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
		facetengine.SchemaConstraintCallbacks{
			ValidateRangeConsistency: validateRangeFacets,
			ValidateRangeValues:      validateRangeFacetValues,
			ValidateEnumerationValue: func(value string, baseType types.Type, context map[string]string) error {
				return validateValueAgainstTypeWithFacets(schema, value, baseType, context, make(map[types.Type]bool))
			},
		},
	)
}

func isValidFacetName(name string) bool {
	return facetengine.IsValidFacetName(name)
}
