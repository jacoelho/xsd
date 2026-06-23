package runtime

// AttributeDecl returns the validation read projection for an attribute.
func (rt *Schema) AttributeDecl(id AttributeID) (AttributeDeclRead, bool) {
	return AttributeDeclReadByID(rt.AttributeDeclReads, id)
}

// SimpleTypePrimitive returns the primitive kind for a simple type.
func (rt *Schema) SimpleTypePrimitive(id SimpleTypeID) (PrimitiveKind, bool) {
	if !rt.ReadProjectionsPublished() {
		st, ok := rt.UsableSimpleType(id)
		if !ok {
			return 0, false
		}
		return st.Primitive, true
	}
	return SimpleTypePrimitiveByID(rt.SimpleTypePrimitives, id)
}

// ForEachElementIdentityConstraint iterates identity constraints on an element.
func (rt *Schema) ForEachElementIdentityConstraint(id ElementID, fn func(IdentityConstraintID) bool) {
	ForEachElementIdentityConstraint(rt.ElementIdentityConstraintReads, id, fn)
}

// ElementIdentityConstraints returns identity constraints attached to an
// element.
func (rt *Schema) ElementIdentityConstraints(id ElementID) []IdentityConstraintID {
	if !ValidElementID(id, len(rt.ElementIdentityConstraintReads)) {
		return nil
	}
	return rt.ElementIdentityConstraintReads[id]
}

// HasIdentityConstraints reports whether the schema has identity constraints.
func (rt *Schema) HasIdentityConstraints() bool {
	return len(rt.IdentityConstraintReads) != 0
}

// IdentitySelectorPaths returns selector paths for an identity constraint.
func (rt *Schema) IdentitySelectorPaths(id IdentityConstraintID) ([]IdentityPath, bool) {
	return IdentitySelectorPaths(rt.IdentityConstraintReads, id)
}

// ForEachIdentitySelector iterates selector paths for an identity constraint.
func (rt *Schema) ForEachIdentitySelector(id IdentityConstraintID, fn func(IdentityPath) bool) bool {
	return ForEachIdentitySelector(rt.IdentityConstraintReads, id, fn)
}

// IdentityFieldCount returns the number of fields for an identity constraint.
func (rt *Schema) IdentityFieldCount(id IdentityConstraintID) (int, bool) {
	return IdentityFieldCount(rt.IdentityConstraintReads, id)
}

// IdentityElementFields returns element-field lookups for an identity
// constraint.
func (rt *Schema) IdentityElementFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityElementFields(rt.IdentityConstraintReads, id)
}

// ForEachIdentityElementField iterates element fields for an identity constraint.
func (rt *Schema) ForEachIdentityElementField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityElementField(rt.IdentityConstraintReads, id, fn)
}

// IdentityAttributeFields returns exact attribute-field lookups for an identity
// constraint and attribute name.
func (rt *Schema) IdentityAttributeFields(id IdentityConstraintID, name QName) ([]CompiledIdentityField, bool) {
	return IdentityAttributeFields(rt.IdentityConstraintReads, id, name)
}

// ForEachIdentityAttributeField iterates attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeField(id IdentityConstraintID, name QName, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeField(rt.IdentityConstraintReads, id, name, fn)
}

// IdentityAttributeWildcardFields returns wildcard attribute-field lookups for
// an identity constraint.
func (rt *Schema) IdentityAttributeWildcardFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityAttributeWildcardFields(rt.IdentityConstraintReads, id)
}

// ForEachIdentityAttributeWildcardField iterates wildcard attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeWildcardField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeWildcardField(rt.IdentityConstraintReads, id, fn)
}

// IdentityConstraintInfo returns metadata for an identity constraint.
func (rt *Schema) IdentityConstraintInfo(id IdentityConstraintID) (IdentityConstraintInfo, bool) {
	return IdentityConstraintInfoByID(rt.IdentityConstraintReads, id)
}

func (rt *Schema) elementChildContent(t TypeID) (ElementChildContent, bool) {
	return ElementChildContentByType(len(rt.SimpleTypePrimitives), rt.ComplexChildContentReads, t)
}

func (rt *Schema) complexAttributeUses(id ComplexTypeID) (AttributeUseSetRead, bool) {
	return AttributeUseSetReadForComplexType(rt.ComplexAttributeUseSetIDs, rt.AttributeUseSetReads, id)
}

