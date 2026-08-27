package runtime

// AttributeDecl returns the validation read projection for an attribute.
func (rt *Schema) AttributeDecl(id AttributeID) (AttributeDeclRead, bool) {
	return AttributeDeclReadByID(rt.runtime.Attributes, id)
}

// SimpleTypePrimitive returns the primitive kind for a simple type.
func (rt *Schema) SimpleTypePrimitive(id SimpleTypeID) (PrimitiveKind, bool) {
	read, ok := simpleValueRouteSlotByID(rt.runtime.SimpleValueRoutes, id)
	if !ok {
		return 0, false
	}
	return read.primitive, true
}

// ElementIdentityConstraints returns an immutable view of identity constraints
// attached to an element.
func (rt *Schema) ElementIdentityConstraints(id ElementID) (IdentityConstraintIDs, bool) {
	return rt.runtime.Elements.identityConstraints(id)
}

// HasIdentityConstraints reports whether the schema has identity constraints.
func (rt *Schema) HasIdentityConstraints() bool {
	return len(rt.runtime.Identities) != 0
}

// IdentityConstraint returns the aggregate validation read for an identity constraint.
func (rt *Schema) IdentityConstraint(id IdentityConstraintID) (IdentityConstraintRead, bool) {
	return IdentityConstraintReadByID(rt.runtime.Identities, id)
}

func (rt *Schema) complexAttributeUses(id ComplexTypeID) (AttributeUseSetRead, bool) {
	if !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return AttributeUseSetRead{}, false
	}
	set := rt.runtime.ComplexTypes[id].attributeUseSet
	if !ValidAttributeUseSetID(set, len(rt.runtime.AttributeUseSets)) {
		return AttributeUseSetRead{}, false
	}
	return rt.runtime.AttributeUseSets[set], true
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

// SimpleContentType returns the simple-content type for a runtime type.
func (rt *Schema) SimpleContentType(t TypeID) (SimpleTypeID, bool, bool) {
	if id, ok := t.Simple(); ok {
		return id, true, ValidSimpleTypeID(id, len(rt.runtime.SimpleValueRoutes))
	}
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return NoSimpleType, false, false
	}
	read := rt.runtime.ComplexTypes[id].simpleContent()
	if !read.HasSimpleContent() {
		return NoSimpleType, false, true
	}
	textType := read.TypeID()
	return textType, true, ValidSimpleTypeID(textType, len(rt.runtime.SimpleValueRoutes))
}

// SimpleIdentity returns identity behavior for a simple type.
func (rt *Schema) SimpleIdentity(id SimpleTypeID) SimpleIdentityKind {
	read, ok := simpleValueRouteSlotByID(rt.runtime.SimpleValueRoutes, id)
	if !ok {
		return SimpleIdentityNone
	}
	return read.identity
}

// ElementValueConstraints returns value constraints for an element declaration.
func (rt *Schema) ElementValueConstraints(id ElementID) (ElementValueConstraints, bool, bool) {
	return rt.runtime.Elements.valueConstraints(id)
}

// ElementTextContent returns text-content metadata for a runtime type and element.
func (rt *Schema) ElementTextContent(t TypeID, elem ElementID) (ElementTextContent, bool) {
	var constraints ElementValueConstraints
	if elem != NoElement {
		var declared, valid bool
		constraints, declared, valid = rt.runtime.Elements.valueConstraints(elem)
		if !valid || !declared {
			return ElementTextContent{}, false
		}
	}
	if id, ok := t.Complex(); ok {
		if !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
			return ElementTextContent{}, false
		}
		_, fixed := constraints.FixedValue()
		return rt.runtime.ComplexTypes[id].textContent(fixed, constraints.HasAny()), true
	}
	if id, ok := t.Simple(); !ok || !ValidSimpleTypeID(id, len(rt.runtime.SimpleValueRoutes)) {
		return ElementTextContent{}, false
	}
	return ElementTextContent{constrained: constraints.HasAny()}, true
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
	return rt.ValidateRawSimpleValueWithScratch(id, raw, nil)
}

// ValidateRawSimpleValueWithScratch validates raw simple content through
// fast-path reads while reusing caller-owned string-pattern buffers.
func (rt *Schema) ValidateRawSimpleValueWithScratch(id SimpleTypeID, raw []byte, scratch *StringPatternScratch) (bool, error) {
	if id == NoSimpleType {
		return false, nil
	}
	return validateResolvedRawSimpleValue(rawSimpleValueResolver{runtime: &rt.runtime, scratch: scratch}, id, raw)
}
