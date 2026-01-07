package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/facets"
	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// parseSimpleType parses a top-level simpleType definition
func parseSimpleType(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) error {
	name := getAttr(doc, elem, "name")
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
func parseInlineSimpleType(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.SimpleType, error) {
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
func parseSimpleTypeDefinition(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.SimpleType, error) {
	st := &types.SimpleType{}
	seenDerivation := false

	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
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
			st.SetVariety(types.AtomicVariety)
			restriction := &types.Restriction{}

			if base == "" {
				// restriction without base attribute must have an inline simpleType child
				var inlineBaseType *types.SimpleType
				for _, grandchild := range doc.Children(child) {
					if doc.NamespaceURI(grandchild) == xml.XSDNamespace && doc.LocalName(grandchild) == "simpleType" {
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
				st.ResolvedBase = inlineBaseType
				restriction.SimpleType = inlineBaseType
				// leave restriction.Base as zero QName (empty)
			} else {
				// base attribute is present - check that there's no inline simpleType child
				// (per XSD spec: "Either the base attribute or the simpleType child must be present, but not both")
				for _, grandchild := range doc.Children(child) {
					if doc.NamespaceURI(grandchild) == xml.XSDNamespace && doc.LocalName(grandchild) == "simpleType" {
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
			if err := parseFacets(doc, child, restriction, st, schema); err != nil {
				return nil, fmt.Errorf("parse facets: %w", err)
			}

			st.Restriction = restriction

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
			st.SetVariety(types.ListVariety)
			st.SetWhiteSpace(types.WhiteSpaceCollapse)

			var inlineItemType *types.SimpleType
			var restriction *types.Restriction
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xml.XSDNamespace {
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
					if err := parseFacets(doc, grandchild, restriction, st, schema); err != nil {
						return nil, fmt.Errorf("parse facets in list restriction: %w", err)
					}
					st.Restriction = restriction
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
				st.ItemType = inlineItemType // also store in st.ItemType for resolution
				st.List = &types.ListType{
					ItemType:       types.QName{}, // zero QName indicates inline type
					InlineItemType: inlineItemType,
				}
			} else {
				// itemType attribute - resolve QName
				itemTypeQName, err := resolveQName(doc, itemType, child, schema)
				if err != nil {
					return nil, err
				}
				st.List = &types.ListType{ItemType: itemTypeQName}
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
			st.SetVariety(types.UnionVariety)
			union := &types.UnionType{
				MemberTypes: []types.QName{},
				InlineTypes: []*types.SimpleType{},
			}

			// parse memberTypes attribute (space-separated list of QNames)
			if memberTypesAttr != "" {
				memberTypeNames := strings.FieldsSeq(memberTypesAttr)
				for memberTypeName := range memberTypeNames {
					memberTypeQName, err := resolveQName(doc, memberTypeName, child, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve member type %s: %w", memberTypeName, err)
					}
					union.MemberTypes = append(union.MemberTypes, memberTypeQName)
				}
			}

			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xml.XSDNamespace {
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

			st.Union = union
		default:
			return nil, fmt.Errorf("simpleType: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	parsed, err := types.NewSimpleTypeFromParsed(st)
	if err != nil {
		return nil, fmt.Errorf("simpleType: %w", err)
	}

	return parsed, nil
}

// tryResolveBaseType attempts to resolve the base type for a restriction
// Returns the Type if it can be resolved (built-in or already parsed), nil otherwise
func tryResolveBaseType(restriction *types.Restriction, schema *xsdschema.Schema) types.Type {
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

func parseFacets(doc *xml.Document, restrictionElem xml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *xsdschema.Schema) error {
	return parseFacetsWithPolicy(doc, restrictionElem, restriction, st, schema, facetAttributesDisallowed)
}

func parseFacetsWithAttributes(doc *xml.Document, restrictionElem xml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *xsdschema.Schema) error {
	return parseFacetsWithPolicy(doc, restrictionElem, restriction, st, schema, facetAttributesAllowed)
}

// parseFacetsWithPolicy parses facet elements from a restriction element.
func parseFacetsWithPolicy(doc *xml.Document, restrictionElem xml.NodeID, restriction *types.Restriction, st *types.SimpleType, schema *xsdschema.Schema, policy facetAttributePolicy) error {
	// try to resolve base type for use with constructors
	// if a nested simpleType is provided (e.g., in complex type restrictions with simpleContent),
	// use its base type instead of the restriction's base type
	var baseType types.Type
	if st != nil && st.Restriction != nil {
		// use the nested simpleType's base type
		baseType = tryResolveBaseType(st.Restriction, schema)
	} else {
		// use the restriction's base type (normal case)
		baseType = tryResolveBaseType(restriction, schema)
	}

	for _, child := range doc.Children(restrictionElem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		var facet facets.Facet

		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			continue
		case "attribute", "attributeGroup", "anyAttribute":
			if policy == facetAttributesAllowed {
				continue
			}
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", doc.LocalName(child))
		case "pattern":
			if err := validateOnlyAnnotationChildren(doc, child, "pattern"); err != nil {
				return err
			}
			// empty pattern is valid per XSD spec (matches only empty string)
			value := doc.GetAttribute(child, "value")
			facet = &facets.Pattern{Value: value}

		case "enumeration":
			if err := validateOnlyAnnotationChildren(doc, child, "enumeration"); err != nil {
				return err
			}
			// empty string is a valid enumeration value per XSD spec
			// we check if attribute is present, not if value is empty
			if !doc.HasAttribute(child, "value") {
				return fmt.Errorf("enumeration facet missing value attribute")
			}
			value := doc.GetAttribute(child, "value")
			// check if we already have an enumeration facet
			var enum *facets.Enumeration
			for _, f := range restriction.Facets {
				if e, ok := f.(*facets.Enumeration); ok {
					enum = e
					break
				}
			}
			if enum == nil {
				enum = &facets.Enumeration{Values: []string{value}}
				facet = enum
			} else {
				enum.Values = append(enum.Values, value)
				continue // skip adding duplicate
			}

		case "length":
			if err := validateOnlyAnnotationChildren(doc, child, "length"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("length facet missing value")
			}
			length, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid length value: %w", err)
			}
			// per XSD spec, length must be a non-negative integer
			if length < 0 {
				return fmt.Errorf("length value must be non-negative, got %d", length)
			}
			facet = &facets.Length{Value: length}

		case "minLength":
			if err := validateOnlyAnnotationChildren(doc, child, "minLength"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("minLength facet missing value")
			}
			length, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid minLength value: %w", err)
			}
			// per XSD spec, minLength must be a non-negative integer
			if length < 0 {
				return fmt.Errorf("minLength value must be non-negative, got %d", length)
			}
			facet = &facets.MinLength{Value: length}

		case "maxLength":
			if err := validateOnlyAnnotationChildren(doc, child, "maxLength"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("maxLength facet missing value")
			}
			length, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid maxLength value: %w", err)
			}
			// per XSD spec, maxLength must be a non-negative integer
			if length < 0 {
				return fmt.Errorf("maxLength value must be non-negative, got %d", length)
			}
			facet = &facets.MaxLength{Value: length}

		case "minInclusive":
			if err := validateOnlyAnnotationChildren(doc, child, "minInclusive"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("minInclusive facet missing value")
			}
			// try to use type-safe constructor if base type is available
			if baseType != nil {
				if f, err := facets.NewMinInclusive(value, baseType); err == nil && f != nil {
					facet = f
				} else {
					// if constructor fails due to undetermined primitive type during parsing,
					if errors.Is(err, facets.ErrCannotDeterminePrimitiveType) {
						restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
							FacetName:  "minInclusive",
							FacetValue: value,
						})
						continue
					}
					// for other errors (e.g., non-ordered type), return error
					return fmt.Errorf("minInclusive facet: %w", err)
				}
			} else {
				// base type not available yet, store as deferred facet
				restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
					FacetName:  "minInclusive",
					FacetValue: value,
				})
				continue
			}

		case "maxInclusive":
			if err := validateOnlyAnnotationChildren(doc, child, "maxInclusive"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("maxInclusive facet missing value")
			}
			// try to use type-safe constructor if base type is available
			if baseType != nil {
				if f, err := facets.NewMaxInclusive(value, baseType); err == nil && f != nil {
					facet = f
				} else {
					// if constructor fails due to undetermined primitive type during parsing,
					if errors.Is(err, facets.ErrCannotDeterminePrimitiveType) {
						restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
							FacetName:  "maxInclusive",
							FacetValue: value,
						})
						continue
					}
					// for other errors (e.g., non-ordered type), return error
					return fmt.Errorf("maxInclusive facet: %w", err)
				}
			} else {
				// base type not available yet, store as deferred facet
				restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
					FacetName:  "maxInclusive",
					FacetValue: value,
				})
				continue
			}

		case "minExclusive":
			if err := validateOnlyAnnotationChildren(doc, child, "minExclusive"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("minExclusive facet missing value")
			}
			// try to use type-safe constructor if base type is available
			if baseType != nil {
				if f, err := facets.NewMinExclusive(value, baseType); err == nil && f != nil {
					facet = f
				} else {
					// if constructor fails due to undetermined primitive type during parsing,
					if errors.Is(err, facets.ErrCannotDeterminePrimitiveType) {
						restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
							FacetName:  "minExclusive",
							FacetValue: value,
						})
						continue
					}
					// for other errors (e.g., non-ordered type), return error
					return fmt.Errorf("minExclusive facet: %w", err)
				}
			} else {
				// base type not available yet, store as deferred facet
				restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
					FacetName:  "minExclusive",
					FacetValue: value,
				})
				continue
			}

		case "maxExclusive":
			if err := validateOnlyAnnotationChildren(doc, child, "maxExclusive"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("maxExclusive facet missing value")
			}
			// try to use type-safe constructor if base type is available
			if baseType != nil {
				if f, err := facets.NewMaxExclusive(value, baseType); err == nil && f != nil {
					facet = f
				} else {
					// if constructor fails due to undetermined primitive type during parsing,
					if errors.Is(err, facets.ErrCannotDeterminePrimitiveType) {
						restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
							FacetName:  "maxExclusive",
							FacetValue: value,
						})
						continue
					}
					// for other errors (e.g., non-ordered type), return error
					return fmt.Errorf("maxExclusive facet: %w", err)
				}
			} else {
				// base type not available yet, store as deferred facet
				restriction.Facets = append(restriction.Facets, &facets.DeferredFacet{
					FacetName:  "maxExclusive",
					FacetValue: value,
				})
				continue
			}

		case "totalDigits":
			if err := validateOnlyAnnotationChildren(doc, child, "totalDigits"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("totalDigits facet missing value")
			}
			digits, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid totalDigits value: %w", err)
			}
			// per XSD spec, totalDigits must be a positive integer (>0)
			if digits <= 0 {
				return fmt.Errorf("totalDigits value must be positive, got %d", digits)
			}
			facet = &facets.TotalDigits{Value: digits}

		case "fractionDigits":
			if err := validateOnlyAnnotationChildren(doc, child, "fractionDigits"); err != nil {
				return err
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("fractionDigits facet missing value")
			}
			digits, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid fractionDigits value: %w", err)
			}
			// per XSD spec, fractionDigits must be a non-negative integer
			if digits < 0 {
				return fmt.Errorf("fractionDigits value must be non-negative, got %d", digits)
			}
			facet = &facets.FractionDigits{Value: digits}

		case "whiteSpace":
			if err := validateOnlyAnnotationChildren(doc, child, "whiteSpace"); err != nil {
				return err
			}
			// parse whiteSpace facet and set it on the SimpleType (if present)
			if st == nil {
				// complex content restrictions don't have SimpleType
				continue
			}
			value := doc.GetAttribute(child, "value")
			if value == "" {
				return fmt.Errorf("whiteSpace facet missing value")
			}
			// this allows validation to detect invalid relaxations of the constraint
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
			continue

		default:
			// unknown facet - reject it as invalid
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", doc.LocalName(child))
		}

		if facet != nil {
			restriction.Facets = append(restriction.Facets, facet)
		}
	}

	return nil
}

// hasIDAttribute checks if an element has an id attribute (even if empty)
func hasIDAttribute(doc *xml.Document, elem xml.NodeID) bool {
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "id" && attr.NamespaceURI() == "" {
			return true
		}
	}
	return false
}

// validateIDAttribute validates that an id attribute is a valid NCName.
// Per XSD spec, id attributes on schema components must be valid NCNames.
// Also registers the id for uniqueness validation.
func validateIDAttribute(id string, elementName string, schema *xsdschema.Schema) error {
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
