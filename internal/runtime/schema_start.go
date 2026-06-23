package runtime

// AnyType returns the runtime xs:anyType reference for validation start assessment.
func (rt *Schema) AnyType() TypeID {
	return ComplexRef(rt.TypeDerivations.AnyTypeID())
}

// RootElement returns the global element declaration for name.
func (rt *Schema) RootElement(name RuntimeName) (ElementID, ElementStartInfo, bool) {
	return RootElementByName(rt.GlobalElementReads, rt.ElementStartInfos, name)
}

// Element returns validation start data for an element declaration.
func (rt *Schema) Element(id ElementID) (ElementStartInfo, bool) {
	return ElementStartInfoByID(rt.ElementStartInfos, id)
}

// Type returns the global type for name.
func (rt *Schema) Type(name QName) (TypeID, bool) {
	return rt.GlobalType(name)
}

// GlobalType returns the global type declaration for name.
func (rt *Schema) GlobalType(name QName) (TypeID, bool) {
	return GlobalTypeByName(rt.GlobalTypeReads, rt.TypeDerivations, name)
}

// LookupQName returns the runtime QName for a namespace URI and local name.
func (rt *Schema) LookupQName(ns, local string) (QName, bool) {
	return rt.NameReads.LookupQName(ns, local)
}

// Namespace returns the namespace URI for id.
func (rt *Schema) Namespace(id NamespaceID) string {
	return rt.NameReads.Namespace(id)
}

// TypeInfo returns validation start data for a runtime type.
func (rt *Schema) TypeInfo(id TypeID) (TypeInfo, bool) {
	return TypeInfoByID(len(rt.SimpleTypePrimitives), rt.ComplexTypeInfos, id)
}

// TypeDerivation reports how derived derives from base.
func (rt *Schema) TypeDerivation(derived, base TypeID) (DerivationMask, bool) {
	if rt.ReadProjectionsPublished() {
		return TypeDerivationMask(typeDerivationReadRuntime{read: rt.TypeDerivations}, derived, base)
	}
	return TypeDerivationMask(rt.TypeDerivations, derived, base)
}

// ChildContent returns validation content data for a runtime type.
func (rt *Schema) ChildContent(id TypeID) (ChildContentInfo, bool) {
	content, ok := rt.elementChildContent(id)
	if !ok {
		return ChildContentInfo{}, false
	}
	return NewChildContentInfoForElementChildContent(content), true
}

// ContentModelForType returns the content model used to validate children of a runtime type.
func (rt *Schema) ContentModelForType(t TypeID) ContentModelID {
	return ContentModelForTypeByID(rt.ComplexContentModelIDs, t)
}

// DeclaredElementType returns the declared runtime type for an element.
func (rt *Schema) DeclaredElementType(id ElementID) (TypeID, bool) {
	return DeclaredElementTypeByID(rt.ElementStartInfos, id)
}

// CompiledContentModelView returns the compiled runtime content model view.
func (rt *Schema) CompiledContentModelView(id ContentModelID) (CompiledModelView, bool) {
	return CompiledModelViewByID(rt.CompiledModelViews, id)
}

// GlobalElement returns the global element declaration for name.
func (rt *Schema) GlobalElement(name QName) (ElementID, bool) {
	return GlobalElementByName(rt.GlobalElementReads, rt.ElementStartInfos, name)
}

// GlobalAttribute returns the global attribute declaration for name.
func (rt *Schema) GlobalAttribute(name QName) (AttributeID, bool, bool) {
	return GlobalAttributeByName(rt.GlobalAttributeReads, rt.AttributeDeclReads, name)
}

// WildcardView returns a validation-facing wildcard view.
func (rt *Schema) WildcardView(id WildcardID) (WildcardView, bool) {
	return WildcardViewByID(rt.WildcardReads, id)
}
