package schema

import "slices"

// newDeclaredIdentityConstraint constructs a declared but not yet compiled
// identity constraint placeholder.
func newDeclaredIdentityConstraint(name QName) identityConstraintSpec {
	return identityConstraintSpec{Name: name, Refer: NoIdentityConstraint}
}

// newIdentityConstraint owns parsed selector and field metadata.
func newIdentityConstraint(kind IdentityKind, name QName, refer IdentityConstraintID, selector []identityPath, fields []identityField) identityConstraintSpec {
	if kind != IdentityKeyRef {
		refer = NoIdentityConstraint
	}
	selector = cloneIdentityPaths(selector)
	fields = cloneIdentityFields(fields)
	return identityConstraintSpec{
		Selector: selector,
		Fields:   fields,
		Name:     name,
		Refer:    refer,
		Kind:     kind,
	}
}

// cloneIdentityPaths deep-clones parsed identity selector path metadata.
func cloneIdentityPaths(in []identityPath) []identityPath {
	out := slices.Clone(in)
	for i := range out {
		out[i].Steps = slices.Clone(in[i].Steps)
	}
	return out
}

// cloneIdentityFields deep-clones parsed identity field metadata.
func cloneIdentityFields(in []identityField) []identityField {
	out := slices.Clone(in)
	for i := range out {
		out[i].Paths = cloneIdentityFieldPaths(in[i].Paths)
	}
	return out
}

func cloneIdentityFieldPaths(in []identityFieldPath) []identityFieldPath {
	out := slices.Clone(in)
	for i := range out {
		out[i] = cloneIdentityFieldPath(in[i])
	}
	return out
}

func cloneIdentityFieldPath(in identityFieldPath) identityFieldPath {
	in.Steps = slices.Clone(in.Steps)
	return in
}
