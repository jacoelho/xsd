package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func ValidateFacetConstraints(schema *parser.Schema, facetList []model.Facet, baseType model.Type, baseQName model.QName) error {
	return facets.ValidateSchemaConstraints(
		facets.SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
		facets.SchemaConstraintCallbacks{
			ValidateRangeConsistency: facets.ValidateRangeConsistency,
			ValidateRangeValues:      facets.ValidateRangeValues,
			ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
				return validateValueAgainstTypeWithFacets(schema, value, baseType, context)
			},
		},
	)
}
