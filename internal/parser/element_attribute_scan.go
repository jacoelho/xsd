package parser

import (
	"github.com/jacoelho/xsd/internal/value"
)

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

func scanElementAttributes(doc *Document, elem NodeID) elementAttrScan {
	var attrs elementAttrScan
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == value.XSDNamespace {
			recordInvalidElementAttribute(&attrs, attr.LocalName())
			continue
		}
		if attr.NamespaceURI() != "" {
			continue
		}
		recordElementAttribute(&attrs, attr.LocalName(), attr.Value())
	}
	return attrs
}

func recordInvalidElementAttribute(attrs *elementAttrScan, name string) {
	if attrs.invalidRefAttr == "" {
		attrs.invalidRefAttr = name
	}
	if attrs.invalidLocalAttr == "" {
		attrs.invalidLocalAttr = name
	}
}

func recordElementAttribute(attrs *elementAttrScan, name, value string) {
	switch name {
	case "id":
		attrs.hasID = true
		attrs.id = value
	case "ref":
		setElementAttributeValue(&attrs.hasRef, &attrs.ref, value)
	case "name":
		setElementAttributeValue(&attrs.hasName, &attrs.name, value)
	case "type":
		setElementAttributeValue(&attrs.hasType, &attrs.typ, value)
	case "minOccurs":
		setElementAttributeValue(&attrs.hasMinOccurs, &attrs.minOccurs, value)
	case "maxOccurs":
		setElementAttributeValue(&attrs.hasMaxOccurs, &attrs.maxOccurs, value)
	case "default":
		setElementAttributeValue(&attrs.hasDefault, &attrs.defaultVal, value)
	case "fixed":
		setElementAttributeValue(&attrs.hasFixed, &attrs.fixedVal, value)
	case "nillable":
		setElementAttributeValue(&attrs.hasNillable, &attrs.nillable, value)
	case "block":
		setElementAttributeValue(&attrs.hasBlock, &attrs.block, value)
	case "form":
		setElementAttributeValue(&attrs.hasForm, &attrs.form, value)
	case "abstract":
		attrs.hasAbstract = true
	case "final":
		attrs.hasFinal = true
	}
	recordElementAttributeProfileViolations(attrs, name)
}

func setElementAttributeValue(present *bool, dst *string, value string) {
	if *present {
		return
	}
	*present = true
	*dst = value
}

func recordElementAttributeProfileViolations(attrs *elementAttrScan, name string) {
	if attrs.invalidRefAttr == "" && !elementReferenceAttributeProfile.allows(name) {
		attrs.invalidRefAttr = name
	}
	if attrs.invalidLocalAttr == "" && !localElementAttributeProfile.allows(name) {
		attrs.invalidLocalAttr = name
	}
}
