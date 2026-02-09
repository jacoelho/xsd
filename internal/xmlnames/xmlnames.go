package xmlnames

import (
	"bytes"
	"fmt"
)

const (
	// XMLPrefix is the reserved prefix for the XML namespace.
	XMLPrefix = "xml"
	// XMLNSPrefix is the reserved prefix for namespace declarations.
	XMLNSPrefix = "xmlns"
	// XMLNamespace is the XML namespace URI.
	XMLNamespace = "http://www.w3.org/XML/1998/namespace"
	// XMLNSNamespace is the XMLNS namespace URI.
	XMLNSNamespace = "http://www.w3.org/2000/xmlns/"
	// XSINamespace is the XML Schema instance namespace URI.
	XSINamespace = "http://www.w3.org/2001/XMLSchema-instance"
)

var (
	xmlPrefixBytes    = []byte(XMLPrefix)
	xmlnsPrefixBytes  = []byte(XMLNSPrefix)
	xmlNamespaceBytes = []byte(XMLNamespace)
)

// IsXMLPrefix reports whether prefix is the reserved xml prefix.
func IsXMLPrefix(prefix []byte) bool {
	return bytes.Equal(prefix, xmlPrefixBytes)
}

// IsXMLNSPrefix reports whether prefix is the reserved xmlns prefix.
func IsXMLNSPrefix(prefix []byte) bool {
	return bytes.Equal(prefix, xmlnsPrefixBytes)
}

// XMLNamespaceBytes returns the XML namespace URI as bytes.
// The returned slice must be treated as read-only.
func XMLNamespaceBytes() []byte {
	return xmlNamespaceBytes
}

// ValidateXMLPrefixBinding verifies that an explicit xml prefix binding is correct.
func ValidateXMLPrefixBinding(binding string, ok bool) error {
	if !ok {
		return nil
	}
	if binding != XMLNamespace {
		return fmt.Errorf("prefix %s must be bound to %s", XMLPrefix, XMLNamespace)
	}
	return nil
}

// ValidateXMLPrefixBindingBytes verifies that an explicit xml prefix binding is correct.
func ValidateXMLPrefixBindingBytes(binding []byte, ok bool) error {
	if !ok {
		return nil
	}
	if !bytes.Equal(binding, xmlNamespaceBytes) {
		return fmt.Errorf("prefix %s must be bound to %s", XMLPrefix, XMLNamespace)
	}
	return nil
}
