package runtime

// AttributeDecl returns the validation read projection for an attribute.
func (rt *Schema) AttributeDecl(id AttributeID) (AttributeDeclRead, bool) {
	return AttributeDeclReadByID(rt.reads.Attributes, id)
}

// SimpleTypePrimitive returns the primitive kind for a simple type.
func (rt *Schema) SimpleTypePrimitive(id SimpleTypeID) (PrimitiveKind, bool) {
	return SimpleTypePrimitiveByID(rt.reads.SimpleTypePrimitives, id)
}

// ForEachElementIdentityConstraint iterates identity constraints on an element.
func (rt *Schema) ForEachElementIdentityConstraint(id ElementID, fn func(IdentityConstraintID) bool) {
	ForEachElementIdentityConstraint(rt.reads.ElementIdentities, id, fn)
}

// ElementIdentityConstraints returns identity constraints attached to an
// element.
func (rt *Schema) ElementIdentityConstraints(id ElementID) []IdentityConstraintID {
	if !ValidElementID(id, len(rt.reads.ElementIdentities)) {
		return nil
	}
	return rt.reads.ElementIdentities[id]
}

// HasIdentityConstraints reports whether the schema has identity constraints.
func (rt *Schema) HasIdentityConstraints() bool {
	return len(rt.reads.Identities) != 0
}

// IdentitySelectorPaths returns selector paths for an identity constraint.
func (rt *Schema) IdentitySelectorPaths(id IdentityConstraintID) ([]IdentityPath, bool) {
	return IdentitySelectorPaths(rt.reads.Identities, id)
}

// ForEachIdentitySelector iterates selector paths for an identity constraint.
func (rt *Schema) ForEachIdentitySelector(id IdentityConstraintID, fn func(IdentityPath) bool) bool {
	return ForEachIdentitySelector(rt.reads.Identities, id, fn)
}

// IdentityFieldCount returns the number of fields for an identity constraint.
func (rt *Schema) IdentityFieldCount(id IdentityConstraintID) (int, bool) {
	return IdentityFieldCount(rt.reads.Identities, id)
}

// IdentityElementFields returns element-field lookups for an identity
// constraint.
func (rt *Schema) IdentityElementFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityElementFields(rt.reads.Identities, id)
}

// ForEachIdentityElementField iterates element fields for an identity constraint.
func (rt *Schema) ForEachIdentityElementField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityElementField(rt.reads.Identities, id, fn)
}

// IdentityAttributeFields returns exact attribute-field lookups for an identity
// constraint and attribute name.
func (rt *Schema) IdentityAttributeFields(id IdentityConstraintID, name QName) ([]CompiledIdentityField, bool) {
	return IdentityAttributeFields(rt.reads.Identities, id, name)
}

// ForEachIdentityAttributeField iterates attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeField(id IdentityConstraintID, name QName, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeField(rt.reads.Identities, id, name, fn)
}

// IdentityAttributeWildcardFields returns wildcard attribute-field lookups for
// an identity constraint.
func (rt *Schema) IdentityAttributeWildcardFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityAttributeWildcardFields(rt.reads.Identities, id)
}

// ForEachIdentityAttributeWildcardField iterates wildcard attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeWildcardField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeWildcardField(rt.reads.Identities, id, fn)
}

// IdentityConstraintInfo returns metadata for an identity constraint.
func (rt *Schema) IdentityConstraintInfo(id IdentityConstraintID) (IdentityConstraintInfo, bool) {
	return IdentityConstraintInfoByID(rt.reads.Identities, id)
}

func (rt *Schema) elementChildContent(t TypeID) (ElementChildContent, bool) {
	return ElementChildContentByType(len(rt.reads.SimpleTypePrimitives), rt.reads.ComplexChildContent, t)
}

func (rt *Schema) complexAttributeUses(id ComplexTypeID) (AttributeUseSetRead, bool) {
	return AttributeUseSetReadForComplexType(rt.reads.ComplexAttributeUseSetIDs, rt.reads.AttributeUseSets, id)
}

// AttributeUseSetForType returns attribute-use reads for a runtime type.
func (rt *Schema) AttributeUseSetForType(typ TypeID) (AttributeUseSetRead, bool, bool) {
	return AttributeUseSetReadByType(rt.reads.ComplexAttributeUseSetIDs, rt.reads.AttributeUseSets, typ)
}

// AttributeUseSetForTypePtr returns attribute-use reads for a runtime type
// without copying the immutable read projection.
func (rt *Schema) AttributeUseSetForTypePtr(typ TypeID) (*AttributeUseSetRead, bool, bool) {
	return AttributeUseSetReadByTypePtr(rt.reads.ComplexAttributeUseSetIDs, rt.reads.AttributeUseSets, typ)
}

// SimpleContentType returns the simple-content type for a runtime type.
func (rt *Schema) SimpleContentType(t TypeID) (SimpleTypeID, bool, bool) {
	return SimpleContentTypeByType(len(rt.reads.SimpleTypePrimitives), rt.reads.ComplexSimpleContent, t)
}

// SimpleIdentity returns identity behavior for a simple type.
func (rt *Schema) SimpleIdentity(id SimpleTypeID) SimpleIdentityKind {
	return SimpleTypeIdentityByID(rt.reads.SimpleTypeIdentities, id)
}

// ElementValueConstraints returns value constraints for an element declaration.
func (rt *Schema) ElementValueConstraints(id ElementID) (ElementValueConstraints, bool, bool) {
	return ElementValueConstraintsByID(rt.reads.ElementValueConstraints, id)
}

// ElementTextContent returns text-content metadata for a runtime type and element.
func (rt *Schema) ElementTextContent(t TypeID, elem ElementID) (ElementTextContent, bool) {
	return ElementTextContentByType(
		len(rt.reads.SimpleTypePrimitives),
		rt.reads.ComplexTextContent,
		rt.reads.FixedComplexTextContent,
		rt.reads.ElementValueConstraints,
		rt.reads.SimpleTextContent,
		t,
		elem,
	)
}

// ElementHasSimpleContent reports whether a runtime type and element have simple content.
func (rt *Schema) ElementHasSimpleContent(t TypeID, elem ElementID) (bool, bool) {
	return ElementHasSimpleContentByType(
		len(rt.reads.SimpleTypePrimitives),
		rt.reads.ComplexTextContent,
		rt.reads.FixedComplexTextContent,
		rt.reads.ElementValueConstraints,
		rt.reads.SimpleTextContent,
		t,
		elem,
	)
}

// SimpleValueNeedsQNameResolver reports whether validating id can require
// lexical QName namespace resolution.
func (rt *Schema) SimpleValueNeedsQNameResolver(id SimpleTypeID) bool {
	if rt == nil {
		return false
	}
	if ValidSimpleTypeID(id, len(rt.reads.SimpleValueQNameResolverNeeds)) {
		return rt.reads.SimpleValueQNameResolverNeeds[id]
	}
	return false
}

// ValidateRawSimpleValue validates raw simple content through fast-path reads.
func (rt *Schema) ValidateRawSimpleValue(id SimpleTypeID, raw []byte) (bool, error) {
	read, ok := simpleValueTypeReadByID(rt.reads.SimpleValueTypes, id)
	if !ok {
		if id != NoSimpleType {
			return false, ErrSimpleValueMetadata
		}
		return false, nil
	}
	return validateRawSimpleValueType(rt.reads.SimpleValueTypes, rt.reads.SimpleValueFacets, id, &read.Type, raw)
}
