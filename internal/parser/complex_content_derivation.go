package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseComplexContentRestriction(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.Restriction, model.QName, error) {
	if err := validateOptionalID(doc, elem, "restriction", schema); err != nil {
		return nil, model.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, model.QName{}, fmt.Errorf("restriction missing base")
	}
	baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
	if err != nil {
		return nil, model.QName{}, err
	}
	restriction := &model.Restriction{Base: baseQName}

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
		var particle model.Particle
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

func parseComplexContentExtension(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.Extension, model.QName, error) {
	if err := validateOptionalID(doc, elem, "extension", schema); err != nil {
		return nil, model.QName{}, err
	}
	base := doc.GetAttribute(elem, "base")
	if base == "" {
		return nil, model.QName{}, fmt.Errorf("extension missing base")
	}
	baseQName, err := resolveQNameWithPolicy(doc, base, elem, schema, useDefaultNamespace)
	if err != nil {
		return nil, model.QName{}, err
	}
	extension := &model.Extension{Base: baseQName}

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
		var particle model.Particle
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
