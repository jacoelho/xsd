package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type facetAttributePolicy int

const (
	facetAttributesDisallowed facetAttributePolicy = iota
	facetAttributesAllowed
)

// parseFacetsWithPolicy parses facet elements from a restriction element.
func parseFacetsWithPolicy(doc *xmltree.Document, restrictionElem xmltree.NodeID, restriction *model.Restriction, st *model.SimpleType, schema *Schema, policy facetAttributePolicy) error {
	baseType := tryResolveBaseType(restriction, schema)
	if st != nil && st.Restriction != nil {
		baseType = tryResolveBaseType(st.Restriction, schema)
	}

	for _, child := range doc.Children(restrictionElem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
			continue
		}

		localName := doc.LocalName(child)
		switch localName {
		case "annotation":
			continue
		case "simpleType":
			continue
		case "attribute", "attributeGroup", "anyAttribute":
			if policy == facetAttributesAllowed {
				continue
			}
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", localName)
		}

		var (
			facet model.Facet
			err   error
		)

		if localName == "whiteSpace" {
			if st == nil {
				if restriction != nil && restriction.SimpleType == nil {
					restriction.SimpleType = &model.SimpleType{}
				}
				st = restriction.SimpleType
			}
			if st == nil {
				continue
			}
			err = applyWhiteSpaceFacet(doc, child, st)
			if err != nil {
				return err
			}
			continue
		}
		if parser := directFacetParsers[localName]; parser != nil {
			facet, err = parser(doc, child)
		} else if localName == "enumeration" {
			facet, err = parseEnumerationFacet(doc, child, restriction, schema)
		} else if constructor := orderedFacetConstructors[localName]; constructor != nil {
			facet, err = parseOrderedFacet(doc, child, restriction, baseType, localName, constructor)
		} else {
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", localName)
		}

		if err != nil {
			return err
		}
		if facet != nil {
			restriction.Facets = append(restriction.Facets, facet)
		}
	}

	for _, facet := range restriction.Facets {
		if enum, ok := facet.(*model.Enumeration); ok {
			enum.Seal()
		}
	}

	return nil
}
