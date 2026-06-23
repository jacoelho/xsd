package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) validateSimpleValue(id runtime.SimpleTypeID, lexical string, resolve runtime.ResolveQNameParts, needs runtime.SimpleValueNeed) (runtime.SimpleValue, error) {
	cb := c.simpleValues
	if cb.Type == nil {
		cb = runtime.SimpleValueCallbacks{
			Type:                     c.simpleValueType,
			Facets:                   c.simpleValueFacets,
			ForEachStringEnumeration: c.forEachStringEnumeration,
			StringEnumeration:        c.stringEnumerationContains,
			Notation:                 c.notationDeclared,
			Unsupported:              xsderrors.IsUnsupported,
		}
		c.simpleValues = cb
	}
	cb.ResolveQName = resolve
	return runtime.ValidateSimpleValue(cb, id, lexical, needs)
}

func (c *compiler) simpleValueType(id runtime.SimpleTypeID) (runtime.SimpleValueType, bool) {
	st, ok := c.rt.UsableSimpleType(id)
	if !ok {
		return runtime.SimpleValueType{}, false
	}
	return runtime.SimpleValueTypeForSimpleType(*st), true
}

func (c *compiler) simpleValueFacets(id runtime.SimpleTypeID) (runtime.SimpleValueFacets, bool) {
	st, ok := c.rt.UsableSimpleType(id)
	if !ok {
		return runtime.SimpleValueFacets{}, false
	}
	return runtime.SimpleValueFacetsForFacetSet(st.Facets), true
}

func (c *compiler) forEachStringEnumeration(id runtime.SimpleTypeID, yield func(string) bool) {
	st, ok := c.rt.UsableSimpleType(id)
	if !ok {
		return
	}
	for _, lit := range st.Facets.Enumeration {
		if !yield(lit.Canonical) {
			return
		}
	}
}

func (c *compiler) stringEnumerationContains(id runtime.SimpleTypeID, canonical string) (bool, bool) {
	st, ok := c.rt.UsableSimpleType(id)
	if !ok {
		return false, false
	}
	for _, lit := range st.Facets.Enumeration {
		if lit.Canonical == canonical {
			return true, true
		}
	}
	return false, true
}

func (c *compiler) notationDeclared(ns, local string) bool {
	q, ok := c.rt.Names.LookupQName(ns, local)
	return ok && c.rt.Notations[q]
}
