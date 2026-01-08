package types

import "fmt"

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
