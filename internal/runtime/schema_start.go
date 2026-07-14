package runtime

// AnyType returns the runtime xs:anyType reference for validation start assessment.
func (rt *Schema) AnyType() TypeID {
	return ComplexRef(rt.runtime.TypeDerivations.AnyTypeID())
}

// RootElement returns the global element declaration for name.
func (rt *Schema) RootElement(name RuntimeName) (ElementID, ElementStartInfo, bool) {
	if !name.Known {
		return NoElement, ElementStartInfo{}, false
	}
	id, ok := rt.runtime.GlobalElements[name.Name]
	if !ok {
		return NoElement, ElementStartInfo{}, false
	}
	info, ok := rt.runtime.Elements.start(id)
	return id, info, ok
}

// Element returns validation start data for an element declaration.
func (rt *Schema) Element(id ElementID) (ElementStartInfo, bool) {
	return rt.runtime.Elements.start(id)
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
		availability, _, ok := rt.simpleTypeAvailability(simple)
		if !ok {
			return TypeInfo{}, false
		}
		return NewTypeInfo(TypeInfoShape{Unavailable: availability == simpleTypeAvailabilityUnavailable}), true
	}
	complexID, ok := id.Complex()
	if !ok || !ValidComplexTypeID(complexID, len(rt.runtime.ComplexTypes)) {
		return TypeInfo{}, false
	}
	read := rt.runtime.ComplexTypes[complexID]
	info := read.typeInfo()
	if simple := read.simpleContent(); simple.HasSimpleContent() {
		availability, present, ok := rt.simpleTypeAvailability(simple.TypeID())
		if !ok || !present {
			return TypeInfo{}, false
		}
		info.Unavailable = availability == simpleTypeAvailabilityUnavailable
	}
	return info, true
}

func (rt *Schema) simpleTypeAvailability(id SimpleTypeID) (simpleTypeAvailability, bool, bool) {
	read, ok := simpleValueRouteSlotByID(rt.runtime.SimpleValueRoutes, id)
	if !ok || read.availability == simpleTypeAvailabilityInvalid {
		return simpleTypeAvailabilityInvalid, false, false
	}
	return read.availability, read.present, true
}

// SimpleTypeUnavailable reports whether id contains an intentionally absent
// schema subcomponent. ok is false only for invalid published metadata.
func (rt *Schema) SimpleTypeUnavailable(id SimpleTypeID) (unavailable, ok bool) {
	availability, _, ok := rt.simpleTypeAvailability(id)
	return availability == simpleTypeAvailabilityUnavailable, ok
}

// TypeDerivation reports how derived derives from base.
func (rt *Schema) TypeDerivation(derived, base TypeID) (DerivationMask, bool) {
	return rt.runtime.TypeDerivations.derivation(derived, base, nil)
}

// TypeDerivationWithScratch reports how derived derives from base while reusing
// document-local union traversal storage.
func (rt *Schema) TypeDerivationWithScratch(derived, base TypeID, scratch *TypeDerivationScratch) (DerivationMask, bool) {
	return rt.runtime.TypeDerivations.derivation(derived, base, scratch)
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
