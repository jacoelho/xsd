package value

import (
	"bytes"
	"fmt"
	"unicode/utf8"
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
	// XSDNamespace is the XML Schema namespace URI.
	XSDNamespace = "http://www.w3.org/2001/XMLSchema"
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

// DocumentState tracks document-boundary lexical state shared by XML token loops.
type DocumentState struct {
	allowBOM   bool
	rootSeen   bool
	rootClosed bool
}

// NewDocumentState returns an initialized document-boundary state.
func NewDocumentState() DocumentState {
	return DocumentState{allowBOM: true}
}

// RootSeen reports whether a root start element has been seen.
func (s *DocumentState) RootSeen() bool {
	return s != nil && s.rootSeen
}

// RootClosed reports whether the root element has been closed.
func (s *DocumentState) RootClosed() bool {
	return s != nil && s.rootClosed
}

// StartElementAllowed reports whether a start element may appear at this point.
func (s *DocumentState) StartElementAllowed() bool {
	return s == nil || !s.rootClosed
}

// OnStartElement advances state for a start-element token.
func (s *DocumentState) OnStartElement() {
	if s == nil {
		return
	}
	s.rootSeen = true
	s.allowBOM = false
}

// OnEndElement advances state for an end-element token.
// closeRoot should be true only when this token closes the document root element.
func (s *DocumentState) OnEndElement(closeRoot bool) {
	if s == nil {
		return
	}
	if closeRoot {
		s.rootClosed = true
	}
	s.allowBOM = false
}

// ValidateOutsideCharData reports whether character data outside root is ignorable.
func (s *DocumentState) ValidateOutsideCharData(data []byte) bool {
	if s == nil {
		return IsIgnorableOutsideRoot(data, true)
	}
	ok := IsIgnorableOutsideRoot(data, s.allowBOM)
	if ok {
		s.allowBOM = false
	}
	return ok
}

// OnOutsideMarkup advances state for comments/PI/directives outside root.
func (s *DocumentState) OnOutsideMarkup() {
	if s == nil {
		return
	}
	s.allowBOM = false
}

// IsIgnorableOutsideRoot reports whether data contains only XML whitespace.
// If allowBOM is true, a leading BOM is permitted before any other character.
func IsIgnorableOutsideRoot(data []byte, allowBOM bool) bool {
	sawNonBOM := false
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r == '\uFEFF' {
			if !allowBOM || sawNonBOM {
				return false
			}
			allowBOM = false
			data = data[size:]
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		sawNonBOM = true
		allowBOM = false
		data = data[size:]
	}
	return true
}
