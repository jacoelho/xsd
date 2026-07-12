package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type simpleValueFacetCache struct {
	projector runtime.SimpleValueFacetProjector
	index     []uint32
	values    []runtime.SimpleValueFacets
}

func (c *simpleValueFacetCache) read(types []runtime.SimpleType, id runtime.SimpleTypeID) (runtime.SimpleValueFacets, bool) {
	if !runtime.ValidSimpleTypeID(id, len(types)) || types[id].Missing {
		return runtime.SimpleValueFacets{}, false
	}
	if missing := len(types) - len(c.index); missing > 0 {
		c.index = append(c.index, make([]uint32, missing)...)
	}
	if slot := c.index[id]; slot != 0 {
		return c.values[slot-1], true
	}
	facets := types[id].Facets
	c.values = append(c.values, c.projector.Project(facets))
	c.index[id] = uint32(len(c.values)) //nolint:gosec // the compiler bounds simple-type IDs to uint32.
	return c.values[len(c.values)-1], true
}

func (c *compiler) validateSimpleValue(id runtime.SimpleTypeID, lexical string, resolve runtime.ResolveQNameParts, needs runtime.SimpleValueNeed) (runtime.SimpleValue, error) {
	cb := c.simpleValues
	if cb.Type == nil {
		cb = runtime.SimpleValueCallbacks{
			Type:              c.simpleValueType,
			Facets:            c.simpleValueFacets,
			StringEnumeration: c.stringEnumerationContains,
			Notation:          c.notationDeclared,
			Unsupported:       xsderrors.IsUnsupported,
		}
		c.simpleValues = cb
	}
	cb.ResolveQName = resolve
	return runtime.ValidateSimpleValue(cb, id, lexical, needs)
}

func (c *compiler) simpleValueType(id runtime.SimpleTypeID) (runtime.SimpleValueType, bool) {
	return c.rt.simpleValueType(id)
}

func (c *compiler) simpleValueFacets(id runtime.SimpleTypeID) (runtime.SimpleValueFacets, bool) {
	return c.readSimpleFacets(id)
}

func (c *compiler) stringEnumerationContains(id runtime.SimpleTypeID, canonical string) (bool, bool) {
	return c.rt.StringEnumerationContains(id, canonical)
}

func (c *compiler) notationDeclared(ns, local string) bool {
	q, ok := c.rt.lookupQName(ns, local)
	return ok && c.rt.notationDeclared(q)
}
