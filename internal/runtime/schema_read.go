package runtime

// AttributeDecl returns the validation read projection for an attribute.
func (rt *Schema) AttributeDecl(id AttributeID) (AttributeDeclRead, bool) {
	return AttributeDeclReadByID(rt.runtime.Attributes, id)
}

// SimpleTypePrimitive returns the primitive kind for a simple type.
func (rt *Schema) SimpleTypePrimitive(id SimpleTypeID) (PrimitiveKind, bool) {
	return SimpleTypePrimitiveByID(rt.runtime.SimpleTypePrimitives, id)
}

// ForEachElementIdentityConstraint iterates identity constraints on an element.
func (rt *Schema) ForEachElementIdentityConstraint(id ElementID, fn func(IdentityConstraintID) bool) {
	ForEachElementIdentityConstraint(rt.runtime.ElementIdentities, id, fn)
}

// ElementIdentityConstraints returns identity constraints attached to an
// element.
func (rt *Schema) ElementIdentityConstraints(id ElementID) []IdentityConstraintID {
	if !ValidElementID(id, len(rt.runtime.ElementIdentities)) {
		return nil
	}
	return rt.runtime.ElementIdentities[id]
}

// HasIdentityConstraints reports whether the schema has identity constraints.
func (rt *Schema) HasIdentityConstraints() bool {
	return len(rt.runtime.Identities) != 0
}

// IdentitySelectorPaths returns selector paths for an identity constraint.
func (rt *Schema) IdentitySelectorPaths(id IdentityConstraintID) ([]IdentityPath, bool) {
	return IdentitySelectorPaths(rt.runtime.Identities, id)
}

// ForEachIdentitySelector iterates selector paths for an identity constraint.
func (rt *Schema) ForEachIdentitySelector(id IdentityConstraintID, fn func(IdentityPath) bool) bool {
	return ForEachIdentitySelector(rt.runtime.Identities, id, fn)
}

// IdentityFieldCount returns the number of fields for an identity constraint.
func (rt *Schema) IdentityFieldCount(id IdentityConstraintID) (int, bool) {
	return IdentityFieldCount(rt.runtime.Identities, id)
}

// IdentityElementFields returns element-field lookups for an identity
// constraint.
func (rt *Schema) IdentityElementFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityElementFields(rt.runtime.Identities, id)
}

// ForEachIdentityElementField iterates element fields for an identity constraint.
func (rt *Schema) ForEachIdentityElementField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityElementField(rt.runtime.Identities, id, fn)
}

// IdentityAttributeFields returns exact attribute-field lookups for an identity
// constraint and attribute name.
func (rt *Schema) IdentityAttributeFields(id IdentityConstraintID, name QName) ([]CompiledIdentityField, bool) {
	return IdentityAttributeFields(rt.runtime.Identities, id, name)
}

// ForEachIdentityAttributeField iterates attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeField(id IdentityConstraintID, name QName, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeField(rt.runtime.Identities, id, name, fn)
}

// IdentityAttributeWildcardFields returns wildcard attribute-field lookups for
// an identity constraint.
func (rt *Schema) IdentityAttributeWildcardFields(id IdentityConstraintID) ([]CompiledIdentityField, bool) {
	return IdentityAttributeWildcardFields(rt.runtime.Identities, id)
}

// ForEachIdentityAttributeWildcardField iterates wildcard attribute fields for an identity constraint.
func (rt *Schema) ForEachIdentityAttributeWildcardField(id IdentityConstraintID, fn func(CompiledIdentityField) bool) bool {
	return ForEachIdentityAttributeWildcardField(rt.runtime.Identities, id, fn)
}

// IdentityConstraintInfo returns metadata for an identity constraint.
func (rt *Schema) IdentityConstraintInfo(id IdentityConstraintID) (IdentityConstraintInfo, bool) {
	return IdentityConstraintInfoByID(rt.runtime.Identities, id)
}

func (rt *Schema) elementChildContent(t TypeID) (ElementChildContent, bool) {
	if simple, ok := t.Simple(); ok {
		return ElementChildContent{}, ValidSimpleTypeID(simple, len(rt.runtime.SimpleTypePrimitives))
	}
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return ElementChildContent{}, false
	}
	return rt.runtime.ComplexTypes[id].childContent, true
}

func (rt *Schema) complexAttributeUses(id ComplexTypeID) (AttributeUseSetRead, bool) {
	set, ok := rt.complexAttributeUsesPtr(id)
	if !ok {
		return AttributeUseSetRead{}, false
	}
	return *set, true
}

