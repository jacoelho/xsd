package grammar

import "github.com/jacoelho/xsd/internal/types"

// CompiledElement is a fully-resolved element declaration.
type CompiledElement struct {
	QName    types.QName
	Original *types.ElementDecl
	Type     *CompiledType // Direct pointer (not QName)

	// Substitution group membership (pre-computed transitive closure)
	SubstitutionHead *CompiledElement   // What this element substitutes
	Substitutes      []*CompiledElement // Elements that can substitute this

	// Element properties
	Nillable bool
	Abstract bool
	Default  string
	Fixed    string
	HasFixed bool // true if fixed="" was explicitly present (even if empty)
	Block    types.DerivationSet

	// Identity constraints (resolved)
	Constraints []*CompiledConstraint
}

// CompiledConstraint represents a resolved identity constraint.
type CompiledConstraint struct {
	Original *types.IdentityConstraint
}
