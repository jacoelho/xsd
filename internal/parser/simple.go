package parser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// parseSimpleType parses a top-level simpleType definition
func parseSimpleType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("simpleType missing name attribute")
	}

	if doc.HasAttribute(elem, "id") {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "simpleType", schema); err != nil {
			return err
		}
	}

	st, err := parseSimpleTypeDefinition(doc, elem, schema)
	if err != nil {
		return err
	}

	st.QName = types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	st.SourceNamespace = schema.TargetNamespace

	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if finalAttr == "" {
			st.Final = 0
		} else {
			final, err := parseSimpleTypeFinal(finalAttr)
			if err != nil {
				return fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
			}
			st.Final = final
		}
	} else if schema.FinalDefault != 0 {
		// apply finalDefault (restriction, list, union only)
		st.Final = schema.FinalDefault & types.DerivationSet(types.DerivationRestriction|types.DerivationList|types.DerivationUnion)
	}

	if _, exists := schema.TypeDefs[st.QName]; exists {
		return fmt.Errorf("duplicate type definition: '%s'", st.QName)
	}

	schema.TypeDefs[st.QName] = st
	return nil
}

// parseSimpleTypeFinal parses the final attribute value for simpleType
// Valid values: #all, restriction, list, union (space-separated)
func parseSimpleTypeFinal(value string) (types.DerivationSet, error) {
	return parseDerivationSetWithValidation(value, types.DerivationSet(types.DerivationRestriction|types.DerivationList|types.DerivationUnion))
}

// parseInlineSimpleType parses an inline simpleType definition.
func parseInlineSimpleType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
	if doc.GetAttribute(elem, "name") != "" {
		return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
	}
	if doc.HasAttribute(elem, "id") {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "simpleType", schema); err != nil {
			return nil, err
		}
	}
	return parseSimpleTypeDefinition(doc, elem, schema)
}

