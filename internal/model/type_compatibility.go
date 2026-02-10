package model

// ElementTypesCompatible reports whether two element declaration types are compatible.
// It compares by QName when available and falls back to identity for anonymous types.
func ElementTypesCompatible(a, b Type) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	nameA := a.Name()
	nameB := b.Name()
	if !nameA.IsZero() || !nameB.IsZero() {
		return nameA == nameB
	}
	return a == b
}
