package runtime

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// SimpleTypeIdentity returns the stored ID/IDREF behavior for simple type id.
func (rt *Schema) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	if !rt.ReadProjectionsPublished() {
		st, ok := rt.UsableSimpleType(id)
		if !ok {
			return SimpleIdentityNone, false
		}
		return st.Identity, true
	}
	st, ok := rt.SimpleType(id)
	if !ok {
		return SimpleIdentityNone, false
	}
	return st.Identity, true
}

// DerivedSimpleIdentity derives the identity behavior for a simple type.
func (rt *Schema) DerivedSimpleIdentity(st SimpleType) SimpleIdentityKind {
	return DerivedSimpleIdentityForSimpleType(rt, st)
}

func (rt *Schema) runtimeSimpleValueTypeRead(id SimpleTypeID) (*SimpleValueTypeRead, bool) {
	return simpleValueTypeReadByID(rt.SimpleValueTypeReads, id)
}

// ValidateSimpleValue validates a lexical simple value using frozen runtime reads.
func (rt *Schema) ValidateSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if len(rt.SimpleValueTypeReads) != 0 {
		if value, handled, err := validateSimpleValueTypeReadFast(rt.SimpleValueTypeReads, id, lexical, needs); handled {
			return value, err
		}
	}
	cb := rt.simpleValueCallbacks
	if cb.Type == nil {
		if len(rt.SimpleValueTypeReads) != 0 {
			if len(rt.SimpleValueFacetReads.Index) != 0 {
				cb = NewSimpleValueCallbacksForTypeReads(rt.SimpleValueTypeReads, rt.SimpleValueFacetReads, runtimeNotationLookup(rt), nil, nil)
			} else {
				cb = NewSimpleValueCallbacksForTypeReadsAndSimpleTypes(rt.SimpleValueTypeReads, rt.SimpleTypes, runtimeNotationLookup(rt), nil, nil)
			}
		} else {
			cb = NewSimpleValueCallbacksForSimpleTypes(rt.SimpleTypes, runtimeNotationLookup(rt), nil, nil)
		}
	}
	cb.ResolveQName = resolve
	cb.Unsupported = xsderrors.IsUnsupported
	return ValidateSimpleValue(cb, id, lexical, needs)
}

func runtimeNotationLookup(rt *Schema) func(ns, local string) bool {
	return notationReadLookup(rt.NotationReads)
}

func notationReadLookup(reads map[ExpandedName]bool) func(ns, local string) bool {
	return func(ns, local string) bool {
		return reads[ExpandedName{Namespace: ns, Local: local}]
	}
}

func compilerNotationLookup(rt *Schema) func(ns, local string) bool {
	return func(ns, local string) bool {
		q, ok := rt.Names.LookupQName(ns, local)
		return ok && rt.Notations[q]
	}
}