// parseSimpleTypeDefinition parses the derivation content of a simpleType element.
func parseSimpleTypeDefinition(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.SimpleType, error) {
	var parsed *types.SimpleType
	seenDerivation := false

	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "restriction":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			// validate id attribute if present
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "restriction", schema); err != nil {
					return nil, err
				}
			}

			base := doc.GetAttribute(child, "base")
			restriction := &types.Restriction{}
			facetType := &types.SimpleType{}

			if base == "" {
				// restriction without base attribute must have an inline simpleType child
				var inlineBaseType *types.SimpleType
				for _, grandchild := range doc.Children(child) {
					if doc.NamespaceURI(grandchild) == xsdxml.XSDNamespace && doc.LocalName(grandchild) == "simpleType" {
						if inlineBaseType != nil {
							return nil, fmt.Errorf("restriction cannot have multiple simpleType children")
						}
						var err error
						inlineBaseType, err = parseInlineSimpleType(doc, grandchild, schema)
						if err != nil {
							return nil, fmt.Errorf("parse inline simpleType in restriction: %w", err)
						}
					}
				}
				if inlineBaseType == nil {
					return nil, fmt.Errorf("restriction missing base attribute and inline simpleType")
				}
				restriction.SimpleType = inlineBaseType
				// leave restriction.Base as zero QName (empty)
			} else {
				// base attribute is present - check that there's no inline simpleType child
				// (per XSD spec: "Either the base attribute or the simpleType child must be present, but not both")
				for _, grandchild := range doc.Children(child) {
					if doc.NamespaceURI(grandchild) == xsdxml.XSDNamespace && doc.LocalName(grandchild) == "simpleType" {
						return nil, fmt.Errorf("restriction cannot have both base attribute and inline simpleType child")
					}
				}
				baseQName, err := resolveQName(doc, base, child, schema)
				if err != nil {
					return nil, err
				}
				restriction.Base = baseQName
			}

			// parse facets (including whiteSpace) - this will skip the simpleType child since it's not a facet
			if err := parseFacets(doc, child, restriction, facetType, schema); err != nil {
				return nil, fmt.Errorf("parse facets: %w", err)
			}

			var err error
			parsed, err = types.NewAtomicSimpleType(types.QName{}, "", restriction)
			if err != nil {
				return nil, fmt.Errorf("simpleType: %w", err)
			}
			if facetType.WhiteSpaceExplicit() {
				parsed.SetWhiteSpaceExplicit(facetType.WhiteSpace())
			} else {
				parsed.SetWhiteSpace(facetType.WhiteSpace())
			}

		case "list":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			// validate id attribute if present
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "list", schema); err != nil {
					return nil, err
				}
			}

			itemType := doc.GetAttribute(child, "itemType")
			facetType := &types.SimpleType{}
			facetType.SetWhiteSpace(types.WhiteSpaceCollapse)

			var inlineItemType *types.SimpleType
			var restriction *types.Restriction
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xsdxml.XSDNamespace {
					continue
				}
				if doc.LocalName(grandchild) == "simpleType" {
					if inlineItemType != nil {
						return nil, fmt.Errorf("list cannot have multiple simpleType children")
					}
					var err error
					inlineItemType, err = parseInlineSimpleType(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse inline simpleType in list: %w", err)
					}
				} else if doc.LocalName(grandchild) == "restriction" {
					// list can have a restriction child with facets (like pattern)
					// this allows facets to be applied to the list type
					if restriction != nil {
						return nil, fmt.Errorf("list cannot have multiple restriction children")
					}
					restriction = &types.Restriction{}
					if err := parseFacets(doc, grandchild, restriction, facetType, schema); err != nil {
						return nil, fmt.Errorf("parse facets in list restriction: %w", err)
					}
				}
			}

			// per XSD spec: Either itemType attribute or inline simpleType child must be present, but not both
			if itemType != "" && inlineItemType != nil {
				return nil, fmt.Errorf("list cannot have both itemType attribute and inline simpleType child")
			}

			if itemType == "" && inlineItemType == nil {
				return nil, fmt.Errorf("list must have either itemType attribute or inline simpleType child")
			}

			if inlineItemType != nil {
				// inline simpleType child - store it in ListType.InlineItemType
				list := &types.ListType{
					ItemType:       types.QName{}, // zero QName indicates inline type
					InlineItemType: inlineItemType,
				}
				var err error
				parsed, err = types.NewListSimpleType(types.QName{}, "", list, restriction)
				if err != nil {
					return nil, fmt.Errorf("simpleType: %w", err)
				}
			} else {
				// itemType attribute - resolve QName
				itemTypeQName, err := resolveQName(doc, itemType, child, schema)
				if err != nil {
					return nil, err
				}
				list := &types.ListType{ItemType: itemTypeQName}
				parsed, err = types.NewListSimpleType(types.QName{}, "", list, restriction)
				if err != nil {
					return nil, fmt.Errorf("simpleType: %w", err)
				}
			}
			if facetType.WhiteSpaceExplicit() {
				parsed.SetWhiteSpaceExplicit(facetType.WhiteSpace())
			} else {
				parsed.SetWhiteSpace(facetType.WhiteSpace())
			}

		case "union":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			// validate id attribute if present
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "union", schema); err != nil {
					return nil, err
				}
			}

			memberTypesAttr := doc.GetAttribute(child, "memberTypes")
			union := &types.UnionType{
				MemberTypes: []types.QName{},
				InlineTypes: []*types.SimpleType{},
			}

			// parse memberTypes attribute (space-separated list of QNames)
			if memberTypesAttr != "" {
				for memberTypeName := range types.FieldsXMLWhitespaceSeq(memberTypesAttr) {
					memberTypeQName, err := resolveQName(doc, memberTypeName, child, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve member type %s: %w", memberTypeName, err)
					}
					union.MemberTypes = append(union.MemberTypes, memberTypeQName)
				}
			}

			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xsdxml.XSDNamespace {
					continue
				}
				if doc.LocalName(grandchild) == "simpleType" {
					inlineType, err := parseInlineSimpleType(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse inline simpleType in union: %w", err)
					}
					union.InlineTypes = append(union.InlineTypes, inlineType)
				}
			}

			var err error
			parsed, err = types.NewUnionSimpleType(types.QName{}, "", union)
			if err != nil {
				return nil, fmt.Errorf("simpleType: %w", err)
			}
		default:
			return nil, fmt.Errorf("simpleType: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if parsed == nil {
		return nil, fmt.Errorf("simpleType must have a derivation")
	}

	return parsed, nil
}

// tryResolveBaseType attempts to resolve the base type for a restriction
// Returns the Type if it can be resolved (built-in or already parsed), nil otherwise
func tryResolveBaseType(restriction *types.Restriction, schema *Schema) types.Type {
	// if BaseType is already set (from two-phase resolution), use it
	if restriction.Base.IsZero() {
		return nil
	}

	// try built-in types first
	if builtinType := types.GetBuiltinNS(restriction.Base.Namespace, restriction.Base.Local); builtinType != nil {
		// return BuiltinType directly - it implements types.Type and is in same package
		return builtinType
	}

	// try schema types (may not be available yet during parsing)
	if typeDef, ok := schema.TypeDefs[restriction.Base]; ok {
		if ct, ok := typeDef.(*types.ComplexType); ok {
			if _, ok := ct.Content().(*types.SimpleContent); ok {
				return types.ResolveSimpleContentBaseType(ct.BaseType())
			}
			return nil
		}
		return typeDef
	}

	return nil
}