// AttributeUseSetForType returns attribute-use reads for a runtime type.
func (rt *Schema) AttributeUseSetForType(typ TypeID) (AttributeUseSetRead, bool, bool) {
	return AttributeUseSetReadByType(rt.ComplexAttributeUseSetIDs, rt.AttributeUseSetReads, typ)
}

// AttributeUseSetForTypePtr returns attribute-use reads for a runtime type
// without copying the immutable read projection.
func (rt *Schema) AttributeUseSetForTypePtr(typ TypeID) (*AttributeUseSetRead, bool, bool) {
	return AttributeUseSetReadByTypePtr(rt.ComplexAttributeUseSetIDs, rt.AttributeUseSetReads, typ)
}

// SimpleContentType returns the simple-content type for a runtime type.
func (rt *Schema) SimpleContentType(t TypeID) (SimpleTypeID, bool, bool) {
	return SimpleContentTypeByType(len(rt.SimpleTypePrimitives), rt.ComplexSimpleContentReads, t)
}

// SimpleIdentity returns identity behavior for a simple type.
func (rt *Schema) SimpleIdentity(id SimpleTypeID) SimpleIdentityKind {
	if !rt.ReadProjectionsPublished() {
		identity, ok := rt.SimpleTypeIdentity(id)
		if !ok {
			return SimpleIdentityNone
		}
		return identity
	}
	return SimpleTypeIdentityByID(rt.SimpleTypeIdentities, id)
}

// ElementValueConstraints returns value constraints for an element declaration.
func (rt *Schema) ElementValueConstraints(id ElementID) (ElementValueConstraints, bool, bool) {
	return ElementValueConstraintsByID(rt.ElementValueConstraintReads, id)
}

// ElementTextContent returns text-content metadata for a runtime type and element.
func (rt *Schema) ElementTextContent(t TypeID, elem ElementID) (ElementTextContent, bool) {
	return ElementTextContentByType(
		len(rt.SimpleTypePrimitives),
		rt.ComplexTextContentReads,
		rt.FixedComplexTextContentReads,
		rt.ElementValueConstraintReads,
		rt.SimpleTextContentRead,
		t,
		elem,
	)
}

// ElementHasSimpleContent reports whether a runtime type and element have simple content.
func (rt *Schema) ElementHasSimpleContent(t TypeID, elem ElementID) (bool, bool) {
	return ElementHasSimpleContentByType(
		len(rt.SimpleTypePrimitives),
		rt.ComplexTextContentReads,
		rt.FixedComplexTextContentReads,
		rt.ElementValueConstraintReads,
		rt.SimpleTextContentRead,
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
	if ValidSimpleTypeID(id, len(rt.SimpleValueQNameResolverNeeds)) {
		return rt.SimpleValueQNameResolverNeeds[id]
	}
	if ValidSimpleTypeID(id, len(rt.SimpleValueTypeReads)) {
		return NewSimpleValueQNameResolverNeedsForTypeReads(rt.SimpleValueTypeReads)[id]
	}
	if ValidSimpleTypeID(id, len(rt.SimpleTypes)) {
		return SimpleTypeNeedsQNameResolver(rt.SimpleTypes, id)
	}
	return SimpleValueReadNeedsQNameResolver(rt.SimpleValueReads, id)
}

// ValidateRawSimpleValue validates raw simple content through fast-path reads.
func (rt *Schema) ValidateRawSimpleValue(id SimpleTypeID, raw []byte) (bool, error) {
	if len(rt.SimpleValueTypeReads) != 0 {
		read, ok := simpleValueTypeReadByID(rt.SimpleValueTypeReads, id)
		if !ok {
			if id != NoSimpleType {
				return false, ErrSimpleValueMetadata
			}
			return false, nil
		}
		return validateRawSimpleValueType(rt.SimpleValueTypeReads, rt.SimpleValueFacetReads, id, &read.Type, raw)
	}
	cb := rt.rawSimpleValueCallbacks
	if cb.Type == nil {
		if len(rt.SimpleValueTypeReads) != 0 {
			cb = NewRawSimpleValueCallbacksForTypeReads(rt.SimpleValueTypeReads)
		} else {
			cb = NewRawSimpleValueCallbacksForSimpleTypes(rt.SimpleTypes)
		}
	}
	return ValidateRawSimpleValue(cb, id, raw)
}
