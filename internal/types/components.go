package types

import "fmt"

// NamedComponent exposes the component name without namespace details.
type NamedComponent interface {
	// ComponentName returns the QName of this component.
	ComponentName() QName
}

// NamespacedComponent exposes the declared namespace for a component.
type NamespacedComponent interface {
	// DeclaredNamespace returns the targetNamespace where this component was declared.
	DeclaredNamespace() NamespaceURI
}

// SchemaComponent is implemented by any named component in a schema.
type SchemaComponent interface {
	NamedComponent
	NamespacedComponent
}

// FormChoice represents the form attribute value for element/attribute declarations.
type FormChoice int

const (
	// FormDefault uses the schema's elementFormDefault/attributeFormDefault setting.
	FormDefault FormChoice = iota
	// FormQualified requires the element/attribute to be in the target namespace.
	FormQualified
	// FormUnqualified requires the element/attribute to be in no namespace.
	FormUnqualified
)

// UnboundedOccurs indicates no upper bound on occurrences (-1 per XSD spec).
const UnboundedOccurs = -1

// ElementDecl represents an element declaration
type ElementDecl struct {
	Name              QName
	Type              Type
	MinOccurs         int
	MaxOccurs         int
	Nillable          bool
	Abstract          bool
	SubstitutionGroup QName
	Block             DerivationSet
	// Derivation methods blocked for this element
	Final   DerivationSet
	Default string
	Fixed   string
	// True if fixed attribute was explicitly set (even if empty)
	HasFixed    bool
	Constraints []*IdentityConstraint
	IsReference bool
	// targetNamespace of the schema where this element was originally declared
	SourceNamespace NamespaceURI
	// Element's form attribute (qualified/unqualified)
	Form FormChoice
}

// NewElementDeclFromParsed validates a parsed element declaration and returns it if valid.
func NewElementDeclFromParsed(decl *ElementDecl) (*ElementDecl, error) {
	if decl == nil {
		return nil, fmt.Errorf("element declaration is nil")
	}
	if decl.Name.IsZero() {
		return nil, fmt.Errorf("element declaration missing name")
	}
	if !decl.IsReference && decl.Type == nil {
		return nil, fmt.Errorf("element %s must declare a type", decl.Name)
	}
	if decl.MinOccurs < 0 {
		return nil, fmt.Errorf("element %s has negative minOccurs", decl.Name)
	}
	if decl.MaxOccurs != UnboundedOccurs && decl.MaxOccurs < decl.MinOccurs {
		return nil, fmt.Errorf("element %s has maxOccurs less than minOccurs", decl.Name)
	}
	return decl, nil
}

// MinOcc implements Particle interface
func (e *ElementDecl) MinOcc() int {
	return e.MinOccurs
}

// MaxOcc implements Particle interface
func (e *ElementDecl) MaxOcc() int {
	return e.MaxOccurs
}

// ComponentName returns the QName of this component.
// Implements SchemaComponent interface.
func (e *ElementDecl) ComponentName() QName {
	return e.Name
}

// DeclaredNamespace returns the targetNamespace where this component was declared.
// Implements SchemaComponent interface.
func (e *ElementDecl) DeclaredNamespace() NamespaceURI {
	return e.SourceNamespace
}

// Copy creates a copy of the element declaration with remapped QNames.
func (e *ElementDecl) Copy(opts CopyOptions) *ElementDecl {
	clone := *e
	clone.Name = opts.RemapQName(e.Name)
	clone.SourceNamespace = opts.SourceNamespace
	if e.Type != nil {
		clone.Type = CopyType(e.Type, opts)
	}
	if !e.SubstitutionGroup.IsZero() {
		clone.SubstitutionGroup = opts.RemapQName(e.SubstitutionGroup)
	}
	clone.Constraints = copyIdentityConstraints(e.Constraints, opts)
	return &clone
}

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
	Name    QName
	Type    Type
	Use     AttributeUse
	Default string
	Fixed   string
	// True if fixed attribute was explicitly set (even if empty)
	HasFixed bool
	// targetNamespace of the schema where this attribute was originally declared
	SourceNamespace NamespaceURI
	// Attribute's form attribute (qualified/unqualified)
	Form FormChoice
	// True if this came from ref="...", false if from name="..."
	IsReference bool
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
	Name         QName
	Attributes   []*AttributeDecl
	AttrGroups   []QName
	AnyAttribute *AnyAttribute
	// targetNamespace of the schema where this attribute group was originally declared
	SourceNamespace NamespaceURI
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
