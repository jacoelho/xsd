package parser

import "github.com/jacoelho/xsd/internal/xml"

func isXMLNSDeclaration(attr xsdxml.Attr) bool {
	if attr.NamespaceURI() == xsdxml.XMLNSNamespace || attr.NamespaceURI() == "xmlns" {
		return true
	}
	return attr.LocalName() == "xmlns"
}
