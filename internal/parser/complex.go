package parser

import (
	"fmt"
	"strconv"
	"strings"

	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

var (
	validAnyElementAttributes = map[string]bool{
		"namespace":       true,
		"processContents": true,
		"minOccurs":       true,
		"maxOccurs":       true,
		"id":              true,
	}
	validAnyAttributeAttributes = map[string]bool{
		"namespace":       true,
		"processContents": true,
		"id":              true,
	}
	validModelGroupAttributes = map[string]bool{
		"id":        true,
		"minOccurs": true,
		"maxOccurs": true,
	}
	validSimpleContentFacetElements = map[string]bool{
		"length":         true,
		"minLength":      true,
		"maxLength":      true,
		"pattern":        true,
		"enumeration":    true,
		"whiteSpace":     true,
		"maxInclusive":   true,
		"maxExclusive":   true,
		"minInclusive":   true,
		"minExclusive":   true,
		"totalDigits":    true,
		"fractionDigits": true,
	}
	validComplexContentChildren = map[string]bool{
		"annotation":     true,
		"sequence":       true,
		"choice":         true,
		"all":            true,
		"group":          true,
		"element":        true,
		"any":            true,
		"attribute":      true,
		"attributeGroup": true,
		"anyAttribute":   true,
	}
	validTopLevelGroupAttributes = map[string]bool{
		"id":   true,
		"name": true,
	}
	validNamespaceConstraintTokens = map[string]bool{
		"##targetNamespace": true,
		"##local":           true,
	}
	validIdentityConstraintAttributes = map[string]bool{
		"xpath": true,
		"id":    true,
	}
)

// parseComplexType parses a top-level complexType definition
func parseComplexType(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) error {
	name := getAttr(doc, elem, "name")
	if name == "" {
		return fmt.Errorf("complexType missing name attribute")
	}

	// validate id attribute if present (must be a valid NCName, cannot be empty)
	if doc.HasAttribute(elem, "id") {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "complexType", schema); err != nil {
			return err
		}
	}

	ct, err := parseInlineComplexType(doc, elem, schema)
	if err != nil {
		return err
	}

	ct.QName = types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	ct.SourceNamespace = schema.TargetNamespace

	if _, exists := schema.TypeDefs[ct.QName]; exists {
		return fmt.Errorf("duplicate type definition: '%s'", ct.QName)
	}

	schema.TypeDefs[ct.QName] = ct
	return nil
}

// parseInlineComplexType parses a complexType definition (inline or named)
func parseInlineComplexType(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.ComplexType, error) {
	ct := &types.ComplexType{}

	if doc.HasAttribute(elem, "id") && doc.GetAttribute(elem, "name") == "" {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "complexType", schema); err != nil {
			return nil, err
		}
	}

	// parse mixed attribute - must be exactly "true" or "false", not "1", "0", etc.
	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		ct.SetMixed(value)
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "abstract"); err != nil {
		return nil, err
	} else if ok {
		ct.Abstract = value
	}

	// parse block attribute (space-separated list: extension, restriction, #all)
	if doc.HasAttribute(elem, "block") {
		blockAttr := doc.GetAttribute(elem, "block")
		if blockAttr == "" {
			ct.Block = 0
		} else {
			block, err := parseDerivationSetWithValidation(blockAttr, types.DerivationSet(types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return nil, fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
			}
			ct.Block = block
		}
	} else {
		// apply blockDefault from schema if no explicit block attribute.
		// only extension/restriction are valid for complexType.
		ct.Block = schema.BlockDefault & types.DerivationSet(types.DerivationExtension|types.DerivationRestriction)
	}

	// parse final attribute (space-separated list: extension, restriction, #all)
	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if finalAttr == "" {
			ct.Final = 0
		} else {
			final, err := parseDerivationSetWithValidation(finalAttr, types.DerivationSet(types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return nil, fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
			}
			ct.Final = final
		}
	} else if schema.FinalDefault != 0 {
		ct.Final = schema.FinalDefault & types.DerivationSet(types.DerivationExtension|types.DerivationRestriction)
	}

	// track annotation constraints: at most one, must be first
	// content model: (annotation?, (simpleContent | complexContent | ((group | all | choice | sequence)?, ((attribute | attributeGroup)*, anyAttribute?))))
	hasAnnotation := false
	hasNonAnnotation := false
	hasAnyAttribute := false
	hasParticle := false
	hasSimpleContent := false
	hasComplexContent := false
	hasAttributeLike := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("complexType: at most one annotation is allowed")
			}
			if hasNonAnnotation {
				return nil, fmt.Errorf("complexType: annotation must appear before other elements")
			}
			hasAnnotation = true

		case "sequence", "choice", "all":
			hasNonAnnotation = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
			}
			if hasAttributeLike {
				return nil, fmt.Errorf("complexType: content model must appear before attributes")
			}
			if hasParticle {
				return nil, fmt.Errorf("complexType: only one content model is allowed")
			}
			hasParticle = true
			mg, err := parseModelGroup(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse model group: %w", err)
			}
			ct.SetContent(&types.ElementContent{Particle: mg})

		case "any":
			hasNonAnnotation = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
			}
			if hasAttributeLike {
				return nil, fmt.Errorf("complexType: content model must appear before attributes")
			}
			if hasParticle {
				return nil, fmt.Errorf("complexType: only one content model is allowed")
			}
			hasParticle = true
			// xs:any as a direct child of complexType (single particle content)
			anyElem, err := parseAnyElement(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse any element: %w", err)
			}
			ct.SetContent(&types.ElementContent{Particle: anyElem})

		case "group":
			hasNonAnnotation = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
			}
			if hasAttributeLike {
				return nil, fmt.Errorf("complexType: content model must appear before attributes")
			}
			if hasParticle {
				return nil, fmt.Errorf("complexType: only one content model is allowed")
			}
			hasParticle = true
			// reference to a named group as direct child of complexType
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "group", schema); err != nil {
					return nil, err
				}
			}
			if err := validateOnlyAnnotationChildren(doc, child, "group"); err != nil {
				return nil, err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return nil, fmt.Errorf("group reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
			}
			minOccurs, err := parseOccursAttr(doc, child, "minOccurs", 1)
			if err != nil {
				return nil, err
			}
			maxOccurs, err := parseOccursAttr(doc, child, "maxOccurs", 1)
			if err != nil {
				return nil, err
			}
			groupRef := &types.GroupRef{
				RefQName:  refQName,
				MinOccurs: minOccurs,
				MaxOccurs: maxOccurs,
			}
			ct.SetContent(&types.ElementContent{Particle: groupRef})

		case "attribute":
			hasNonAnnotation = true
			hasAttributeLike = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
			}
			if hasAnyAttribute {
				return nil, fmt.Errorf("complexType: anyAttribute must appear after all attributes")
			}
			attr, err := parseAttribute(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse attribute: %w", err)
			}
			ct.SetAttributes(append(ct.Attributes(), attr))

		case "attributeGroup":
			hasNonAnnotation = true
			hasAttributeLike = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
			}
			if hasAnyAttribute {
				return nil, fmt.Errorf("complexType: anyAttribute must appear after all attributes")
			}
			// reference to an attributeGroup
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
					return nil, err
				}
			}
			if err := validateOnlyAnnotationChildren(doc, child, "attributeGroup"); err != nil {
				return nil, err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return nil, fmt.Errorf("attributeGroup reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
			}
			ct.AttrGroups = append(ct.AttrGroups, refQName)

		case "anyAttribute":
			hasNonAnnotation = true
			hasAttributeLike = true
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
			}
			if hasAnyAttribute {
				return nil, fmt.Errorf("complexType: at most one anyAttribute is allowed")
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse anyAttribute: %w", err)
			}
			ct.SetAnyAttribute(anyAttr)

		case "simpleContent":
			hasNonAnnotation = true
			if hasParticle || hasAttributeLike {
				return nil, fmt.Errorf("complexType: simpleContent must be the only content model")
			}
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: only one content model is allowed")
			}
			hasSimpleContent = true
			sc, err := parseSimpleContent(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse simpleContent: %w", err)
			}
			ct.SetContent(sc)
			// set base type if available (will be fully resolved in A7)
			if !sc.Base.IsZero() {
				ct.ResolvedBase = resolveBaseTypeForComplex(schema, sc.Base)
			}
			if sc.Extension != nil {
				ct.DerivationMethod = types.DerivationExtension
			} else if sc.Restriction != nil {
				ct.DerivationMethod = types.DerivationRestriction
			}

		case "complexContent":
			hasNonAnnotation = true
			if hasParticle || hasAttributeLike {
				return nil, fmt.Errorf("complexType: complexContent must be the only content model")
			}
			if hasSimpleContent || hasComplexContent {
				return nil, fmt.Errorf("complexType: only one content model is allowed")
			}
			hasComplexContent = true
			cc, err := parseComplexContent(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse complexContent: %w", err)
			}
			ct.SetContent(cc)
			// set base type if available (will be fully resolved in A7)
			if !cc.Base.IsZero() {
				ct.ResolvedBase = resolveBaseTypeForComplex(schema, cc.Base)
			}
			if cc.Extension != nil {
				ct.DerivationMethod = types.DerivationExtension
			} else if cc.Restriction != nil {
				ct.DerivationMethod = types.DerivationRestriction
			}

		case "key", "keyref", "unique":
			return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		default:
			return nil, fmt.Errorf("complexType: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if ct.Content() == nil {
		ct.SetContent(&types.EmptyContent{})
	}

	parsed, err := types.NewComplexTypeFromParsed(ct)
	if err != nil {
		return nil, fmt.Errorf("complexType: %w", err)
	}
	return parsed, nil
}

