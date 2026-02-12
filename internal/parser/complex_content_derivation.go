package parser

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseComplexContentRestriction(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.Restriction, model.QName, error) {
	baseQName, err := parseDerivationBaseQName(doc, elem, schema, "restriction")
	if err != nil {
		return nil, model.QName{}, err
	}
	restriction := &model.Restriction{Base: baseQName}

	parsed, err := parseComplexDerivationBody(doc, elem, schema, "restriction")
	if err != nil {
		return nil, baseQName, err
	}
	restriction.Particle = parsed.particle
	restriction.Attributes = parsed.uses.attributes
	restriction.AttrGroups = parsed.uses.attrGroups
	restriction.AnyAttribute = parsed.uses.anyAttribute

	return restriction, baseQName, nil
}

func parseComplexContentExtension(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) (*model.Extension, model.QName, error) {
	baseQName, err := parseDerivationBaseQName(doc, elem, schema, "extension")
	if err != nil {
		return nil, model.QName{}, err
	}
	extension := &model.Extension{Base: baseQName}

	parsed, err := parseComplexDerivationBody(doc, elem, schema, "extension")
	if err != nil {
		return nil, baseQName, err
	}
	extension.Particle = parsed.particle
	extension.Attributes = parsed.uses.attributes
	extension.AttrGroups = parsed.uses.attrGroups
	extension.AnyAttribute = parsed.uses.anyAttribute

	return extension, baseQName, nil
}

type parsedComplexDerivationBody struct {
	particle model.Particle
	uses     attributeUses
}

func parseComplexDerivationBody(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, context string) (parsedComplexDerivationBody, error) {
	parsed := parsedComplexDerivationBody{}
	children := collectXSDChildren(doc, elem)
	err := validateComplexContentChildren(doc, children, context)
	if err != nil {
		return parsed, err
	}
	particleIndex, err := findComplexContentParticleIndex(doc, children, context)
	if err != nil {
		return parsed, err
	}
	if particleIndex != -1 {
		parsed.particle, err = parseComplexContentParticle(doc, children[particleIndex], schema, context)
		if err != nil {
			return parsed, err
		}
	}

	parsed.uses, err = parseAttributeUses(doc, children, schema, context)
	if err != nil {
		return parsed, err
	}
	return parsed, nil
}
