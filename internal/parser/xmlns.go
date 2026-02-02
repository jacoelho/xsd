package parser

import "github.com/jacoelho/xsd/internal/xml"

func isXMLNSDeclaration(attr xsdxml.Attr) bool {
	if attr.NamespaceURI() == xsdxml.XMLNSNamespace {
		return true
	}
	return attr.NamespaceURI() == "" && attr.LocalName() == "xmlns"
}
