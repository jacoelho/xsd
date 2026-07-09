package runtime

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// SimpleTypeIdentity returns the stored ID/IDREF behavior for simple type id.
func (rt *Schema) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	return SimpleTypeIdentityByID(rt.reads.SimpleTypeIdentities, id), ValidSimpleTypeID(id, len(rt.reads.SimpleTypeIdentities))
}

// SimpleTypeIdentity returns compiler-owned identity metadata.
func (rt *SchemaBuild) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return SimpleIdentityNone, false
	}
	return st.Identity, true
}

// DerivedSimpleIdentity derives compiler-owned identity metadata.
func (rt *SchemaBuild) DerivedSimpleIdentity(st SimpleType) SimpleIdentityKind {
	return DerivedSimpleIdentityForSimpleType(rt, st)
}

func (rt *Schema) runtimeSimpleValueTypeRead(id SimpleTypeID) (*SimpleValueTypeRead, bool) {
	return simpleValueTypeReadByID(rt.reads.SimpleValueTypes, id)
}

// ValidateSimpleValue validates a lexical simple value using frozen runtime reads.
func (rt *Schema) ValidateSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if value, handled, err := validateSimpleValueTypeReadFast(rt.reads.SimpleValueTypes, id, lexical, needs); handled {
		return value, err
	}
	cb := rt.reads.simpleValueCallbacks
	cb.ResolveQName = resolve
	cb.Unsupported = xsderrors.IsUnsupported
	return ValidateSimpleValue(cb, id, lexical, needs)
}

func notationReadLookup(reads map[ExpandedName]bool) func(ns, local string) bool {
	return func(ns, local string) bool {
		return reads[ExpandedName{Namespace: ns, Local: local}]
	}
}
