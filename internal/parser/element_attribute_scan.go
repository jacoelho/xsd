package parser

import "github.com/jacoelho/xsd/internal/xmltree"

type elementAttrScan struct {
	defaultVal       string
	ref              string
	name             string
	typ              string
	minOccurs        string
	maxOccurs        string
	invalidRefAttr   string
	fixedVal         string
	nillable         string
	block            string
	form             string
	invalidLocalAttr string
	id               string
	hasRef           bool
	hasType          bool
	hasMinOccurs     bool
	hasMaxOccurs     bool
	hasDefault       bool
	hasFixed         bool
	hasNillable      bool
	hasBlock         bool
	hasForm          bool
	hasAbstract      bool
	hasFinal         bool
	hasName          bool
	hasID            bool
}

func scanElementAttributes(doc *xmltree.Document, elem xmltree.NodeID) elementAttrScan {
	var attrs elementAttrScan
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == xmltree.XSDNamespace {
			if attrs.invalidRefAttr == "" {
				attrs.invalidRefAttr = attr.LocalName()
			}
			if attrs.invalidLocalAttr == "" {
				attrs.invalidLocalAttr = attr.LocalName()
			}
			continue
		}
		if attr.NamespaceURI() != "" {
			continue
		}
		attrName := attr.LocalName()
		switch attrName {
		case "id":
			attrs.hasID = true
			attrs.id = attr.Value()
		case "ref":
			if !attrs.hasRef {
				attrs.hasRef = true
				attrs.ref = attr.Value()
			}
		case "name":
			if !attrs.hasName {
				attrs.hasName = true
				attrs.name = attr.Value()
			}
		case "type":
			if !attrs.hasType {
				attrs.hasType = true
				attrs.typ = attr.Value()
			}
		case "minOccurs":
			if !attrs.hasMinOccurs {
				attrs.hasMinOccurs = true
				attrs.minOccurs = attr.Value()
			}
		case "maxOccurs":
			if !attrs.hasMaxOccurs {
				attrs.hasMaxOccurs = true
				attrs.maxOccurs = attr.Value()
			}
		case "default":
			if !attrs.hasDefault {
				attrs.hasDefault = true
				attrs.defaultVal = attr.Value()
			}
		case "fixed":
			if !attrs.hasFixed {
				attrs.hasFixed = true
				attrs.fixedVal = attr.Value()
			}
		case "nillable":
			if !attrs.hasNillable {
				attrs.hasNillable = true
				attrs.nillable = attr.Value()
			}
		case "block":
			if !attrs.hasBlock {
				attrs.hasBlock = true
				attrs.block = attr.Value()
			}
		case "form":
			if !attrs.hasForm {
				attrs.hasForm = true
				attrs.form = attr.Value()
			}
		case "abstract":
			attrs.hasAbstract = true
		case "final":
			attrs.hasFinal = true
		}

		if attr.NamespaceURI() != "" {
			continue
		}
		if attrs.invalidRefAttr == "" && !elementReferenceAttributeProfile.allows(attrName) {
			attrs.invalidRefAttr = attrName
		}
		if attrs.invalidLocalAttr == "" && !localElementAttributeProfile.allows(attrName) {
			attrs.invalidLocalAttr = attrName
		}
	}
	return attrs
}
