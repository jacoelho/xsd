package parser

import "github.com/jacoelho/xsd/internal/schemaxml"

func isXMLNSDeclaration(attr schemaxml.Attr) bool {
	if attr.NamespaceURI() == schemaxml.XMLNSNamespace {
		return true
	}
	return attr.NamespaceURI() == "" && attr.LocalName() == "xmlns"
}
