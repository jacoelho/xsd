package parser

import (
	"github.com/jacoelho/xsd/internal/value"
)

func isXMLNSDeclaration(attr Attr) bool {
	return attr.NamespaceURI() == value.XMLNSNamespace || (attr.NamespaceURI() == "" && attr.LocalName() == "xmlns")
}
