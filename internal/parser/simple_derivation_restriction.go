package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseRestrictionDerivation(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, err
	}

	base := doc.GetAttribute(elem, "base")
	restriction := &types.Restriction{}
	facetType := &types.SimpleType{}

	if base == "" {
		var inlineBaseType *types.SimpleType
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) == xsdxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
				if inlineBaseType != nil {
					return nil, fmt.Errorf("restriction cannot have multiple simpleType children")
				}
				var err error
				inlineBaseType, err = parseInlineSimpleType(doc, child, schema)
				if err != nil {
					return nil, fmt.Errorf("parse inline simpleType in restriction: %w", err)
				}
			}
		}
		if inlineBaseType == nil {
			return nil, fmt.Errorf("restriction missing base attribute and inline simpleType")
		}
		restriction.SimpleType = inlineBaseType
	} else {
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) == xsdxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
				return nil, fmt.Errorf("restriction cannot have both base attribute and inline simpleType child")
			}
		}
		baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, err
		}
		restriction.Base = baseQName
	}

	if err := parseFacetsWithPolicy(doc, elem, restriction, facetType, schema, facetAttributesDisallowed); err != nil {
		return nil, fmt.Errorf("parse facets: %w", err)
	}

	parsed, err := types.NewAtomicSimpleType(types.QName{}, "", restriction)
	if err != nil {
		return nil, fmt.Errorf("simpleType: %w", err)
	}
	if facetType.WhiteSpaceExplicit() {
		parsed.SetWhiteSpaceExplicit(facetType.WhiteSpace())
	} else {
		parsed.SetWhiteSpace(facetType.WhiteSpace())
	}

	return parsed, nil
}

// tryResolveBaseType attempts to resolve the base type for a restriction
// Returns the Type if it can be resolved (built-in or already parsed), nil otherwise
func tryResolveBaseType(restriction *types.Restriction, schema *Schema) types.Type {
	if restriction.Base.IsZero() {
		return nil
	}

	if builtinType := types.GetBuiltinNS(restriction.Base.Namespace, restriction.Base.Local); builtinType != nil {
		return builtinType
	}

	if typeDef, ok := schema.TypeDefs[restriction.Base]; ok {
		if ct, ok := types.AsComplexType(typeDef); ok {
			if _, ok := ct.Content().(*types.SimpleContent); ok {
				return types.ResolveSimpleContentBaseType(ct.BaseType())
			}
			return nil
		}
		return typeDef
	}

	return nil
}