// resolveBaseTypeForComplex resolves a base type QName to a Type for complex types
// This is a simple resolution that works if the type is already available.
// Full two-phase resolution will be implemented in A7.
func resolveBaseTypeForComplex(schema *xsdschema.Schema, baseQName types.QName) types.Type {
	// check if it's a built-in type
	if builtinType := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); builtinType != nil {
		if baseQName.Local == "anyType" {
			// anyType is a complex type
			ct := &types.ComplexType{
				QName: baseQName,
			}
			ct.SetContent(&types.EmptyContent{})
			ct.SetMixed(false)
			return ct
		}
		// for simple types used as base in simpleContent, return BuiltinType directly
		return builtinType
	}

	// check if it's already in the schema
	if baseType, ok := schema.TypeDefs[baseQName]; ok {
		return baseType
	}

	// not found yet - will be resolved in two-phase resolution (A7)
	return nil
}

func parseModelGroup(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.ModelGroup, error) {
	var kind types.GroupKind
	switch doc.LocalName(elem) {
	case "sequence":
		kind = types.Sequence
	case "choice":
		kind = types.Choice
	case "all":
		kind = types.AllGroup
	default:
		return nil, fmt.Errorf("unknown model group: %s", doc.LocalName(elem))
	}

	var (
		idAttr        string
		minOccursAttr string
		maxOccursAttr string
		hasID         bool
		hasMinOccurs  bool
		hasMaxOccurs  bool
	)
	// per XSD 1.0, model groups (sequence, choice, all) only allow: id, minOccurs, maxOccurs
	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		if attr.NamespaceURI() == "" {
			if !validModelGroupAttributes[attrName] {
				return nil, fmt.Errorf("invalid attribute '%s' on <%s> (only id, minOccurs, maxOccurs allowed)", attrName, doc.LocalName(elem))
			}
			switch attrName {
			case "id":
				hasID = true
				idAttr = attr.Value()
			case "minOccurs":
				if !hasMinOccurs {
					hasMinOccurs = true
					minOccursAttr = attr.Value()
				}
				if minOccursAttr == "" {
					return nil, fmt.Errorf("%s: minOccurs attribute cannot be empty", doc.LocalName(elem))
				}
			case "maxOccurs":
				if !hasMaxOccurs {
					hasMaxOccurs = true
					maxOccursAttr = attr.Value()
				}
				if maxOccursAttr == "" {
					return nil, fmt.Errorf("%s: maxOccurs attribute cannot be empty", doc.LocalName(elem))
				}
			}
			continue
		}

		switch attrName {
		case "minOccurs":
			if !hasMinOccurs {
				hasMinOccurs = true
				minOccursAttr = attr.Value()
			}
		case "maxOccurs":
			if !hasMaxOccurs {
				hasMaxOccurs = true
				maxOccursAttr = attr.Value()
			}
		}
	}

	if hasID {
		if err := validateIDAttribute(idAttr, doc.LocalName(elem), schema); err != nil {
			return nil, err
		}
	}

	// for xs:all, enforce that minOccurs must be 0 or 1, and maxOccurs must be 1
	if kind == types.AllGroup {
		// check if minOccurs is explicitly set and not "0" or "1"
		if minOccursAttr != "" && minOccursAttr != "0" && minOccursAttr != "1" {
			return nil, fmt.Errorf("xs:all must have minOccurs='0' or '1' (got %s)", minOccursAttr)
		}

		// check if maxOccurs is explicitly set and not "1"
		if maxOccursAttr != "" && maxOccursAttr != "1" {
			return nil, fmt.Errorf("xs:all must have maxOccurs='1' (got %s)", maxOccursAttr)
		}
	}

	minOccurs, err := parseOccursValue("minOccurs", minOccursAttr, hasMinOccurs, 1)
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursValue("maxOccurs", maxOccursAttr, hasMaxOccurs, 1)
	if err != nil {
		return nil, err
	}
	mg := &types.ModelGroup{
		Kind:      kind,
		MinOccurs: minOccurs,
		MaxOccurs: maxOccurs,
	}

	// track annotation constraints: at most one, must be first
	// content model for model groups: (annotation?, (element | group | choice | sequence | any)*)
	// for xs:all specifically: (annotation?, element*)
	hasAnnotation := false
	hasNonAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("%s: at most one annotation is allowed", doc.LocalName(elem))
			}
			if hasNonAnnotation {
				return nil, fmt.Errorf("%s: annotation must appear before other elements", doc.LocalName(elem))
			}
			hasAnnotation = true

		case "element":
			hasNonAnnotation = true
			elemDecl, err := parseElement(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse element: %w", err)
			}
			mg.Particles = append(mg.Particles, elemDecl)

		case "sequence", "choice", "all":
			hasNonAnnotation = true
			// xs:all can only contain element declarations, not nested model groups
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain %s (only element declarations are allowed)", doc.LocalName(child))
			}
			nestedMG, err := parseModelGroup(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse nested model group: %w", err)
			}
			mg.Particles = append(mg.Particles, nestedMG)

		case "group":
			hasNonAnnotation = true
			// xs:all can only contain element declarations, not group references
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain group references (only element declarations are allowed)")
			}
			// reference to a named group - create placeholder for later resolution
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "group", schema); err != nil {
					return nil, err
				}
			}
			if err := validateOnlyAnnotationChildren(doc, child, "group"); err != nil {
				return nil, err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return nil, fmt.Errorf("group reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
			}
			minOccurs, err := parseOccursAttr(doc, child, "minOccurs", 1)
			if err != nil {
				return nil, err
			}
			maxOccurs, err := parseOccursAttr(doc, child, "maxOccurs", 1)
			if err != nil {
				return nil, err
			}
			// this allows forward references and references to groups in imported schemas
			groupRef := &types.GroupRef{
				RefQName:  refQName,
				MinOccurs: minOccurs,
				MaxOccurs: maxOccurs,
			}
			mg.Particles = append(mg.Particles, groupRef)

		case "any":
			hasNonAnnotation = true
			// xs:all can only contain element declarations, not any wildcards
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain any wildcards (only element declarations are allowed)")
			}
			anyElem, err := parseAnyElement(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse any element: %w", err)
			}
			mg.Particles = append(mg.Particles, anyElem)

		case "key", "keyref", "unique":
			return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))

		case "attribute", "attributeGroup", "anyAttribute":
			// attributes are not allowed inside model groups (sequence/choice/all)
			// they must be declared at the complexType level
			return nil, fmt.Errorf("%s cannot appear inside %s (attributes must be declared at complexType level, not inside content model groups)", doc.LocalName(child), doc.LocalName(elem))
		default:
			return nil, fmt.Errorf("%s: unexpected child element <%s>", doc.LocalName(elem), doc.LocalName(child))
		}
	}

	return mg, nil
}

