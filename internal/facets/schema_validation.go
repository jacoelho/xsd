package facets

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateRestrictionFacets validates restriction facets against a value.
func ValidateRestrictionFacets(
	schema *parser.Schema,
	restriction *types.Restriction,
	baseType types.Type,
	value string,
	context map[string]string,
	convert typeops.DeferredFacetConverter,
) error {
	if restriction == nil || baseType == nil {
		return nil
	}
	normalized := types.NormalizeWhiteSpace(value, baseType)
	facets, err := typeops.CollectRestrictionFacets(schema, restriction, baseType, convert)
	if err != nil {
		return err
	}
	return ValidateSchemaValue(SchemaInput{
		Value:    normalized,
		BaseType: baseType,
		Facets:   facets,
		Context:  context,
	})
}

// ValidateSimpleTypeFacets validates collected simpleType facets against a value.
func ValidateSimpleTypeFacets(
	schema *parser.Schema,
	st *types.SimpleType,
	value string,
	context map[string]string,
	convert typeops.DeferredFacetConverter,
) error {
	if st == nil {
		return nil
	}
	normalized := types.NormalizeWhiteSpace(value, st)
	facets, err := typeops.CollectSimpleTypeFacets(schema, st, convert)
	if err != nil {
		return err
	}
	return ValidateSchemaValue(SchemaInput{
		Value:    normalized,
		BaseType: st,
		Facets:   facets,
		Context:  context,
	})
}

// ValidateQNameContext validates QName/NOTATION lexical context.
func ValidateQNameContext(value string, context map[string]string) error {
	_, err := types.ParseQNameValue(value, context)
	return err
}
