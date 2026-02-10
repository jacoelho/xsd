package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseRestrictionDerivation(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.SimpleType, error) {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, err
	}

	base := doc.GetAttribute(elem, "base")
	restriction := &model.Restriction{}
	facetType := &model.SimpleType{}

	if base == "" {
		var inlineBaseType *model.SimpleType
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) == schemaxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
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
			if doc.NamespaceURI(child) == schemaxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
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

	parsed, err := model.NewAtomicSimpleType(model.QName{}, "", restriction)
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
func tryResolveBaseType(restriction *model.Restriction, schema *Schema) model.Type {
	if restriction.Base.IsZero() {
		return nil
	}

	if builtinType := builtins.GetNS(restriction.Base.Namespace, restriction.Base.Local); builtinType != nil {
		return builtinType
	}

	if typeDef, ok := schema.TypeDefs[restriction.Base]; ok {
		if ct, ok := model.AsComplexType(typeDef); ok {
			if _, ok := ct.Content().(*model.SimpleContent); ok {
				return model.ResolveSimpleContentBaseType(ct.BaseType())
			}
			return nil
		}
		return typeDef
	}

	return nil
}