func parseSimpleContent(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.SimpleContent, error) {
	sc := &types.SimpleContent{}

	// validate id attribute if present on simpleContent
	if doc.HasAttribute(elem, "id") {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "simpleContent", schema); err != nil {
			return nil, err
		}
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent: annotation must appear before restriction or extension")
			}
			if seenAnnotation {
				return nil, fmt.Errorf("simpleContent: at most one annotation is allowed")
			}
			seenAnnotation = true
			continue
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "restriction", schema); err != nil {
					return nil, err
				}
			}
			base := doc.GetAttribute(child, "base")
			if base == "" {
				return nil, fmt.Errorf("restriction missing base")
			}
			baseQName, err := resolveQName(doc, base, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			restriction := &types.Restriction{Base: baseQName}

			// check for nested simpleType (allowed in complex type restrictions with simpleContent)
			// if present, parse it and use it as the base for facets
			seenSimpleType := false
			seenAttributeLike := false
			seenFacet := false
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xml.XSDNamespace {
					continue
				}
				switch doc.LocalName(grandchild) {
				case "annotation":
					continue
				case "simpleType":
					if seenSimpleType || seenFacet || seenAttributeLike {
						return nil, fmt.Errorf("simpleContent restriction: simpleType must appear before facets and attributes")
					}
					seenSimpleType = true
				case "attribute", "attributeGroup", "anyAttribute":
					seenAttributeLike = true
				default:
					if validSimpleContentFacetElements[doc.LocalName(grandchild)] {
						if seenAttributeLike {
							return nil, fmt.Errorf("simpleContent restriction: facets must appear before attributes")
						}
						seenFacet = true
					}
				}
			}

			var nestedSimpleType *types.SimpleType
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) == xml.XSDNamespace && doc.LocalName(grandchild) == "simpleType" {
					nestedSimpleType, err = parseInlineSimpleType(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse nested simpleType: %w", err)
					}
					break
				}
			}

			restriction.SimpleType = nestedSimpleType

			// parse facets - pass nested simpleType if present, otherwise nil
			// facets apply to the nested simpleType if present, otherwise to the base type
			if err := parseFacetsWithAttributes(doc, child, restriction, nestedSimpleType, schema); err != nil {
				return nil, fmt.Errorf("parse facets: %w", err)
			}

			hasAnyAttribute := false
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xml.XSDNamespace {
					continue
				}

				switch doc.LocalName(grandchild) {
				case "annotation", "simpleType":
					continue
				case "attribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: anyAttribute must appear after all attributes")
					}
					attr, err := parseAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse attribute in restriction: %w", err)
					}
					restriction.Attributes = append(restriction.Attributes, attr)
				case "attributeGroup":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: anyAttribute must appear after all attributes")
					}
					if hasIDAttribute(doc, grandchild) {
						idAttr := doc.GetAttribute(grandchild, "id")
						if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
							return nil, err
						}
					}
					if err := validateOnlyAnnotationChildren(doc, grandchild, "attributeGroup"); err != nil {
						return nil, err
					}
					ref := doc.GetAttribute(grandchild, "ref")
					if ref == "" {
						return nil, fmt.Errorf("attributeGroup reference missing ref attribute")
					}
					refQName, err := resolveQName(doc, ref, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
					}
					restriction.AttrGroups = append(restriction.AttrGroups, refQName)
				case "anyAttribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: at most one anyAttribute is allowed")
					}
					hasAnyAttribute = true
					anyAttr, err := parseAnyAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse anyAttribute in restriction: %w", err)
					}
					restriction.AnyAttribute = anyAttr
				}
			}

			sc.Restriction = restriction

		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			// validate id attribute if present on extension
			if doc.HasAttribute(child, "id") {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "extension", schema); err != nil {
					return nil, err
				}
			}

			base := doc.GetAttribute(child, "base")
			if base == "" {
				return nil, fmt.Errorf("extension missing base")
			}
			baseQName, err := resolveQName(doc, base, child, schema)
			if err != nil {
				return nil, err
			}
			sc.Base = baseQName
			extension := &types.Extension{Base: baseQName}

			hasAnyAttribute := false
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) != xml.XSDNamespace {
					continue
				}

				switch doc.LocalName(grandchild) {
				case "annotation":
					continue
				case "attribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: anyAttribute must appear after all attributes")
					}
					attr, err := parseAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse attribute in extension: %w", err)
					}
					extension.Attributes = append(extension.Attributes, attr)
				case "attributeGroup":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: anyAttribute must appear after all attributes")
					}
					// reference to an attributeGroup in extension
					if hasIDAttribute(doc, grandchild) {
						idAttr := doc.GetAttribute(grandchild, "id")
						if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
							return nil, err
						}
					}
					if err := validateOnlyAnnotationChildren(doc, grandchild, "attributeGroup"); err != nil {
						return nil, err
					}
					ref := doc.GetAttribute(grandchild, "ref")
					if ref == "" {
						return nil, fmt.Errorf("attributeGroup reference missing ref attribute")
					}
					refQName, err := resolveQName(doc, ref, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
					}
					extension.AttrGroups = append(extension.AttrGroups, refQName)
				case "anyAttribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: at most one anyAttribute is allowed")
					}
					hasAnyAttribute = true
					anyAttr, err := parseAnyAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse anyAttribute in extension: %w", err)
					}
					extension.AnyAttribute = anyAttr
				default:
					return nil, fmt.Errorf("simpleContent extension has unexpected child element '%s'", doc.LocalName(grandchild))
				}
			}

			sc.Extension = extension
		default:
			return nil, fmt.Errorf("simpleContent has unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return nil, fmt.Errorf("simpleContent must have exactly one derivation child (restriction or extension)")
	}

	return sc, nil
}

