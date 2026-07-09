package runtime

// RuntimeGlobalsForTest exposes runtime global declaration projections.
func (rt *Schema) RuntimeGlobalsForTest() RuntimeGlobals {
	return rt.RuntimeGlobals()
}

// ValidateSimpleValueRuntimeBoundaryForTest validates a simple value through runtime reads.
func (rt *Schema) ValidateSimpleValueRuntimeBoundaryForTest(id SimpleTypeID, lexical string, resolve func(string) (string, string, bool), needs SimpleValueNeed) (SimpleValue, error) {
	return rt.ValidateSimpleValue(id, lexical, ResolveQNameParts(resolve), needs)
}

// WildcardAllowsURIForTest reports whether a wildcard accepts a namespace URI.
func (rt *Schema) WildcardAllowsURIForTest(w Wildcard, ns string) bool {
	return WildcardAllowsURI(&rt.build.Names, w, ns)
}

// SimpleContentTypeForTest exposes simple-content type projection reads.
func (rt *Schema) SimpleContentTypeForTest(t TypeID) (SimpleTypeID, bool, bool) {
	return rt.SimpleContentType(t)
}

// ElementValueConstraintsForTest exposes element value-constraint projection reads.
func (rt *Schema) ElementValueConstraintsForTest(id ElementID) (ElementValueConstraints, bool, bool) {
	return rt.ElementValueConstraints(id)
}

// DeclaredElementTypeForTest exposes declared element type projection reads.
func (rt *Schema) DeclaredElementTypeForTest(id ElementID) (TypeID, bool) {
	return rt.DeclaredElementType(id)
}

// ElementChildContentForTest exposes child-content projection reads.
func (rt *Schema) ElementChildContentForTest(t TypeID) (ElementChildContent, bool) {
	return rt.elementChildContent(t)
}

// ComplexAttributeUsesForTest exposes complex attribute-use projection reads.
func (rt *Schema) ComplexAttributeUsesForTest(id ComplexTypeID) (AttributeUseSetRead, bool) {
	return rt.complexAttributeUses(id)
}

// ElementTextContentForTest exposes text-content projection reads.
func (rt *Schema) ElementTextContentForTest(t TypeID, elem ElementID) (ElementTextContent, bool) {
	return rt.ElementTextContent(t, elem)
}
