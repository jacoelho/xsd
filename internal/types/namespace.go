package types

// NamespaceURI represents a namespace URI
// This is a newtype over string to provide type safety for namespace URIs
type NamespaceURI string

// NamespaceEmpty represents an empty namespace URI (no namespace)
const NamespaceEmpty NamespaceURI = ""

// String returns the namespace URI as a string
func (ns NamespaceURI) String() string {
	return string(ns)
}

// IsEmpty returns true if the namespace URI is empty
func (ns NamespaceURI) IsEmpty() bool {
	return ns == NamespaceEmpty
}

// Equal returns true if two namespace URIs are equal
func (ns NamespaceURI) Equal(other NamespaceURI) bool {
	return ns == other
}