func parseComplexContent(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.ComplexContent, error) {
	cc := &types.ComplexContent{}

	if doc.HasAttribute(elem, "id") {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "complexContent", schema); err != nil {
			return nil, err
		}
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		cc.Mixed = value
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenDerivation {
				return nil, fmt.Errorf("complexContent: annotation must appear before restriction or extension")
			}
			if seenAnnotation {
				return nil, fmt.Errorf("complexContent: at most one annotation is allowed")
			}
			seenAnnotation = true
			continue
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "restriction", schema); err != nil {
					return nil, err
				}
			}
			base := doc.GetAttribute(child, "base")
			if base == "" {
				return nil, fmt.Errorf("restriction missing base")
			}
			baseQName, err := resolveQName(doc, base, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
			restriction := &types.Restriction{Base: baseQName}

			// a restriction can have one top-level content model (sequence, choice, all, or element)
			// followed by attributes (attribute, attributeGroup, anyAttribute)
			// the order must be: particle first (if present), then attributes
			// attributes can exist without particles

			// collect all XSD namespace children
			var children []xml.NodeID
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) == xml.XSDNamespace {
					children = append(children, grandchild)
				}
			}

			// reject unexpected children (facets are not allowed in complexContent restrictions).
			for _, grandchild := range children {
				if !validComplexContentChildren[doc.LocalName(grandchild)] {
					return nil, fmt.Errorf("complexContent restriction has unexpected child element '%s'", doc.LocalName(grandchild))
				}
			}

			particleIndex := -1
			firstAttributeIndex := -1
			for i, grandchild := range children {
				name := doc.LocalName(grandchild)
				isParticle := name == "sequence" || name == "choice" || name == "all" ||
					name == "group" || name == "element" || name == "any"
				isAttribute := name == "attribute" || name == "attributeGroup" || name == "anyAttribute"

				if isParticle {
					if particleIndex == -1 {
						particleIndex = i
					} else {
						return nil, fmt.Errorf("ComplexContent restriction can only have one content model particle")
					}
				}
				if isAttribute && firstAttributeIndex == -1 {
					firstAttributeIndex = i
				}
			}

			// validate order: if both particle and attributes exist, particle must come first
			if particleIndex != -1 && firstAttributeIndex != -1 && firstAttributeIndex < particleIndex {
				return nil, fmt.Errorf("ComplexContent restriction: attributes must come after the content model particle")
			}

			if particleIndex != -1 {
				grandchild := children[particleIndex]
				grandchildName := doc.LocalName(grandchild)
				var particle types.Particle
				var err error

				switch grandchildName {
				case "sequence", "choice", "all":
					particle, err = parseModelGroup(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse model group in restriction: %w", err)
					}
				case "group":
					// reference to a named group - create placeholder for later resolution
					ref := doc.GetAttribute(grandchild, "ref")
					if ref == "" {
						return nil, fmt.Errorf("group reference missing ref attribute")
					}
					refQName, err := resolveQName(doc, ref, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
					}
					minOccurs, err := parseOccursAttr(doc, grandchild, "minOccurs", 1)
					if err != nil {
						return nil, err
					}
					maxOccurs, err := parseOccursAttr(doc, grandchild, "maxOccurs", 1)
					if err != nil {
						return nil, err
					}
					particle = &types.GroupRef{
						RefQName:  refQName,
						MinOccurs: minOccurs,
						MaxOccurs: maxOccurs,
					}
				case "element":
					// single element particle
					particle, err = parseElement(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse element in restriction: %w", err)
					}
				case "any":
					particle, err = parseAnyElement(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse any element in restriction: %w", err)
					}
				}
				restriction.Particle = particle
			}

			// parse attributes (can come after particle or without particle)
			hasAnyAttribute := false
			for _, grandchild := range children {
				grandchildName := doc.LocalName(grandchild)
				switch grandchildName {
				case "annotation":
					continue
				case "attribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: anyAttribute must appear after all attributes")
					}
					attr, err := parseAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse attribute in restriction: %w", err)
					}
					restriction.Attributes = append(restriction.Attributes, attr)
				case "attributeGroup":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: anyAttribute must appear after all attributes")
					}
					// reference to an attributeGroup in restriction
					if hasIDAttribute(doc, grandchild) {
						idAttr := doc.GetAttribute(grandchild, "id")
						if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
							return nil, err
						}
					}
					if err := validateOnlyAnnotationChildren(doc, grandchild, "attributeGroup"); err != nil {
						return nil, err
					}
					ref := doc.GetAttribute(grandchild, "ref")
					if ref == "" {
						return nil, fmt.Errorf("attributeGroup reference missing ref attribute")
					}
					refQName, err := resolveQName(doc, ref, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
					}
					restriction.AttrGroups = append(restriction.AttrGroups, refQName)
				case "anyAttribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("restriction: at most one anyAttribute is allowed")
					}
					hasAnyAttribute = true
					anyAttr, err := parseAnyAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse anyAttribute in restriction: %w", err)
					}
					restriction.AnyAttribute = anyAttr
				}
			}

			cc.Restriction = restriction

		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "extension", schema); err != nil {
					return nil, err
				}
			}
			base := doc.GetAttribute(child, "base")
			if base == "" {
				return nil, fmt.Errorf("extension missing base")
			}
			baseQName, err := resolveQName(doc, base, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
			extension := &types.Extension{Base: baseQName}

			// an extension can have one top-level content model (sequence, choice, all, or element)
			// followed by attributes (attribute, attributeGroup, anyAttribute)
			// the order must be: particle first (if present), then attributes
			// attributes can exist without particles

			var children []xml.NodeID
			for _, grandchild := range doc.Children(child) {
				if doc.NamespaceURI(grandchild) == xml.XSDNamespace {
					children = append(children, grandchild)
				}
			}

			for _, grandchild := range children {
				if !validComplexContentChildren[doc.LocalName(grandchild)] {
					return nil, fmt.Errorf("complexContent extension has unexpected child element '%s'", doc.LocalName(grandchild))
				}
			}

			particleIndex := -1
			firstAttributeIndex := -1
			for i, grandchild := range children {
				name := doc.LocalName(grandchild)
				isParticle := name == "sequence" || name == "choice" || name == "all" ||
					name == "group" || name == "element" || name == "any"
				isAttribute := name == "attribute" || name == "attributeGroup" || name == "anyAttribute"

				if isParticle {
					if particleIndex == -1 {
						particleIndex = i
					} else {
						return nil, fmt.Errorf("ComplexContent extension can only have one content model particle")
					}
				}
				if isAttribute && firstAttributeIndex == -1 {
					firstAttributeIndex = i
				}
			}

			if particleIndex != -1 && firstAttributeIndex != -1 && firstAttributeIndex < particleIndex {
				return nil, fmt.Errorf("ComplexContent extension: attributes must come after the content model particle")
			}

			particleFound := false
			hasAnyAttribute := false
			for _, grandchild := range children {

				grandchildName := doc.LocalName(grandchild)

				if !particleFound {
					var particle types.Particle
					var err error

					switch grandchildName {
					case "annotation":
						continue
					case "sequence", "choice", "all":
						particle, err = parseModelGroup(doc, grandchild, schema)
						if err != nil {
							return nil, fmt.Errorf("parse model group in extension: %w", err)
						}
					case "group":
						// reference to a named group - create placeholder for later resolution
						ref := doc.GetAttribute(grandchild, "ref")
						if ref == "" {
							return nil, fmt.Errorf("group reference missing ref attribute")
						}
						refQName, err := resolveQName(doc, ref, grandchild, schema)
						if err != nil {
							return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
						}
						minOccurs, err := parseOccursAttr(doc, grandchild, "minOccurs", 1)
						if err != nil {
							return nil, err
						}
						maxOccurs, err := parseOccursAttr(doc, grandchild, "maxOccurs", 1)
						if err != nil {
							return nil, err
						}
						particle = &types.GroupRef{
							RefQName:  refQName,
							MinOccurs: minOccurs,
							MaxOccurs: maxOccurs,
						}
					case "element":
						// single element particle
						particle, err = parseElement(doc, grandchild, schema)
						if err != nil {
							return nil, fmt.Errorf("parse element in extension: %w", err)
						}
					case "any":
						particle, err = parseAnyElement(doc, grandchild, schema)
						if err != nil {
							return nil, fmt.Errorf("parse any element in extension: %w", err)
						}
					case "attribute", "attributeGroup", "anyAttribute":
						// attributes can come after particle or without particle
						// continue to attribute parsing below
					default:
						continue
					}

					if particle != nil {
						extension.Particle = particle
						particleFound = true
						continue
					}
				}

				// parse attributes and anyAttribute (can come after particle or without particle)
				switch grandchildName {
				case "annotation":
					continue
				case "attribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: anyAttribute must appear after all attributes")
					}
					attr, err := parseAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse attribute in extension: %w", err)
					}
					extension.Attributes = append(extension.Attributes, attr)
				case "attributeGroup":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: anyAttribute must appear after all attributes")
					}
					// reference to an attributeGroup in extension
					if hasIDAttribute(doc, grandchild) {
						idAttr := doc.GetAttribute(grandchild, "id")
						if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
							return nil, err
						}
					}
					if err := validateOnlyAnnotationChildren(doc, grandchild, "attributeGroup"); err != nil {
						return nil, err
					}
					ref := doc.GetAttribute(grandchild, "ref")
					if ref == "" {
						return nil, fmt.Errorf("attributeGroup reference missing ref attribute")
					}
					refQName, err := resolveQName(doc, ref, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
					}
					extension.AttrGroups = append(extension.AttrGroups, refQName)
				case "anyAttribute":
					if hasAnyAttribute {
						return nil, fmt.Errorf("extension: at most one anyAttribute is allowed")
					}
					hasAnyAttribute = true
					anyAttr, err := parseAnyAttribute(doc, grandchild, schema)
					if err != nil {
						return nil, fmt.Errorf("parse anyAttribute in extension: %w", err)
					}
					extension.AnyAttribute = anyAttr
				}
			}

			cc.Extension = extension
		default:
			return nil, fmt.Errorf("complexContent has unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if !seenDerivation {
		return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
	}

	return cc, nil
}