type facetAttributePolicy int

const (
	facetAttributesDisallowed facetAttributePolicy = iota
	facetAttributesAllowed
)

func parseFacets(doc *xsdxml.Document, restrictionElem xsdxml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *Schema) error {
	return parseFacetsWithPolicy(doc, restrictionElem, restriction, st, schema, facetAttributesDisallowed)
}

func parseFacetsWithAttributes(doc *xsdxml.Document, restrictionElem xsdxml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *Schema) error {
	return parseFacetsWithPolicy(doc, restrictionElem, restriction, st, schema, facetAttributesAllowed)
}

// parseFacetsWithPolicy parses facet elements from a restriction element.
func parseFacetsWithPolicy(doc *xsdxml.Document, restrictionElem xsdxml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *Schema, policy facetAttributePolicy) error {
	baseType := resolveFacetBaseType(restriction, st, schema)

	for _, child := range doc.Children(restrictionElem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
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
			facet types.Facet
			err   error
		)

		switch localName {
		case "pattern":
			facet, err = parsePatternFacet(doc, child)

		case "enumeration":
			facet, err = parseEnumerationFacet(doc, child, restriction, schema)

		case "length":
			facet, err = parseLengthFacet(doc, child)

		case "minLength":
			facet, err = parseMinLengthFacet(doc, child)

		case "maxLength":
			facet, err = parseMaxLengthFacet(doc, child)

		case "minInclusive":
			facet, err = parseOrderedFacet(doc, child, restriction, baseType, "minInclusive", types.NewMinInclusive)

		case "maxInclusive":
			facet, err = parseOrderedFacet(doc, child, restriction, baseType, "maxInclusive", types.NewMaxInclusive)

		case "minExclusive":
			facet, err = parseOrderedFacet(doc, child, restriction, baseType, "minExclusive", types.NewMinExclusive)

		case "maxExclusive":
			facet, err = parseOrderedFacet(doc, child, restriction, baseType, "maxExclusive", types.NewMaxExclusive)

		case "totalDigits":
			facet, err = parseTotalDigitsFacet(doc, child)

		case "fractionDigits":
			facet, err = parseFractionDigitsFacet(doc, child)

		case "whiteSpace":
			if st == nil {
				if restriction != nil && restriction.SimpleType == nil {
					restriction.SimpleType = &types.SimpleType{}
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

		default:
			// unknown facet - reject it as invalid
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", localName)
		}

		if err != nil {
			return err
		}
		if facet != nil {
			restriction.Facets = append(restriction.Facets, facet)
		}
	}

	return nil
}

func resolveFacetBaseType(restriction *types.Restriction, st *types.SimpleType, schema *Schema) types.Type {
	if st != nil && st.Restriction != nil {
		return tryResolveBaseType(st.Restriction, schema)
	}
	return tryResolveBaseType(restriction, schema)
}

func parsePatternFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, "pattern"); err != nil {
		return nil, err
	}
	value := doc.GetAttribute(elem, "value")
	return &types.Pattern{Value: value}, nil
}

func parseEnumerationFacet(doc *xsdxml.Document, elem xsdxml.NodeID, restriction *types.Restriction, schema *Schema) (types.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, "enumeration"); err != nil {
		return nil, err
	}
	if !doc.HasAttribute(elem, "value") {
		return nil, fmt.Errorf("enumeration facet missing value attribute")
	}
	value := doc.GetAttribute(elem, "value")
	context := namespaceContextForElement(doc, elem, schema)
	if enum := findEnumerationFacet(restriction.Facets); enum != nil {
		if len(enum.ValueContexts) < len(enum.Values) {
			missing := len(enum.Values) - len(enum.ValueContexts)
			enum.ValueContexts = append(enum.ValueContexts, make([]map[string]string, missing)...)
		}
		enum.Values = append(enum.Values, value)
		enum.ValueContexts = append(enum.ValueContexts, context)
		return nil, nil
	}
	return &types.Enumeration{
		Values:        []string{value},
		ValueContexts: []map[string]string{context},
	}, nil
}

