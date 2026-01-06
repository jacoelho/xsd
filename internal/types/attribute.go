package types

import "fmt"

// AttributeUse represents how an attribute is used in an element declaration.
type AttributeUse int

const (
	// Optional indicates the attribute is optional (may be present or absent).
	Optional AttributeUse = iota
	// Required indicates the attribute must be present.
	Required
	// Prohibited indicates the attribute must not be present.
	Prohibited
)

// AttributeDecl represents an attribute declaration
type AttributeDecl struct {
	Name            QName
	Type            Type
	Use             AttributeUse
	Default         string
	Fixed           string
	HasFixed        bool         // True if fixed attribute was explicitly set (even if empty)
	SourceNamespace NamespaceURI // targetNamespace of the schema where this attribute was originally declared
	Form            FormChoice   // Attribute's form attribute (qualified/unqualified)
	IsReference     bool         // True if this came from ref="...", false if from name="..."
}

// NewAttributeDeclFromParsed validates a parsed attribute declaration and returns it if valid.
func NewAttributeDeclFromParsed(decl *AttributeDecl) (*AttributeDecl, error) {
	if decl == nil {
		return nil, fmt.Errorf("attribute declaration is nil")
	}
	if decl.Name.IsZero() {
		return nil, fmt.Errorf("attribute declaration missing name")
	}
	return decl, nil
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (a *AttributeDecl) ComponentName() QName {
	return a.Name
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (a *AttributeDecl) DeclaredNamespace() NamespaceURI {
	return a.SourceNamespace
}

// Copy creates a copy of the attribute declaration with remapped QNames.
func (a *AttributeDecl) Copy(opts CopyOptions) *AttributeDecl {
	clone := *a
	clone.Name = opts.RemapQName(a.Name)
	clone.SourceNamespace = opts.SourceNamespace
	if a.Type != nil {
		clone.Type = CopyType(a.Type, opts)
	}
	return &clone
}

// AttributeGroup represents an attribute group definition
type AttributeGroup struct {
	Name            QName
	Attributes      []*AttributeDecl
	AttrGroups      []QName
	AnyAttribute    *AnyAttribute
	SourceNamespace NamespaceURI // targetNamespace of the schema where this attribute group was originally declared
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (g *AttributeGroup) ComponentName() QName {
	return g.Name
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (g *AttributeGroup) DeclaredNamespace() NamespaceURI {
	return g.SourceNamespace
}

// Copy creates a copy of the attribute group with remapped QNames.
func (g *AttributeGroup) Copy(opts CopyOptions) *AttributeGroup {
	clone := *g
	clone.Name = opts.RemapQName(g.Name)
	clone.SourceNamespace = opts.SourceNamespace
	clone.Attributes = copyAttributeDecls(g.Attributes, opts)
	clone.AttrGroups = copyQNameSlice(g.AttrGroups, opts.RemapQName)
	clone.AnyAttribute = copyAnyAttribute(g.AnyAttribute)
	return &clone
}