// parseTopLevelGroup parses a top-level <group> definition
// Content model: (annotation?, (all | choice | sequence))
func parseTopLevelGroup(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) error {
	name := getAttr(doc, elem, "name")
	if name == "" {
		return fmt.Errorf("group missing name attribute")
	}

	// validate attributes - top-level group can only have: id, name
	// (ref, minOccurs, maxOccurs are only for group references)
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() != "" {
			continue // allow namespace-qualified attributes
		}
		attrName := attr.LocalName()
		if !validTopLevelGroupAttributes[attrName] {
			return fmt.Errorf("invalid attribute '%s' on top-level group (only id, name allowed)", attrName)
		}
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "group", schema); err != nil {
			return err
		}
	}

	qname := types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	if _, exists := schema.Groups[qname]; exists {
		return fmt.Errorf("duplicate group definition: '%s'", name)
	}

	// track annotation constraints: at most one, must be first
	hasAnnotation := false
	hasModelGroup := false
	var mg *types.ModelGroup

	// parse the group content (sequence, choice, or all)
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("group '%s': at most one annotation is allowed", name)
			}
			if hasModelGroup {
				return fmt.Errorf("group '%s': annotation must appear before model group", name)
			}
			hasAnnotation = true

		case "sequence", "choice", "all":
			if hasModelGroup {
				return fmt.Errorf("group '%s': exactly one model group (all, choice, or sequence) is allowed", name)
			}
			var err error
			mg, err = parseModelGroup(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse model group: %w", err)
			}
			hasModelGroup = true
		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		}
	}

	if mg == nil {
		return fmt.Errorf("group '%s' must contain exactly one model group (all, choice, or sequence)", name)
	}

	mg.SourceNamespace = schema.TargetNamespace
	schema.Groups[qname] = mg
	return nil
}

