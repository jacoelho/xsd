package runtime

// AnyType returns the runtime xs:anyType reference for validation start assessment.
func (rt *Schema) AnyType() TypeID {
	return ComplexRef(rt.runtime.TypeDerivations.AnyTypeID())
}

// RootElement returns the global element declaration for name.
func (rt *Schema) RootElement(name RuntimeName) (ElementID, ElementStartInfo, bool) {
	return RootElementByName(rt.runtime.GlobalElements, rt.runtime.ElementStarts, name)
}

// Element returns validation start data for an element declaration.
func (rt *Schema) Element(id ElementID) (ElementStartInfo, bool) {
	return ElementStartInfoByID(rt.runtime.ElementStarts, id)
}

// Type returns the global type for name.
func (rt *Schema) Type(name QName) (TypeID, bool) {
	return rt.GlobalType(name)
}

// GlobalType returns the global type declaration for name.
func (rt *Schema) GlobalType(name QName) (TypeID, bool) {
	return GlobalTypeByName(rt.runtime.GlobalTypes, rt.runtime.TypeDerivations, name)
}

// LookupQName returns the runtime QName for a namespace URI and local name.
func (rt *Schema) LookupQName(ns, local string) (QName, bool) {
	return rt.runtime.Names.LookupQName(ns, local)
}

// Namespace returns the namespace URI for id.
func (rt *Schema) Namespace(id NamespaceID) string {
	return rt.runtime.Names.Namespace(id)
}

// TypeInfo returns validation start data for a runtime type.
func (rt *Schema) TypeInfo(id TypeID) (TypeInfo, bool) {
	if simple, ok := id.Simple(); ok {
		return TypeInfo{}, ValidSimpleTypeID(simple, len(rt.runtime.SimpleValueRoutes))
	}
	complexID, ok := id.Complex()
	if !ok || !ValidComplexTypeID(complexID, len(rt.runtime.ComplexTypes)) {
		return TypeInfo{}, false
	}
	return rt.runtime.ComplexTypes[complexID].typeInfo(), true
}

// TypeDerivation reports how derived derives from base.
func (rt *Schema) TypeDerivation(derived, base TypeID) (DerivationMask, bool) {
	return TypeDerivationMask(typeDerivationReadRuntime{read: rt.runtime.TypeDerivations}, derived, base)
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
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(rt.runtime.ComplexTypes)) {
		return NoContentModel
	}
	return rt.runtime.ComplexTypes[id].contentModel
}

// GlobalAttribute returns the global attribute declaration for name.
func (rt *Schema) GlobalAttribute(name QName) (AttributeID, bool, bool) {
	return GlobalAttributeByName(rt.runtime.GlobalAttributes, rt.runtime.Attributes, name)
}

// WildcardView returns a validation-facing wildcard view.
func (rt *Schema) WildcardView(id WildcardID) (WildcardView, bool) {
	return WildcardViewByID(rt.runtime.Wildcards, id)
}
