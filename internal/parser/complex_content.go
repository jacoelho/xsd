package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseComplexContent(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.ComplexContent, error) {
	cc := &types.ComplexContent{}

	if err := validateOptionalID(doc, elem, "complexContent", schema); err != nil {
		return nil, err
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		cc.Mixed = value
		cc.MixedSpecified = true
	}

	seenDerivation := false
	seenAnnotation := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
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
		case "restriction":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			restriction, baseQName, err := parseComplexContentRestriction(doc, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
			cc.Restriction = restriction
		case "extension":
			if err := validateAnnotationOrder(doc, child); err != nil {
				return nil, err
			}
			if seenDerivation {
				return nil, fmt.Errorf("complexContent must have exactly one derivation child (restriction or extension)")
			}
			seenDerivation = true
			extension, baseQName, err := parseComplexContentExtension(doc, child, schema)
			if err != nil {
				return nil, err
			}
			cc.Base = baseQName
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

func parseComplexContentRestriction(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.Restriction, types.QName, error) {
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, types.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, types.QName{}, fmt.Errorf("restriction missing base")
	}
	baseQName, err := resolveQName(doc, base, elem, schema)
	if err != nil {
		return nil, types.QName{}, err
	}
	restriction := &types.Restriction{Base: baseQName}

	children := collectXSDChildren(doc, elem)
	err = validateComplexContentChildren(doc, children, "restriction")
	if err != nil {
		return nil, baseQName, err
	}
	particleIndex, err := findComplexContentParticleIndex(doc, children, "restriction")
	if err != nil {
		return nil, baseQName, err
	}
	if particleIndex != -1 {
		var particle types.Particle
		particle, err = parseComplexContentParticle(doc, children[particleIndex], schema, "restriction")
		if err != nil {
			return nil, baseQName, err
		}
		restriction.Particle = particle
	}

	uses, err := parseAttributeUses(doc, children, schema, "restriction")
	if err != nil {
		return nil, baseQName, err
	}
	restriction.Attributes = uses.attributes
	restriction.AttrGroups = uses.attrGroups
	restriction.AnyAttribute = uses.anyAttribute

	return restriction, baseQName, nil
}

func parseComplexContentExtension(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.Extension, types.QName, error) {
	if err := validateOptionalID(doc, elem, "extension", schema); err != nil {
		return nil, types.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, types.QName{}, fmt.Errorf("extension missing base")
	}
	baseQName, err := resolveQName(doc, base, elem, schema)
	if err != nil {
		return nil, types.QName{}, err
	}
	extension := &types.Extension{Base: baseQName}

	children := collectXSDChildren(doc, elem)
	err = validateComplexContentChildren(doc, children, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	particleIndex, err := findComplexContentParticleIndex(doc, children, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	if particleIndex != -1 {
		var particle types.Particle
		particle, err = parseComplexContentParticle(doc, children[particleIndex], schema, "extension")
		if err != nil {
			return nil, baseQName, err
		}
		extension.Particle = particle
	}

	uses, err := parseAttributeUses(doc, children, schema, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	extension.Attributes = uses.attributes
	extension.AttrGroups = uses.attrGroups
	extension.AnyAttribute = uses.anyAttribute

	return extension, baseQName, nil
}

func collectXSDChildren(doc *xsdxml.Document, elem xsdxml.NodeID) []xsdxml.NodeID {
	var children []xsdxml.NodeID
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace {
			children = append(children, child)
		}
	}
	return children
}

func validateComplexContentChildren(doc *xsdxml.Document, children []xsdxml.NodeID, context string) error {
	for _, child := range children {
		if !validChildElementNames[childSetComplexContentChild][doc.LocalName(child)] {
			return fmt.Errorf("complexContent %s has unexpected child element '%s'", context, doc.LocalName(child))
		}
	}
	return nil
}

func findComplexContentParticleIndex(doc *xsdxml.Document, children []xsdxml.NodeID, context string) (int, error) {
	particleIndex := -1
	firstAttributeIndex := -1

	for i, child := range children {
		name := doc.LocalName(child)
		if isComplexContentParticle(name) {
			if particleIndex != -1 {
				return -1, fmt.Errorf("ComplexContent %s can only have one content model particle", context)
			}
			particleIndex = i
		}
		if isComplexContentAttribute(name) && firstAttributeIndex == -1 {
			firstAttributeIndex = i
		}
	}

	if particleIndex != -1 && firstAttributeIndex != -1 && firstAttributeIndex < particleIndex {
		return -1, fmt.Errorf("ComplexContent %s: attributes must come after the content model particle", context)
	}

	return particleIndex, nil
}

func parseComplexContentParticle(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, context string) (types.Particle, error) {
	switch doc.LocalName(elem) {
	case "sequence", "choice", "all":
		particle, err := parseModelGroup(doc, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("parse model group in %s: %w", context, err)
		}
		return particle, nil
	case "group":
		ref := doc.GetAttribute(elem, "ref")
		if ref == "" {
			return nil, fmt.Errorf("group reference missing ref attribute")
		}
		refQName, err := resolveQName(doc, ref, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
		}
		minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
		if err != nil {
			return nil, err
		}
		maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
		if err != nil {
			return nil, err
		}
		return &types.GroupRef{
			RefQName:  refQName,
			MinOccurs: minOccurs,
			MaxOccurs: maxOccurs,
		}, nil
	case "element":
		particle, err := parseElement(doc, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("parse element in %s: %w", context, err)
		}
		return particle, nil
	case "any":
		particle, err := parseAnyElement(doc, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("parse any element in %s: %w", context, err)
		}
		return particle, nil
	default:
		return nil, fmt.Errorf("complexContent %s has unexpected child element '%s'", context, doc.LocalName(elem))
	}
}

func isComplexContentParticle(name string) bool {
	switch name {
	case "sequence", "choice", "all", "group", "element", "any":
		return true
	default:
		return false
	}
}

func isComplexContentAttribute(name string) bool {
	switch name {
	case "attribute", "attributeGroup", "anyAttribute":
		return true
	default:
		return false
	}
}