// parseTopLevelAttributeGroup parses a top-level <attributeGroup> definition
// Content model: (annotation?, ((attribute | attributeGroup)*, anyAttribute?))
func parseTopLevelAttributeGroup(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) error {
	name := getAttr(doc, elem, "name")
	if name == "" {
		return fmt.Errorf("attributeGroup missing name attribute")
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
			return err
		}
	}

	attrGroup := &types.AttributeGroup{
		Name: types.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		Attributes:      []*types.AttributeDecl{},
		AttrGroups:      []types.QName{},
		SourceNamespace: schema.TargetNamespace,
	}

	// track annotation constraints: at most one, must be first
	hasAnnotation := false
	hasNonAnnotation := false
	hasAnyAttribute := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("attributeGroup '%s': at most one annotation is allowed", name)
			}
			if hasNonAnnotation {
				return fmt.Errorf("attributeGroup '%s': annotation must appear before other elements", name)
			}
			hasAnnotation = true

		case "attribute":
			hasNonAnnotation = true
			attr, err := parseAttribute(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse attribute: %w", err)
			}
			attrGroup.Attributes = append(attrGroup.Attributes, attr)

		case "attributeGroup":
			hasNonAnnotation = true
			// reference to another attributeGroup
			if doc.HasAttribute(child, "name") {
				return fmt.Errorf("attributeGroup reference cannot have 'name' attribute")
			}
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
					return err
				}
			}
			if err := validateOnlyAnnotationChildren(doc, child, "attributeGroup"); err != nil {
				return err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return fmt.Errorf("attributeGroup reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
			}
			attrGroup.AttrGroups = append(attrGroup.AttrGroups, refQName)

		case "anyAttribute":
			hasNonAnnotation = true
			if hasAnyAttribute {
				return fmt.Errorf("attributeGroup '%s': at most one anyAttribute is allowed", name)
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse anyAttribute in attributeGroup: %w", err)
			}
			attrGroup.AnyAttribute = anyAttr

		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		default:
			return fmt.Errorf("invalid child element <%s> in <attributeGroup> declaration", doc.LocalName(child))
		}
	}

	qname := types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	if _, exists := schema.AttributeGroups[qname]; exists {
		return fmt.Errorf("attributeGroup %s already defined", qname)
	}
	schema.AttributeGroups[qname] = attrGroup
	return nil
}

// parseAnyElement parses an <any> wildcard element
// Content model: (annotation?)
func parseAnyElement(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.AnyElement, error) {
	// validate that <any> doesn't have invalid attributes
	// in XSD 1.0, <any> allows: namespace, processContents, minOccurs, maxOccurs, id
	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		// skip namespace declarations (xmlns attributes)
		if attrName == "xmlns" || strings.HasPrefix(attrName, "xmlns:") {
			continue
		}
		// check if it's an invalid attribute (not in validAttributes and not a namespace declaration)
		if attr.NamespaceURI() == "" && !validAnyElementAttributes[attrName] {
			// invalid attribute on <any> element
			return nil, fmt.Errorf("invalid attribute '%s' on <any> element (XSD 1.0 only allows: namespace, processContents, minOccurs, maxOccurs)", attrName)
		}
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "any", schema); err != nil {
			return nil, err
		}
	}

	// validate annotation constraints: at most one annotation, must be first
	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("any: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("any: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	minOccursAttr := doc.GetAttribute(elem, "minOccurs")
	maxOccursAttr := doc.GetAttribute(elem, "maxOccurs")
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "minOccurs" && attr.NamespaceURI() == "" && minOccursAttr == "" {
			return nil, fmt.Errorf("minOccurs attribute cannot be empty")
		}
		if attr.LocalName() == "maxOccurs" && attr.NamespaceURI() == "" && maxOccursAttr == "" {
			return nil, fmt.Errorf("maxOccurs attribute cannot be empty")
		}
	}
	if err := validateOccursValue(minOccursAttr); err != nil {
		return nil, fmt.Errorf("invalid minOccurs value '%s': %w", minOccursAttr, err)
	}
	if err := validateOccursValueAllowUnbounded(maxOccursAttr); err != nil {
		return nil, fmt.Errorf("invalid maxOccurs value '%s': %w", maxOccursAttr, err)
	}

	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs", 1)
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs", 1)
	if err != nil {
		return nil, err
	}
	anyElem := &types.AnyElement{
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
		ProcessContents: types.Strict,
		TargetNamespace: schema.TargetNamespace,
	}

	// per XSD spec: if namespace attribute is absent, default to ##any
	// if namespace="" (empty string), it means ##local (empty namespace only)
	// we need to distinguish between absent and empty string
	// note: HasAttribute returns false for empty string, so we check attributes directly
	namespaceAttr := doc.GetAttribute(elem, "namespace")
	hasNamespaceAttr := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "namespace" && attr.NamespaceURI() == "" {
			hasNamespaceAttr = true
			break
		}
	}
	if !hasNamespaceAttr {
		namespaceAttr = "##any" // default is ##any when attribute is absent
	} else if namespaceAttr == "" {
		// empty string means ##local (empty namespace only)
		namespaceAttr = "##local"
	}

	nsConstraint, nsList, err := parseNamespaceConstraint(namespaceAttr, schema)
	if err != nil {
		return nil, fmt.Errorf("parse namespace constraint: %w", err)
	}
	anyElem.Namespace = nsConstraint
	anyElem.NamespaceList = nsList

	processContents := doc.GetAttribute(elem, "processContents")
	// check if processContents is explicitly present but empty
	hasProcessContents := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "processContents" && attr.NamespaceURI() == "" {
			hasProcessContents = true
			break
		}
	}
	if hasProcessContents && processContents == "" {
		return nil, fmt.Errorf("processContents attribute cannot be empty")
	}

	switch processContents {
	case "strict":
		anyElem.ProcessContents = types.Strict
	case "lax":
		anyElem.ProcessContents = types.Lax
	case "skip":
		anyElem.ProcessContents = types.Skip
	case "":
		// absent - default to strict
		anyElem.ProcessContents = types.Strict
	default:
		return nil, fmt.Errorf("invalid processContents value '%s': must be 'strict', 'lax', or 'skip'", processContents)
	}

	return anyElem, nil
}

