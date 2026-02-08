package parser

import (
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func namespaceForPrefix(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, prefix string) string {
	for current := elem; current != xsdxml.InvalidNode; current = doc.Parent(current) {
		for _, attr := range doc.Attributes(current) {
			if !isXMLNSDeclaration(attr) {
				continue
			}
			if prefix == "" {
				if attr.LocalName() == "xmlns" {
					return attr.Value()
				}
				continue
			}
			if attr.LocalName() == prefix {
				return attr.Value()
			}
		}
	}

	if schema.NamespaceDecls != nil {
		if prefix == "" {
			if ns, ok := schema.NamespaceDecls[""]; ok {
				return ns
			}
		} else if ns, ok := schema.NamespaceDecls[prefix]; ok {
			return ns
		}
	}

	if prefix == "xml" {
		return xsdxml.XMLNamespace
	}
	return ""
}

func namespaceContextForElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) map[string]string {
	context := make(map[string]string)
	for current := elem; current != xsdxml.InvalidNode; current = doc.Parent(current) {
		for _, attr := range doc.Attributes(current) {
			ns := attr.NamespaceURI()
			local := attr.LocalName()
			if ns != xsdxml.XMLNSNamespace && (ns != "" || local != "xmlns") {
				continue
			}
			prefix := local
			if prefix == "xmlns" {
				prefix = ""
			}
			if _, exists := context[prefix]; !exists {
				context[prefix] = attr.Value()
			}
		}
	}

	if schema != nil {
		for prefix, uri := range schema.NamespaceDecls {
			if _, exists := context[prefix]; !exists {
				context[prefix] = uri
			}
		}
	}

	if _, exists := context["xml"]; !exists {
		context["xml"] = xsdxml.XMLNamespace
	}

	return context
}
