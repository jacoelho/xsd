package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// isSpecialAttribute checks if an attribute is a special XSI or XMLNS attribute.
func isSpecialAttribute(qname types.QName) bool {
	return qname.Namespace == xml.XSINamespace ||
		qname.Namespace == xml.XMLNSNamespace
}

// isXMLNSAttribute checks if an attribute is an XML namespace declaration.
func isXMLNSAttribute(attr xml.Attr) bool {
	return attr.NamespaceURI() == "xmlns" ||
		attr.NamespaceURI() == xml.XMLNSNamespace ||
		attr.LocalName() == "xmlns"
}

// isAnyType checks if a compiled type is xs:anyType.
func isAnyType(ct *grammar.CompiledType) bool {
	return ct.QName.Local == "anyType" && ct.QName.Namespace == xml.XSDNamespace
}

// textTypeForFixedValue returns the type to use for normalizing fixed value comparisons.
// For simple types and complex types with simpleContent, returns the text type.
// For mixed content types without explicit text type, returns the type itself if simple.
// Returns nil if no normalization type can be determined.
func textTypeForFixedValue(decl *grammar.CompiledElement) *grammar.CompiledType {
	if decl == nil || decl.Type == nil {
		return nil
	}
	textType := decl.Type.TextType()
	if textType != nil {
		return textType
	}
	// For mixed content without explicit text type, use the type itself if it's a simple type.
	if decl.Type.Original != nil {
		if _, ok := decl.Type.Original.(types.SimpleTypeDefinition); ok {
			return decl.Type
		}
	}
	return nil
}

// isAnySimpleType checks if a compiled type is xs:anySimpleType.
func isAnySimpleType(ct *grammar.CompiledType) bool {
	return ct.QName.Local == "anySimpleType" && ct.QName.Namespace == xml.XSDNamespace
}

// isWhitespaceOnly checks if a string contains only whitespace characters.
func isWhitespaceOnly(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

// appendPath creates a new path by appending a component.
func appendPath(path, component string) string {
	if path == "/" {
		return "/" + component
	}
	return path + "/" + component
}

// getElementChildren returns element children of an element.
func getElementChildren(elem xml.Element) []xml.Element {
	return elem.Children()
}
