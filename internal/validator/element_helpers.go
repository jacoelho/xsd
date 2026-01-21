package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// isSpecialAttribute checks if an attribute is a special XSI or XMLNS attribute.
func isSpecialAttribute(qname types.QName) bool {
	if qname.Namespace == xsdxml.XMLNSNamespace {
		return true
	}
	if qname.Namespace != xsdxml.XSINamespace {
		return false
	}
	switch qname.Local {
	case "type", "nil", "schemaLocation", "noNamespaceSchemaLocation":
		return true
	default:
		return false
	}
}

// isXMLNSAttribute checks if an attribute is an XML namespace declaration.
func isXMLNSAttribute(attr streamAttr) bool {
	return attr.NamespaceURI() == "xmlns" ||
		attr.NamespaceURI() == xsdxml.XMLNSNamespace ||
		attr.LocalName() == "xmlns"
}

// isAnyType checks if a compiled type is xs:anyType.
func isAnyType(ct *grammar.CompiledType) bool {
	return ct.QName.Local == "anyType" && ct.QName.Namespace == xsdxml.XSDNamespace
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
	// for mixed content without explicit text type, use the type itself if it's a simple type.
	if decl.Type.Original != nil {
		switch decl.Type.Original.(type) {
		case *types.SimpleType, *types.BuiltinType:
			return decl.Type
		}
	}
	return nil
}

// isAnySimpleType checks if a compiled type is xs:anySimpleType.
func isAnySimpleType(ct *grammar.CompiledType) bool {
	return ct.QName.Local == "anySimpleType" && ct.QName.Namespace == xsdxml.XSDNamespace
}

func isWhitespaceOnlyBytes(b []byte) bool {
	for _, r := range b {
		if !isXMLWhitespaceByte(r) {
			return false
		}
	}
	return true
}

func isXMLWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isWhitespaceOnly(b []byte) bool {
	return isWhitespaceOnlyBytes(b)
}
