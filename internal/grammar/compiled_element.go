package grammar

import (
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// CompiledElement is a fully-resolved element declaration.
type CompiledElement struct {
	QName            types.QName
	EffectiveQName   types.QName
	Default          string
	Fixed            string
	Original         *types.ElementDecl
	Type             *CompiledType
	SubstitutionHead *CompiledElement
	Substitutes      []*CompiledElement
	QName            types.QName
	EffectiveQName   types.QName
	Fixed            string
	Default          string
	Constraints      []*CompiledConstraint
	Block            types.DerivationSet
	HasDefault       bool
	HasFixed         bool
	Nillable         bool
	Abstract         bool
	HasDefault       bool
	HasFixed         bool
}

// CompiledConstraint represents a resolved identity constraint.
type CompiledConstraint struct {
	Original      *types.IdentityConstraint
	SelectorPaths []xpath.Path
	FieldPaths    [][]xpath.Path
}

// QName returns the effective QName for the constraint.
// If the constraint has no target namespace, it falls back to the declaring element's namespace.
func (c *CompiledConstraint) QName(decl *CompiledElement) types.QName {
	if c == nil || c.Original == nil {
		return types.QName{}
	}
	ns := c.Original.TargetNamespace
	if ns.IsEmpty() && decl != nil {
		ns = decl.QName.Namespace
	}
	return types.QName{Namespace: ns, Local: c.Original.Name}
}
