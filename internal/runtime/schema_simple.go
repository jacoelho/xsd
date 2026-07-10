package runtime

// SimpleTypeIdentity returns the stored ID/IDREF behavior for simple type id.
func (rt *Schema) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	return SimpleTypeIdentityByID(rt.runtime.SimpleTypeIdentities, id), ValidSimpleTypeID(id, len(rt.runtime.SimpleTypeIdentities))
}

// SimpleTypeIdentity returns compiler-owned identity metadata.
func (rt *SchemaBuild) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	st, ok := rt.UsableSimpleType(id)
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
