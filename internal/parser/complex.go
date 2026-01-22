package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

const (
	attrSetAnyElement         = "any"
	attrSetAnyAttribute       = "anyAttribute"
	attrSetModelGroup         = "modelGroup"
	attrSetTopLevelGroup      = "topLevelGroup"
	attrSetIdentityConstraint = "identityConstraint"

	childSetSimpleContentFacet  = "simpleContentFacet"
	childSetComplexContentChild = "complexContentChild"
)

var (
	validAttributeNames = map[string]map[string]bool{
		attrSetAnyElement: {
			"namespace":       true,
			"processContents": true,
			"minOccurs":       true,
			"maxOccurs":       true,
			"id":              true,
		},
		attrSetAnyAttribute: {
			"namespace":       true,
			"processContents": true,
			"id":              true,
		},
		attrSetModelGroup: {
			"id":        true,
			"minOccurs": true,
			"maxOccurs": true,
		},
		attrSetTopLevelGroup: {
			"id":   true,
			"name": true,
		},
		attrSetIdentityConstraint: {
			"xpath": true,
			"id":    true,
		},
	}
	validChildElementNames = map[string]map[string]bool{
		childSetSimpleContentFacet: {
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
		},
		childSetComplexContentChild: {
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
		},
	}
	validNamespaceConstraintTokens = map[string]bool{
		"##targetNamespace": true,
		"##local":           true,
	}
)

// parseComplexType parses a top-level complexType definition
func parseComplexType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
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

type complexTypeParseState struct {
	doc    *xsdxml.Document
	schema *Schema
	ct     *types.ComplexType

	hasAnnotation     bool
	hasNonAnnotation  bool
	hasAnyAttribute   bool
	hasParticle       bool
	hasSimpleContent  bool
	hasComplexContent bool
	hasAttributeLike  bool
}

func (s *complexTypeParseState) handleChild(child xsdxml.NodeID) error {
	switch s.doc.LocalName(child) {
	case "annotation":
		return s.handleAnnotation()
	case "sequence", "choice", "all":
		return s.handleModelGroup(child)
	case "any":
		return s.handleAny(child)
	case "group":
		return s.handleGroupRef(child)
	case "attribute":
		return s.handleAttribute(child)
	case "attributeGroup":
		return s.handleAttributeGroup(child)
	case "anyAttribute":
		return s.handleAnyAttribute(child)
	case "simpleContent":
		return s.handleSimpleContent(child)
	case "complexContent":
		return s.handleComplexContent(child)
	case "key", "keyref", "unique":
		return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", s.doc.LocalName(child))
	default:
		return fmt.Errorf("complexType: unexpected child element '%s'", s.doc.LocalName(child))
	}
}

func (s *complexTypeParseState) handleAnnotation() error {
	if s.hasAnnotation {
		return fmt.Errorf("complexType: at most one annotation is allowed")
	}
	if s.hasNonAnnotation {
		return fmt.Errorf("complexType: annotation must appear before other elements")
	}
	s.hasAnnotation = true
	return nil
}

func (s *complexTypeParseState) handleModelGroup(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
	}
	if s.hasAttributeLike {
		return fmt.Errorf("complexType: content model must appear before attributes")
	}
	if s.hasParticle {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasParticle = true
	mg, err := parseModelGroup(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse model group: %w", err)
	}
	s.ct.SetContent(&types.ElementContent{Particle: mg})
	return nil
}

func (s *complexTypeParseState) handleAny(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
	}
	if s.hasAttributeLike {
		return fmt.Errorf("complexType: content model must appear before attributes")
	}
	if s.hasParticle {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasParticle = true
	anyElem, err := parseAnyElement(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse any element: %w", err)
	}
	s.ct.SetContent(&types.ElementContent{Particle: anyElem})
	return nil
}

func (s *complexTypeParseState) handleGroupRef(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
	}
	if s.hasAttributeLike {
		return fmt.Errorf("complexType: content model must appear before attributes")
	}
	if s.hasParticle {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasParticle = true
	if err := validateElementConstraints(s.doc, child, "group", s.schema); err != nil {
		return err
	}
	ref := s.doc.GetAttribute(child, "ref")
	if ref == "" {
		return fmt.Errorf("group reference missing ref attribute")
	}
	refQName, err := resolveQName(s.doc, ref, child, s.schema)
	if err != nil {
		return fmt.Errorf("resolve group ref %s: %w", ref, err)
	}
	minOccurs, err := parseOccursAttr(s.doc, child, "minOccurs")
	if err != nil {
		return err
	}
	maxOccurs, err := parseOccursAttr(s.doc, child, "maxOccurs")
	if err != nil {
		return err
	}
	groupRef := &types.GroupRef{
		RefQName:  refQName,
		MinOccurs: minOccurs,
		MaxOccurs: maxOccurs,
	}
	s.ct.SetContent(&types.ElementContent{Particle: groupRef})
	return nil
}

