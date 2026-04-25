package schemaast

import "github.com/jacoelho/xsd/internal/xsdlex"

// NamespaceURI represents a namespace URI.
type NamespaceURI = xsdlex.NamespaceURI

// NamespaceEmpty represents an empty namespace URI (no namespace).
const NamespaceEmpty = xsdlex.NamespaceEmpty

// QName represents a qualified name with namespace and local part.
type QName = xsdlex.QName

var (
	ResolveNamespace = xsdlex.ResolveNamespace
	SplitQName       = xsdlex.SplitQName
	ParseQName       = xsdlex.ParseQName
	IsValidNCName    = xsdlex.IsValidNCName
	IsValidQName     = xsdlex.IsValidQName
	ParseQNameValue  = xsdlex.ParseQNameValue
)
