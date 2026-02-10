package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func collectXSDChildren(doc *schemaxml.Document, elem schemaxml.NodeID) []schemaxml.NodeID {
	var children []schemaxml.NodeID
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == schemaxml.XSDNamespace {
			children = append(children, child)
		}
	}
	return children
}

func validateComplexContentChildren(doc *schemaxml.Document, children []schemaxml.NodeID, context string) error {
	for _, child := range children {
		if !validChildElementNames[childSetComplexContentChild][doc.LocalName(child)] {
			return fmt.Errorf("complexContent %s has unexpected child element '%s'", context, doc.LocalName(child))
		}
	}
	return nil
}

func findComplexContentParticleIndex(doc *schemaxml.Document, children []schemaxml.NodeID, context string) (int, error) {
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

func parseComplexContentParticle(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, context string) (model.Particle, error) {
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
		refQName, err := resolveQNameWithPolicy(doc, ref, elem, schema, useDefaultNamespace)
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
		return &model.GroupRef{RefQName: refQName, MinOccurs: minOccurs, MaxOccurs: maxOccurs}, nil
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
