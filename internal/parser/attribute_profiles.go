package parser

import (
	"errors"

	"github.com/jacoelho/xsd/internal/xmltree"
)

type attributeProfile struct {
	allowed map[string]bool
}

func newAttributeProfile(names ...string) attributeProfile {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return attributeProfile{allowed: allowed}
}

func (p attributeProfile) allows(name string) bool {
	return p.allowed[name]
}

type elementAttrConflictRule struct {
	present func(*elementAttrScan) bool
	message string
}

type attributeConflictRule struct {
	name    string
	message string
}

var (
	topLevelElementAttributeProfile = newAttributeProfile(
		"id",
		"name",
		"type",
		"default",
		"fixed",
		"nillable",
		"abstract",
		"block",
		"final",
		"substitutionGroup",
	)

	elementReferenceAttributeProfile = newAttributeProfile(
		"id",
		"ref",
		"minOccurs",
		"maxOccurs",
	)

	localElementAttributeProfile = newAttributeProfile(
		"id",
		"name",
		"type",
		"minOccurs",
		"maxOccurs",
		"default",
		"fixed",
		"nillable",
		"block",
		"form",
		"ref",
	)

	attributeDeclarationProfile = newAttributeProfile(
		"name",
		"ref",
		"type",
		"use",
		"default",
		"fixed",
		"form",
		"id",
	)

	elementReferenceConflictRules = []elementAttrConflictRule{
		{present: func(attrs *elementAttrScan) bool { return attrs.hasType }, message: "element reference cannot have 'type' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasDefault }, message: "element reference cannot have 'default' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasFixed }, message: "element reference cannot have 'fixed' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasNillable }, message: "element reference cannot have 'nillable' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasBlock }, message: "element reference cannot have 'block' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasFinal }, message: "element reference cannot have 'final' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasForm }, message: "element reference cannot have 'form' attribute"},
		{present: func(attrs *elementAttrScan) bool { return attrs.hasAbstract }, message: "element reference cannot have 'abstract' attribute"},
	}

	attributeReferenceConflictRules = []attributeConflictRule{
		{name: "type", message: "attribute reference cannot have 'type' attribute"},
		{name: "form", message: "attribute reference cannot have 'form' attribute"},
	}
)

func validateElementReferenceConflicts(attrs *elementAttrScan) error {
	for _, rule := range elementReferenceConflictRules {
		if rule.present(attrs) {
			return errors.New(rule.message)
		}
	}
	return nil
}

func validateAttributeConflicts(doc *xmltree.Document, elem xmltree.NodeID, rules []attributeConflictRule) error {
	for _, rule := range rules {
		if doc.HasAttribute(elem, rule.name) {
			return errors.New(rule.message)
		}
	}
	return nil
}
