package grammar

import (
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// CompiledElement is a fully-resolved element declaration.
type CompiledElement struct {
	QName          types.QName
	EffectiveQName types.QName
	Original       *types.ElementDecl
	// Direct pointer (not QName)
	Type *CompiledType

	// Substitution group membership (pre-computed transitive closure)
	// What this element substitutes
	SubstitutionHead *CompiledElement
	// Elements that can substitute this
	Substitutes []*CompiledElement

	// Element properties
	Nillable bool
	Abstract bool
	Default  string
	Fixed    string
	// true if fixed="" was explicitly present (even if empty)
	HasFixed bool
	Block    types.DerivationSet

	// Identity constraints (resolved)
	Constraints []*CompiledConstraint
}

// CompiledConstraint represents a resolved identity constraint.
type CompiledConstraint struct {
	Original      *types.IdentityConstraint
	SelectorPaths []xpath.Path
	FieldPaths    [][]xpath.Path
}