func validateOccursValue(value string) error {
	if value == "" {
		return nil
	}
	if value == "unbounded" {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return validateOccursInteger(value)
}

func validateOccursValueAllowUnbounded(value string) error {
	if value == "" || value == "unbounded" {
		return nil
	}
	return validateOccursInteger(value)
}

func validateOccursInteger(value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return nil
}

// parseAnyAttribute parses an <anyAttribute> wildcard
// Content model: (annotation?)
func parseAnyAttribute(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.AnyAttribute, error) {
	// reject XSD 1.1 features (notNamespace, notQName) - these are not supported in XSD 1.0
	if doc.GetAttribute(elem, "notNamespace") != "" {
		return nil, fmt.Errorf("notNamespace attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}
	if doc.GetAttribute(elem, "notQName") != "" {
		return nil, fmt.Errorf("notQName attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}

	// validate that <anyAttribute> doesn't have invalid attributes
	// in XSD 1.0, <anyAttribute> allows: namespace, processContents, id
	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		// skip namespace declarations (xmlns attributes)
		if attrName == "xmlns" || strings.HasPrefix(attrName, "xmlns:") {
			continue
		}
		if attr.NamespaceURI() == "" && !validAnyAttributeAttributes[attrName] {
			return nil, fmt.Errorf("invalid attribute '%s' on <anyAttribute> element (XSD 1.0 only allows: namespace, processContents)", attrName)
		}
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "anyAttribute", schema); err != nil {
			return nil, err
		}
	}

	// validate annotation constraints: at most one annotation, must be first
	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("anyAttribute: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("anyAttribute: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	anyAttr := &types.AnyAttribute{
		ProcessContents: types.Strict,
		TargetNamespace: schema.TargetNamespace,
	}

	// per XSD spec: if namespace attribute is absent, default to ##any
	// if namespace="" (empty string), it means ##local (empty namespace only)
	// we need to distinguish between absent and empty string
	// note: HasAttribute returns false for empty string, so we check attributes directly
	namespaceAttr := doc.GetAttribute(elem, "namespace")
	hasNamespaceAttr := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "namespace" && attr.NamespaceURI() == "" {
			hasNamespaceAttr = true
			break
		}
	}
	if !hasNamespaceAttr {
		namespaceAttr = "##any" // default is ##any when attribute is absent
	} else if namespaceAttr == "" {
		// empty string means ##local (empty namespace only)
		namespaceAttr = "##local"
	}

	nsConstraint, nsList, err := parseNamespaceConstraint(namespaceAttr, schema)
	if err != nil {
		return nil, fmt.Errorf("parse namespace constraint: %w", err)
	}
	anyAttr.Namespace = nsConstraint
	anyAttr.NamespaceList = nsList

	processContents := doc.GetAttribute(elem, "processContents")
	// check if processContents is explicitly present but empty
	hasProcessContents := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "processContents" && attr.NamespaceURI() == "" {
			hasProcessContents = true
			break
		}
	}
	if hasProcessContents && processContents == "" {
		return nil, fmt.Errorf("processContents attribute cannot be empty")
	}

	switch processContents {
	case "strict":
		anyAttr.ProcessContents = types.Strict
	case "lax":
		anyAttr.ProcessContents = types.Lax
	case "skip":
		anyAttr.ProcessContents = types.Skip
	case "":
		// absent - default to strict
		anyAttr.ProcessContents = types.Strict
	default:
		return nil, fmt.Errorf("invalid processContents value '%s': must be 'strict', 'lax', or 'skip'", processContents)
	}

	return anyAttr, nil
}

// parseNamespaceConstraint parses a namespace constraint value
// Returns: constraint type, namespace list (if applicable), error
// According to XSD spec (structures.xml 3.10.2.2):
// - If absent, defaults to ##any
// - If "##any", then any namespace
// - If "##other", then not(targetNamespace) - must be alone
// - Otherwise: space-delimited list where:
//   - Each substring is a namespace URI, OR
//   - "##targetNamespace" is replaced with the actual targetNamespace value
//   - "##local" is replaced with absent (empty namespace)
//   - "##any" and "##other" CANNOT appear in lists
func parseNamespaceConstraint(value string, schema *xsdschema.Schema) (types.NamespaceConstraint, []types.NamespaceURI, error) {
	// check for exact match of special tokens that must be alone
	switch value {
	case "##any":
		return types.NSCAny, nil, nil
	case "##other":
		return types.NSCOther, nil, nil
	case "##targetNamespace":
		return types.NSCTargetNamespace, nil, nil
	case "##local":
		return types.NSCLocal, nil, nil
	}

	// if not an exact match, it's a space-delimited list
	var resultList []types.NamespaceURI
	seen := false
	for ns := range strings.FieldsSeq(value) {
		seen = true
		// check if it's an invalid special token (starts with ## but not recognized)
		if strings.HasPrefix(ns, "##") && !validNamespaceConstraintTokens[ns] {
			if ns == "##any" || ns == "##other" {
				return 0, nil, fmt.Errorf("invalid namespace constraint: %s cannot appear in a namespace list (must be used alone)", ns)
			}
			// unknown ## token
			return 0, nil, fmt.Errorf("invalid namespace constraint: unknown special token %s (must be one of: ##any, ##other, ##targetNamespace, ##local)", ns)
		}

		// process the list: replace ##targetNamespace and ##local with their actual values
		switch ns {
		case "##targetNamespace":
			// replace with actual targetNamespace value (or empty string if absent)
			resultList = append(resultList, schema.TargetNamespace)
		case "##local":
			// replace with empty string (represents absent/empty namespace)
			resultList = append(resultList, types.NamespaceEmpty)
		default:
			// regular namespace URI
			resultList = append(resultList, types.NamespaceURI(ns))
		}
	}
	if !seen {
		return 0, nil, fmt.Errorf("invalid namespace constraint: empty namespace list")
	}

	return types.NSCList, resultList, nil
}

// parseIdentityConstraint parses a key, keyref, or unique constraint
func parseIdentityConstraint(doc *xml.Document, elem xml.NodeID, schema *xsdschema.Schema) (*types.IdentityConstraint, error) {
	name := getAttr(doc, elem, "name")
	if name == "" {
		return nil, fmt.Errorf("identity constraint missing name attribute")
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, doc.LocalName(elem), schema); err != nil {
			return nil, err
		}
	}

	nsContext := namespaceContextForElement(doc, elem, schema)

	constraint := &types.IdentityConstraint{
		Name:             name,
		TargetNamespace:  schema.TargetNamespace,
		Fields:           []types.Field{},
		NamespaceContext: nsContext,
	}

	switch doc.LocalName(elem) {
	case "key":
		constraint.Type = types.KeyConstraint
	case "keyref":
		constraint.Type = types.KeyRefConstraint
	case "unique":
		constraint.Type = types.UniqueConstraint
	default:
		return nil, fmt.Errorf("unknown identity constraint type: %s", doc.LocalName(elem))
	}

	// read refer attribute for all constraint types (to detect invalid use on key/unique)
	refer := doc.GetAttribute(elem, "refer")
	if refer != "" {
		// for keyref, refer is required and must be resolved.
		// for key/unique, refer is invalid but we store it for validation.
		referQName, err := resolveIdentityConstraintQName(doc, refer, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve refer QName %s: %w", refer, err)
		}
		constraint.ReferQName = referQName
	} else if constraint.Type == types.KeyRefConstraint {
		// keyref requires refer attribute
		return nil, fmt.Errorf("keyref missing refer attribute")
	}

	// XSD spec: (annotation?, selector, field+)
	// annotation must come first (if present), only one allowed
	annotationCount := 0
	seenSelector := false
	seenField := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenSelector || seenField {
				return nil, fmt.Errorf("identity constraint '%s': annotation must appear before selector and field", name)
			}
			annotationCount++
			if annotationCount > 1 {
				return nil, fmt.Errorf("identity constraint '%s': at most one annotation allowed", name)
			}

		case "selector":
			// per XSD spec, only one selector is allowed
			if seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': only one selector allowed", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("selector missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "selector", validIdentityConstraintAttributes); err != nil {
				return nil, err
			}
			if err := validateOnlyAnnotationChildren(doc, child, "selector"); err != nil {
				return nil, err
			}
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "selector", schema); err != nil {
					return nil, err
				}
			}
			constraint.Selector = types.Selector{XPath: xpath}
			seenSelector = true

		case "field":
			// per XSD spec, selector must appear before field
			if !seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': selector must appear before field", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("field missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "field", validIdentityConstraintAttributes); err != nil {
				return nil, err
			}
			if err := validateOnlyAnnotationChildren(doc, child, "field"); err != nil {
				return nil, err
			}
			if hasIDAttribute(doc, child) {
				idAttr := doc.GetAttribute(child, "id")
				if err := validateIDAttribute(idAttr, "field", schema); err != nil {
					return nil, err
				}
			}
			constraint.Fields = append(constraint.Fields, types.Field{XPath: xpath})
			seenField = true
		}
	}

	if constraint.Selector.XPath == "" {
		return nil, fmt.Errorf("identity constraint missing selector")
	}

	if len(constraint.Fields) == 0 {
		return nil, fmt.Errorf("identity constraint missing fields")
	}

	return constraint, nil
}