func (s *complexTypeParseState) handleAttribute(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	s.hasAttributeLike = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if s.hasAnyAttribute {
		return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
	}
	attr, err := parseAttribute(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("complexType: parse attribute: %w", err)
	}
	s.ct.SetAttributes(append(s.ct.Attributes(), attr))
	return nil
}

func (s *complexTypeParseState) handleAttributeGroup(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	s.hasAttributeLike = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if s.hasAnyAttribute {
		return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
	}
	if err := validateElementConstraints(s.doc, child, "attributeGroup", s.schema); err != nil {
		return err
	}
	ref := s.doc.GetAttribute(child, "ref")
	if ref == "" {
		return fmt.Errorf("attributeGroup reference missing ref attribute")
	}
	refQName, err := resolveQName(s.doc, ref, child, s.schema)
	if err != nil {
		return fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
	}
	s.ct.AttrGroups = append(s.ct.AttrGroups, refQName)
	return nil
}

func (s *complexTypeParseState) handleAnyAttribute(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	s.hasAttributeLike = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if s.hasAnyAttribute {
		return fmt.Errorf("complexType: at most one anyAttribute is allowed")
	}
	s.hasAnyAttribute = true
	anyAttr, err := parseAnyAttribute(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse anyAttribute: %w", err)
	}
	s.ct.SetAnyAttribute(anyAttr)
	return nil
}

func (s *complexTypeParseState) handleSimpleContent(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	if s.hasParticle || s.hasAttributeLike {
		return fmt.Errorf("complexType: simpleContent must be the only content model")
	}
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasSimpleContent = true
	sc, err := parseSimpleContent(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse simpleContent: %w", err)
	}
	s.ct.SetContent(sc)
	// set base type if available (fully resolved during schema resolution)
	if !sc.Base.IsZero() {
		s.ct.ResolvedBase = resolveBaseTypeForComplex(s.schema, sc.Base)
	}
	if sc.Extension != nil {
		s.ct.DerivationMethod = types.DerivationExtension
	} else if sc.Restriction != nil {
		s.ct.DerivationMethod = types.DerivationRestriction
	}
	return nil
}

func (s *complexTypeParseState) handleComplexContent(child xsdxml.NodeID) error {
	s.hasNonAnnotation = true
	if s.hasParticle || s.hasAttributeLike {
		return fmt.Errorf("complexType: complexContent must be the only content model")
	}
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasComplexContent = true
	cc, err := parseComplexContent(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse complexContent: %w", err)
	}
	s.ct.SetContent(cc)
	// set base type if available (fully resolved during schema resolution)
	if !cc.Base.IsZero() {
		s.ct.ResolvedBase = resolveBaseTypeForComplex(s.schema, cc.Base)
	}
	if cc.Extension != nil {
		s.ct.DerivationMethod = types.DerivationExtension
	} else if cc.Restriction != nil {
		s.ct.DerivationMethod = types.DerivationRestriction
	}
	return nil
}

// parseInlineComplexType parses a complexType definition (inline or named)
func parseInlineComplexType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.ComplexType, error) {
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
	state := complexTypeParseState{
		doc:    doc,
		schema: schema,
		ct:     ct,
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		if err := state.handleChild(child); err != nil {
			return nil, err
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

// resolveBaseTypeForComplex resolves a base type QName to a Type for complex types.
// This is a simple resolution that works when the type is already available.
func resolveBaseTypeForComplex(schema *Schema, baseQName types.QName) types.Type {
	// check if it's a built-in type
	if builtinType := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); builtinType != nil {
		// for simple types used as base in simpleContent, return BuiltinType directly
		return builtinType
	}

	// check if it's already in the schema
	if baseType, ok := schema.TypeDefs[baseQName]; ok {
		return baseType
	}

	// not found yet - will be resolved during schema resolution
	return nil
}
