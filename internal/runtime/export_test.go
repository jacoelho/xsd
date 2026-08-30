package runtime

// ValidateSimpleValueRuntimeBoundaryForTest validates a simple value through runtime reads.
func (rt *Schema) ValidateSimpleValueRuntimeBoundaryForTest(id SimpleTypeID, lexical string, resolve func(string) (string, string, bool), needs SimpleValueNeed) (SimpleValue, error) {
	return rt.ValidateSimpleValue(id, lexical, ResolveQNameParts(resolve), needs)
}

// SimpleContentTypeForTest exposes simple-content type projection reads.
func (rt *Schema) SimpleContentTypeForTest(t TypeID) (id SimpleTypeID, present, valid bool) {
	return rt.SimpleContentType(t)
}

// ElementValueConstraintsForTest exposes element value-constraint projection reads.
func (rt *Schema) ElementValueConstraintsForTest(id ElementID) (constraints ElementValueConstraints, present, valid bool) {
	return rt.ElementValueConstraints(id)
}

// ComplexAttributeUsesForTest exposes complex attribute-use projection reads.
func (rt *Schema) ComplexAttributeUsesForTest(id ComplexTypeID) (AttributeUseSetRead, bool) {
	return rt.complexAttributeUses(id)
}
