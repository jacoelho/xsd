package parser

import (
	"github.com/jacoelho/xsd/internal/xmlnames"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func isXMLNSDeclaration(attr xmltree.Attr) bool {
	if attr.NamespaceURI() == xmlnames.XMLNSNamespace {
		return true
	}
	return attr.NamespaceURI() == "" && attr.LocalName() == "xmlns"
}
