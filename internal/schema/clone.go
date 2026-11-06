package schema

import "slices"

// cloneWildcard deep-clones wildcard metadata.
func cloneWildcard(in wildcardRuntime) wildcardRuntime {
	in.Namespaces = slices.Clone(in.Namespaces)
	return in
}

// cloneContentModel deep-clones content-model metadata.
func cloneContentModel(in contentModelNode) contentModelNode {
	in.Particles = slices.Clone(in.Particles)
	return in
}

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
	in.qnameContext = cloneValueConstraintQNameContext(in.qnameContext)
	return in
}
