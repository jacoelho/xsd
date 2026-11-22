package schema

import "slices"

// cloneSimpleTypeDerivation deep-clones simple-type derivation projection
// metadata.
func cloneSimpleTypeDerivation(in simpleTypeDerivation) simpleTypeDerivation {
	in.Union = slices.Clone(in.Union)
	return in
}

// cloneValueConstraintIdentity deep-clones value-constraint identity
// projection metadata.
func cloneValueConstraintIdentity(in constraintIdentity) constraintIdentity {
	in.ResolvedNames = slices.Clone(in.ResolvedNames)
	return in
}
