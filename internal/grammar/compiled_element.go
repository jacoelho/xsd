package grammar

import (
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// CompiledElement is a fully-resolved element declaration.
type CompiledElement struct {
	Original         *types.ElementDecl
	Type             *CompiledType
	SubstitutionHead *CompiledElement
	QName            types.QName
	EffectiveQName   types.QName
	Fixed            string
	Default          string
	HasDefault       bool
	Substitutes      []*CompiledElement
	Constraints      []*CompiledConstraint
	Block            types.DerivationSet
	Nillable         bool
	Abstract         bool
	HasFixed         bool
}

// CompiledConstraint represents a resolved identity constraint.
type CompiledConstraint struct {
	Original      *types.IdentityConstraint
	SelectorPaths []xpath.Path
	FieldPaths    [][]xpath.Path
}
