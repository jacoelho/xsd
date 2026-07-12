package runtime

// SimpleTypeIdentity returns compiler-owned identity metadata.
func (rt *SchemaBuild) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	st, ok := UsableSimpleType(rt.SimpleTypes, id)
	if !ok {
		return SimpleIdentityNone, false
	}
	return st.Identity, true
}

// DerivedSimpleIdentity derives compiler-owned identity metadata.
func (rt *SchemaBuild) DerivedSimpleIdentity(st SimpleType) SimpleIdentityKind {
	return DerivedSimpleIdentityForSimpleType(rt, st)
}

// ValidateSimpleValue validates a lexical simple value using frozen runtime reads.
func (rt *Schema) ValidateSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	return rt.validatePublishedSimpleValue(id, lexical, resolve, needs)
}
