package types

// NotationDecl represents a notation declaration
type NotationDecl struct {
	Name QName
	// public identifier (optional)
	Public string
	// system identifier (optional)
	System string
	// targetNamespace of the schema where this notation was originally declared
	SourceNamespace NamespaceURI
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (n *NotationDecl) ComponentName() QName {
	return n.Name
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (n *NotationDecl) DeclaredNamespace() NamespaceURI {
	return n.SourceNamespace
}

// Copy creates a copy of the notation declaration with remapped QNames.
func (n *NotationDecl) Copy(opts CopyOptions) *NotationDecl {
	clone := *n
	clone.Name = opts.RemapQName(n.Name)
	clone.SourceNamespace = opts.SourceNamespace
	return &clone
}
