package schema

// newDeclaredIdentityConstraint constructs a declared but not yet compiled
// identity constraint placeholder.
func newDeclaredIdentityConstraint(name QName) identityConstraintSpec {
	return identityConstraintSpec{Name: name, Refer: NoIdentityConstraint}
}

// newIdentityConstraint builds an identity constraint over selector and
// fields without copying them. The paths are immutable once admitted; a
// selector or field node compiled in several document contexts shares them.
func newIdentityConstraint(kind IdentityKind, name QName, refer IdentityConstraintID, selector []identityPath, fields []identityField) identityConstraintSpec {
	if kind != IdentityKeyRef {
		refer = NoIdentityConstraint
	}
	return identityConstraintSpec{
		Selector: selector,
		Fields:   fields,
		Name:     name,
		Refer:    refer,
		Kind:     kind,
	}
}