func validateAllowedAttributes(doc *xml.Document, elem xml.NodeID, elementName string, allowed map[string]bool) error {
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == xml.XMLNSNamespace || attr.LocalName() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() != "" {
			if attr.NamespaceURI() == xml.XSDNamespace {
				return fmt.Errorf("%s: attribute '%s' must be unprefixed", elementName, attr.LocalName())
			}
			continue
		}
		if !allowed[attr.LocalName()] {
			return fmt.Errorf("%s: unexpected attribute '%s'", elementName, attr.LocalName())
		}
	}
	return nil
}

// parseDerivationSetWithValidation parses and validates a derivation set.
// Returns an error if any token is not a valid derivation method.
// Per XSD spec, #all cannot be combined with other values.
func parseDerivationSetWithValidation(value string, allowed types.DerivationSet) (types.DerivationSet, error) {
	var set types.DerivationSet
	hasAll := false
	for token := range strings.FieldsSeq(value) {
		if hasAll {
			return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
		}
		switch token {
		case "extension":
			if !allowed.Has(types.DerivationExtension) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationExtension)
		case "restriction":
			if !allowed.Has(types.DerivationRestriction) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationRestriction)
		case "list":
			if !allowed.Has(types.DerivationList) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationList)
		case "union":
			if !allowed.Has(types.DerivationUnion) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationUnion)
		case "substitution":
			if !allowed.Has(types.DerivationSubstitution) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationSubstitution)
		case "#all":
			if set != 0 {
				return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
			}
			set = allowed
			hasAll = true
		default:
			return set, fmt.Errorf("invalid derivation method '%s'", token)
		}
	}
	return set, nil
}
