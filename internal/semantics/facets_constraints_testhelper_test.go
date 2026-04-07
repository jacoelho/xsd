package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func ValidateFacetConstraints(schema *parser.Schema, facetList []model.Facet, baseType model.Type, baseQName model.QName) error {
	return ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
		SchemaConstraintCallbacks{
			ValidateRangeConsistency: ValidateRangeConsistency,
			ValidateRangeValues:      ValidateRangeValues,
			ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
				return validateValueAgainstTypeWithFacets(schema, value, baseType, context)
			},
		},
	)
}
