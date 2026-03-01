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
	xmlNamespaceBytes = []byte(XMLNamespace)
)

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

// ValidateNamespaceDeclBinding verifies namespace declaration reserved bindings.
func ValidateNamespaceDeclBinding(prefix, uri string) error {
	switch prefix {
	case "":
		if uri == XMLNamespace {
			return fmt.Errorf("default namespace must not be %s", XMLNamespace)
		}
		if uri == XMLNSNamespace {
			return fmt.Errorf("default namespace must not be %s", XMLNSNamespace)
		}
	case XMLPrefix:
		if uri != XMLNamespace {
			return fmt.Errorf("prefix %s must be bound to %s", XMLPrefix, XMLNamespace)
		}
	case XMLNSPrefix:
		return fmt.Errorf("prefix %s must not be declared", XMLNSPrefix)
	default:
		if uri == XMLNamespace {
			return fmt.Errorf("prefix %s must not be bound to %s", prefix, XMLNamespace)
		}
		if uri == XMLNSNamespace {
			return fmt.Errorf("prefix %s must not be bound to %s", prefix, XMLNSNamespace)
		}
	}
	return nil
}
