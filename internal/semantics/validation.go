package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ValidateRestrictionFacets validates restriction facets against a value.
func ValidateRestrictionFacets(
	schema *parser.Schema,
	restriction *model.Restriction,
	baseType model.Type,
	value string,
	context map[string]string,
	convert model.DeferredFacetConverter,
) error {
	if restriction == nil || baseType == nil {
		return nil
	}
	normalized := model.NormalizeWhiteSpace(value, baseType)
	facets, err := parser.CollectRestrictionFacets(schema, restriction, baseType, convert)
	if err != nil {
		return err
	}
	return model.ValidateFacetValue(normalized, baseType, facets, context)
}

// ValidateSimpleTypeFacets validates collected simpleType facets against a value.
func ValidateSimpleTypeFacets(
	schema *parser.Schema,
	st *model.SimpleType,
	value string,
	context map[string]string,
	convert model.DeferredFacetConverter,
) error {
	if st == nil {
		return nil
	}
	normalized := model.NormalizeWhiteSpace(value, st)
	facets, err := parser.CollectSimpleTypeFacets(schema, st, convert)
	if err != nil {
		return err
	}
	return model.ValidateFacetValue(normalized, st, facets, context)
}

// ValidateQNameContext validates QName/NOTATION lexical context.
func ValidateQNameContext(value string, context map[string]string) error {
	_, err := model.ParseQNameValue(value, context)
	return err
}
