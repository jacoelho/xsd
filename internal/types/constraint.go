package types

// ConstraintType represents the type of identity constraint
type ConstraintType int

const (
	// UniqueConstraint indicates an xs:unique constraint.
	UniqueConstraint ConstraintType = iota
	// KeyConstraint indicates an xs:key constraint.
	KeyConstraint
	// KeyRefConstraint indicates an xs:keyref constraint.
	KeyRefConstraint
)

// String returns the string representation of the constraint type
func (c ConstraintType) String() string {
	switch c {
	case UniqueConstraint:
		return "unique"
	case KeyConstraint:
		return "key"
	case KeyRefConstraint:
		return "keyref"
	default:
		return "unknown"
	}
}

// IdentityConstraint represents key, keyref, or unique constraints
type IdentityConstraint struct {
	Name string
	// From enclosing <xs:schema targetNamespace="...">
	TargetNamespace NamespaceURI
	Type            ConstraintType
	Selector        Selector
	Fields          []Field
	ReferQName      QName
	// NamespaceContext maps namespace prefixes to URIs from the XSD schema.
	// Used to resolve prefixed QNames in selector/field XPath expressions.
	NamespaceContext map[string]string
}

// Selector represents a selector XPath expression
type Selector struct {
	XPath string
}

// Field represents a field XPath expression
type Field struct {
	XPath string
	// Optional: type hint from schema
	Type Type
	// Resolved during schema loading
	ResolvedType Type
}
