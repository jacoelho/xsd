package schema

// AttributeDecl returns the validation read projection for an attribute.
func (rt *Schema) AttributeDecl(id AttributeID) (AttributeDeclRead, bool) {
	return attributeDeclReadByID(rt.program.Attributes, id)
}

// ElementIdentityConstraints returns an immutable view of identity constraints
// attached to an element.
func (rt *Schema) ElementIdentityConstraints(id ElementID) (IdentityConstraintIDs, bool) {
	return rt.program.Elements.identityConstraints(id)
}

// HasIdentityConstraints reports whether the schema has identity constraints.
func (rt *Schema) HasIdentityConstraints() bool {
	return rt.program.IdentityDispatch.ConstraintCount() != 0
}

// IdentityConstraint returns the aggregate validation read for an identity constraint.
func (rt *Schema) IdentityConstraint(id IdentityConstraintID) (IdentityConstraintRead, bool) {
	return rt.program.IdentityDispatch.identityConstraint(id)
}

// IdentityDispatch returns the immutable selector and field dispatch index
// derived during schema publication.
func (rt *Schema) IdentityDispatch() IdentityDispatchRead {
	return rt.program.IdentityDispatch
}

func (rt *Schema) complexAttributeUses(id ComplexTypeID) (AttributeUseSetRead, bool) {
	if !ValidComplexTypeID(id, len(rt.program.ComplexTypes)) {
		return AttributeUseSetRead{}, false
	}
	set := rt.program.ComplexTypes[id].attributeUseSet
	if !ValidAttributeUseSetID(set, len(rt.program.AttributeUseSets)) {
		return AttributeUseSetRead{}, false
	}
	return rt.program.AttributeUseSets[set], true
}

// AttributeUseSetForType returns attribute-use reads for a runtime type.
func (rt *Schema) AttributeUseSetForType(typ TypeID) (set AttributeUseSetRead, present, valid bool) {
	id, ok := typ.Complex()
	if !ok {
		return AttributeUseSetRead{}, false, true
	}
	set, valid = rt.complexAttributeUses(id)
	return set, true, valid
}

// ElementFrame returns the immutable schema projection needed to initialize
// one validation element frame. actualType is the effective type after any
// xsi:type selection; elem remains the declaration that owns value
// constraints and text-content policy.
func (rt *Schema) ElementFrame(actualType TypeID, elem ElementID) (ElementFrameRead, bool) {
	fixed, constrained, valid := rt.program.Elements.constraintFlags(elem)
	if !valid {
		return ElementFrameRead{}, false
	}

	frame := ElementFrameRead{
		SimpleContent: NoSimpleType,
		TextContent:   ElementTextContent{fixed: fixed, constrained: constrained},
	}
	if id, ok := actualType.Simple(); ok {
		return rt.simpleElementFrame(frame, id)
	}
	id, ok := actualType.Complex()
	if !ok {
		return ElementFrameRead{}, false
	}
	return rt.complexElementFrame(frame, id)
}

func (rt *Schema) simpleElementFrame(frame ElementFrameRead, id SimpleTypeID) (ElementFrameRead, bool) {
	if _, _, valid := rt.simpleTypeAvailability(id); !valid {
		return ElementFrameRead{}, false
	}
	content, valid := rt.contentFrameForModel(NoContentModel)
	if !valid {
		return ElementFrameRead{}, false
	}
	frame.Content = content
	frame.SimpleContent = id
	return frame, true
}

func (rt *Schema) complexElementFrame(frame ElementFrameRead, id ComplexTypeID) (ElementFrameRead, bool) {
	if !ValidComplexTypeID(id, len(rt.program.ComplexTypes)) {
		return ElementFrameRead{}, false
	}
	read := rt.program.ComplexTypes[id]
	content, valid := rt.contentFrameForModel(read.contentModel)
	if !valid {
		return ElementFrameRead{}, false
	}
	frame.Content = content
	simple := read.simpleContent()
	if simple.HasSimpleContent() {
		textType := simple.TypeID()
		_, _, valid := rt.simpleTypeAvailability(textType)
		if !valid {
			return ElementFrameRead{}, false
		}
		frame.SimpleContent = textType
	}
	frame.TextContent = read.textContent(frame.TextContent.fixed, frame.TextContent.constrained)
	return frame, true
}

// ElementValueConstraints returns value constraints for an element declaration.
func (rt *Schema) ElementValueConstraints(id ElementID) (constraints ElementValueConstraints, present, valid bool) {
	return rt.program.Elements.valueConstraints(id)
}