func (rt *Schema) complexAttributeUsesPtr(id ComplexTypeID) (*AttributeUseSetRead, bool) {
	if !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return nil, false
	}
	set := rt.runtime.ComplexTypes[id].attributeUseSet
	if !ValidAttributeUseSetID(set, len(rt.runtime.AttributeUseSets)) {
		return nil, false
	}
	return &rt.runtime.AttributeUseSets[set], true
}

// AttributeUseSetForType returns attribute-use reads for a runtime type.
func (rt *Schema) AttributeUseSetForType(typ TypeID) (AttributeUseSetRead, bool, bool) {
	id, ok := typ.Complex()
	if !ok {
		return AttributeUseSetRead{}, false, true
	}
	set, valid := rt.complexAttributeUses(id)
	return set, true, valid
}

// AttributeUseSetForTypePtr returns attribute-use reads for a runtime type
// without copying the immutable read projection.
func (rt *Schema) AttributeUseSetForTypePtr(typ TypeID) (*AttributeUseSetRead, bool, bool) {
	id, ok := typ.Complex()
	if !ok {
		return nil, false, true
	}
	set, valid := rt.complexAttributeUsesPtr(id)
	return set, true, valid
}

// SimpleContentType returns the simple-content type for a runtime type.
func (rt *Schema) SimpleContentType(t TypeID) (SimpleTypeID, bool, bool) {
	if id, ok := t.Simple(); ok {
		return id, true, ValidSimpleTypeID(id, len(rt.runtime.SimpleTypePrimitives))
	}
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return NoSimpleType, false, false
	}
	read := rt.runtime.ComplexTypes[id].simpleContent
	if !read.HasSimpleContent() {
		return NoSimpleType, false, true
	}
	textType := read.TypeID()
	return textType, true, ValidSimpleTypeID(textType, len(rt.runtime.SimpleTypePrimitives))
}

// SimpleIdentity returns identity behavior for a simple type.
func (rt *Schema) SimpleIdentity(id SimpleTypeID) SimpleIdentityKind {
	return SimpleTypeIdentityByID(rt.runtime.SimpleTypeIdentities, id)
}

// ElementValueConstraints returns value constraints for an element declaration.
func (rt *Schema) ElementValueConstraints(id ElementID) (ElementValueConstraints, bool, bool) {
	return ElementValueConstraintsByID(rt.runtime.ElementValueConstraints, id)
}

// ElementTextContent returns text-content metadata for a runtime type and element.
func (rt *Schema) ElementTextContent(t TypeID, elem ElementID) (ElementTextContent, bool) {
	if elem != NoElement && !ValidElementID(elem, len(rt.runtime.ElementValueConstraints)) {
		return ElementTextContent{}, false
	}
	if id, ok := t.Complex(); ok {
		if !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
			return ElementTextContent{}, false
		}
		if elem != NoElement {
			if _, fixed := rt.runtime.ElementValueConstraints[elem].FixedValue(); fixed {
				return rt.runtime.ComplexTypes[id].fixedText, true
			}
		}
		return rt.runtime.ComplexTypes[id].textContent, true
	}
	if id, ok := t.Simple(); !ok || !ValidSimpleTypeID(id, len(rt.runtime.SimpleTypePrimitives)) {
		return ElementTextContent{}, false
	}
	return rt.runtime.SimpleTextContent, true
}

// ElementHasSimpleContent reports whether a runtime type and element have simple content.
func (rt *Schema) ElementHasSimpleContent(t TypeID, elem ElementID) (bool, bool) {
	content, ok := rt.ElementTextContent(t, elem)
	return content.HasSimpleContent(), ok
}

// SimpleValueNeedsQNameResolver reports whether validating id can require
// lexical QName namespace resolution.
func (rt *Schema) SimpleValueNeedsQNameResolver(id SimpleTypeID) bool {
	if rt == nil {
		return false
	}
	if ValidSimpleTypeID(id, len(rt.runtime.SimpleValueQNameNeeds)) {
		return rt.runtime.SimpleValueQNameNeeds[id]
	}
	return false
}

// ValidateRawSimpleValue validates raw simple content through fast-path reads.
func (rt *Schema) ValidateRawSimpleValue(id SimpleTypeID, raw []byte) (bool, error) {
	if id == NoSimpleType {
		return false, nil
	}
	return rt.validatePublishedRawSimpleValue(id, raw)
}
