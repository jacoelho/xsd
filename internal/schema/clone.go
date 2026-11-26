package schema

import "slices"

// cloneValueConstraintIdentity deep-clones value-constraint identity
// projection metadata.
func cloneValueConstraintIdentity(in constraintIdentity) constraintIdentity {
	in.ResolvedNames = slices.Clone(in.ResolvedNames)
	return in
}
