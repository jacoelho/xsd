package schema

import "github.com/jacoelho/xsd/internal/vocab"

// Test fixtures keep their short namespace labels while production code uses
// vocab as the sole vocabulary owner.
const (
	EmptyNamespaceURI = vocab.EmptyNamespaceURI
	XSDNamespaceURI   = vocab.XSDNamespaceURI
	XSINamespaceURI   = vocab.XSINamespaceURI
	XMLNamespaceURI   = vocab.XMLNamespaceURI
	XLinkNamespaceURI = vocab.XLinkNamespaceURI
	XMLNSNamespaceURI = vocab.XMLNSNamespaceURI
)
