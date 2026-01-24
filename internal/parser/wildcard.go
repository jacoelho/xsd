package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseWildcardConstraints(doc *xsdxml.Document, elem xsdxml.NodeID, elementName, allowedAttrs string, allowed map[string]bool) (types.NamespaceConstraint, []types.NamespaceURI, types.ProcessContents, error) {
	if doc.GetAttribute(elem, "notNamespace") != "" {
		return types.NSCInvalid, nil, types.Strict, fmt.Errorf("notNamespace attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}
	if doc.GetAttribute(elem, "notQName") != "" {
		return types.NSCInvalid, nil, types.Strict, fmt.Errorf("notQName attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}

	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == "" && !allowed[attrName] {
			return types.NSCInvalid, nil, types.Strict, fmt.Errorf("invalid attribute '%s' on <%s> element (XSD 1.0 only allows: %s)", attrName, elementName, allowedAttrs)
		}
	}

	namespaceAttr := doc.GetAttribute(elem, "namespace")
	hasNamespaceAttr := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "namespace" && attr.NamespaceURI() == "" {
			hasNamespaceAttr = true
			break
		}
	}
	if !hasNamespaceAttr {
		namespaceAttr = "##any"
	} else if namespaceAttr == "" {
		namespaceAttr = "##local"
	}

	nsConstraint, nsList, err := parseNamespaceConstraint(namespaceAttr)
	if err != nil {
		return types.NSCInvalid, nil, types.Strict, fmt.Errorf("parse namespace constraint: %w", err)
	}

	processContents := doc.GetAttribute(elem, "processContents")
	hasProcessContents := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "processContents" && attr.NamespaceURI() == "" {
			hasProcessContents = true
			break
		}
	}
	if hasProcessContents && processContents == "" {
		return types.NSCInvalid, nil, types.Strict, fmt.Errorf("processContents attribute cannot be empty")
	}

	switch processContents {
	case "strict":
		return nsConstraint, nsList, types.Strict, nil
	case "lax":
		return nsConstraint, nsList, types.Lax, nil
	case "skip":
		return nsConstraint, nsList, types.Skip, nil
	case "":
		return nsConstraint, nsList, types.Strict, nil
	default:
		return types.NSCInvalid, nil, types.Strict, fmt.Errorf("invalid processContents value '%s': must be 'strict', 'lax', or 'skip'", processContents)
	}
}
