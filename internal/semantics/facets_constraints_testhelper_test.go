package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func ValidateFacetConstraints(schema *parser.Schema, facetList []model.Facet, baseType model.Type, baseQName model.QName) error {
	if err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: facetList,
			BaseType:  baseType,
			BaseQName: baseQName,
		},
	); err != nil {
		return err
	}
	return validateSchemaEnumerationValues(schema, facetList, baseType)
}
