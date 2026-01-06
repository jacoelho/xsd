package types

// QName represents a qualified name with namespace and local part
type QName struct {
	Namespace NamespaceURI
	Local     string
}

// String returns the QName in {namespace}local format, or just local if no namespace
func (q QName) String() string {
	if q.Namespace.IsEmpty() {
		return q.Local
	}
	return "{" + q.Namespace.String() + "}" + q.Local
}

// IsZero returns true if the QName is the zero value
func (q QName) IsZero() bool {
	return q.Namespace.IsEmpty() && q.Local == ""
}

// Equal returns true if two QNames are equal
func (q QName) Equal(other QName) bool {
	return q.Namespace == other.Namespace && q.Local == other.Local
}
