package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateSchemaEnumerationValues(schema *parser.Schema, facetList []model.Facet, baseType model.Type) error {
	return validateEnumerationValues(facetList, baseType, func(value string, baseType model.Type, context map[string]string) error {
		return validateValueAgainstTypeWithFacets(schema, value, baseType, context)
	})
}