func findEnumerationFacet(facets []any) *types.Enumeration {
	for _, facet := range facets {
		if enum, ok := facet.(*types.Enumeration); ok {
			return enum
		}
	}
	return nil
}

func parseLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "length")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("length value must be non-negative, got %d", length)
	}
	return &types.Length{Value: length}, nil
}

func parseMinLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "minLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("minLength value must be non-negative, got %d", length)
	}
	return &types.MinLength{Value: length}, nil
}

func parseMaxLengthFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	length, err := parseFacetValueInt(doc, elem, "maxLength")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("maxLength value must be non-negative, got %d", length)
	}
	return &types.MaxLength{Value: length}, nil
}

type orderedFacetConstructor func(string, types.Type) (types.Facet, error)

func parseOrderedFacet(doc *xsdxml.Document, elem xsdxml.NodeID, restriction *types.Restriction, baseType types.Type, facetName string, constructor orderedFacetConstructor) (types.Facet, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, facetName); err != nil {
		return nil, err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return nil, fmt.Errorf("%s facet missing value", facetName)
	}
	if baseType == nil {
		deferFacet(restriction, facetName, value)
		return nil, nil
	}

	facet, err := constructor(value, baseType)
	if err == nil && facet != nil {
		return facet, nil
	}
	if errors.Is(err, types.ErrCannotDeterminePrimitiveType) {
		deferFacet(restriction, facetName, value)
		return nil, nil
	}
	if err == nil {
		return nil, fmt.Errorf("%s facet: %s", facetName, "missing facet")
	}
	return nil, fmt.Errorf("%s facet: %w", facetName, err)
}

func parseTotalDigitsFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "totalDigits")
	if err != nil {
		return nil, err
	}
	if digits <= 0 {
		return nil, fmt.Errorf("totalDigits value must be positive, got %d", digits)
	}
	return &types.TotalDigits{Value: digits}, nil
}

func parseFractionDigitsFacet(doc *xsdxml.Document, elem xsdxml.NodeID) (types.Facet, error) {
	digits, err := parseFacetValueInt(doc, elem, "fractionDigits")
	if err != nil {
		return nil, err
	}
	if digits < 0 {
		return nil, fmt.Errorf("fractionDigits value must be non-negative, got %d", digits)
	}
	return &types.FractionDigits{Value: digits}, nil
}

func parseFacetValueInt(doc *xsdxml.Document, elem xsdxml.NodeID, facetName string) (int, error) {
	if err := validateOnlyAnnotationChildren(doc, elem, facetName); err != nil {
		return 0, err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return 0, fmt.Errorf("%s facet missing value", facetName)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %w", facetName, err)
	}
	return parsed, nil
}

func applyWhiteSpaceFacet(doc *xsdxml.Document, elem xsdxml.NodeID, st *types.SimpleType) error {
	if err := validateOnlyAnnotationChildren(doc, elem, "whiteSpace"); err != nil {
		return err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return fmt.Errorf("whiteSpace facet missing value")
	}
	switch value {
	case "preserve":
		st.SetWhiteSpaceExplicit(types.WhiteSpacePreserve)
	case "replace":
		st.SetWhiteSpaceExplicit(types.WhiteSpaceReplace)
	case "collapse":
		st.SetWhiteSpaceExplicit(types.WhiteSpaceCollapse)
	default:
		return fmt.Errorf("invalid whiteSpace value: %s", value)
	}
	return nil
}

func deferFacet(restriction *types.Restriction, facetName, facetValue string) {
	restriction.Facets = append(restriction.Facets, &types.DeferredFacet{
		FacetName:  facetName,
		FacetValue: facetValue,
	})
}

// hasIDAttribute checks if an element has an id attribute (even if empty)
func hasIDAttribute(doc *xsdxml.Document, elem xsdxml.NodeID) bool {
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "id" && attr.NamespaceURI() == "" {
			return true
		}
	}
	return false
}

// validateIDAttribute validates that an id attribute is a valid NCName.
// Per XSD spec, id attributes on schema components must be valid NCNames.
// Also registers the id for uniqueness schemacheck.
func validateIDAttribute(id, elementName string, schema *Schema) error {
	if !types.IsValidNCName(id) {
		return fmt.Errorf("%s element has invalid id attribute '%s': must be a valid NCName", elementName, id)
	}
	// register id for uniqueness validation
	if existing, exists := schema.IDAttributes[id]; exists {
		return fmt.Errorf("%s element has duplicate id attribute '%s' (already used by %s)", elementName, id, existing)
	}
	schema.IDAttributes[id] = elementName
	return nil
}
